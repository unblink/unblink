package relay

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/google/uuid"
)

// VideoRecorder records continuous video using HLS format
// HLS creates .m3u8 playlist + .ts segments, handling rotation automatically
type VideoRecorder struct {
	serviceID      string
	segmentTime    int // in seconds
	storageManager *StorageManager
	storageTable   *table_storage
	closeChan      chan struct{}
	closeOnce      sync.Once
	mu             sync.Mutex

	// FFmpeg pipeline
	ffmpegCmd   *exec.Cmd
	ffmpegStdin io.WriteCloser
	producer    core.Producer

	// HLS stream info
	playlistID   string
	playlistPath string
	storagePath  string
	startTime    time.Time
}

// NewVideoRecorder creates a new video recorder
func NewVideoRecorder(serviceID string, segmentDuration time.Duration,
	storageManager *StorageManager, storageTable *table_storage) *VideoRecorder {
	return &VideoRecorder{
		serviceID:      serviceID,
		segmentTime:    int(segmentDuration.Seconds()),
		storageManager: storageManager,
		storageTable:   storageTable,
		closeChan:      make(chan struct{}),
	}
}

// Start begins recording from the media source
func (r *VideoRecorder) Start(mediaSource MediaSource) error {
	log.Printf("[VideoRecorder] Starting HLS recording for service %s (segment_time=%ds)",
		r.serviceID, r.segmentTime)

	// Get producer from media source
	producer := mediaSource.GetProducer()
	if producer == nil {
		return fmt.Errorf("media source has no producer")
	}
	r.producer = producer

	// Create HLS playlist
	if err := r.startHLSStream(); err != nil {
		return fmt.Errorf("failed to start HLS stream: %w", err)
	}

	// Start consumer goroutine to pump H.264 to FFmpeg
	go r.consumeH264ToFFmpeg()

	return nil
}

// startHLSStream creates the HLS playlist and starts FFmpeg
func (r *VideoRecorder) startHLSStream() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	playlistID := uuid.New().String()
	startTime := time.Now()

	// Get base directory from storage manager
	baseDir := r.storageManager.backend.(*LocalStorage).baseDir
	// Use playlistID (unique UUID) instead of serviceID to avoid overwriting old recordings
	playlistDir := filepath.Join(baseDir, "hls", playlistID)

	// Ensure directory exists
	if err := os.MkdirAll(playlistDir, 0755); err != nil {
		return fmt.Errorf("failed to create HLS directory: %w", err)
	}

	// HLS playlist path (stream.m3u8)
	playlistPath := filepath.Join(playlistDir, "stream.m3u8")
	storagePath := "local://hls/" + playlistID + "/stream.m3u8"

	// Create database record upfront with "recording" status
	metadata := map[string]interface{}{
		"status":        "recording",
		"start_time":    startTime.Format(time.RFC3339),
		"format":        "hls",
		"segment_time":  r.segmentTime,
		"playlist_path": storagePath,
		"segments_dir":  "local://hls/" + playlistID,
	}
	if err := r.storageTable.CreateStorage(playlistID, r.serviceID, "video", storagePath,
		startTime, 0, "application/vnd.apple.mpegurl", metadata); err != nil {
		return fmt.Errorf("failed to create video record: %w", err)
	}

	// FFmpeg command for HLS recording
	// -hls_time: segment duration in seconds
	// -hls_list_size: number of segments to keep in playlist (0 = all)
	cmd := exec.Command("ffmpeg",
		"-loglevel", "error",
		"-f", "mpegts",
		"-i", "pipe:0",
		"-map", "0", // Map all input streams (video + optional audio)
		"-c", "copy", // Copy both video and audio streams
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", r.segmentTime),
		"-hls_list_size", "0", // Keep all segments in playlist
		"-hls_flags", "independent_segments+omit_endlist", // omit_endlist is better for live
		"-hls_segment_filename", filepath.Join(playlistDir, "segment_%03d.ts"),
		playlistPath,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	// Capture stderr to see ffmpeg errors in relay logs
	cmd.Stderr = &ffmpegErrorWriter{prefix: r.serviceID}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	r.ffmpegCmd = cmd
	r.ffmpegStdin = stdin
	r.playlistID = playlistID
	r.playlistPath = playlistPath
	r.storagePath = storagePath
	r.startTime = startTime

	log.Printf("[VideoRecorder] Started HLS stream %s for service %s (playlist: %s)",
		playlistID, r.serviceID, playlistPath)

	return nil
}

type ffmpegErrorWriter struct {
	prefix string
}

func (w *ffmpegErrorWriter) Write(p []byte) (n int, err error) {
	log.Printf("[FFmpeg-%s] %s", w.prefix, string(p))
	return len(p), nil
}

// consumeH264ToFFmpeg reads H.264 packets and pipes to FFmpeg
func (r *VideoRecorder) consumeH264ToFFmpeg() {
	producer := r.producer
	defer log.Printf("[VideoRecorder] Stopped H.264 consumer for service %s", r.serviceID)

	// Create MPEG-TS consumer using shared helper
	tsConsumer, err := NewMPEGTSConsumer(producer, r.serviceID)
	if err != nil {
		log.Printf("[VideoRecorder] Failed to create MPEG-TS consumer: %v", err)
		return
	}
	defer tsConsumer.Stop()

	for {
		select {
		case <-r.closeChan:
			return
		default:
		}

		r.mu.Lock()
		stdin := r.ffmpegStdin
		r.mu.Unlock()

		if stdin == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Write to FFmpeg stdin
		// HLS handles segment rotation automatically
		n, err := tsConsumer.WriteTo(stdin)
		if err != nil {
			select {
			case <-r.closeChan:
				// Recorder is closing
				return
			default:
				// Unexpected error
				log.Printf("[VideoRecorder] WriteTo error: %v", err)
				return
			}
		}
		if n == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Close stops the recorder and finalizes the recording
func (r *VideoRecorder) Close() {
	r.closeOnce.Do(func() {
		log.Printf("[VideoRecorder] Closing HLS recorder for service %s", r.serviceID)
		close(r.closeChan)

		r.mu.Lock()
		defer r.mu.Unlock()

		// Close FFmpeg stdin to signal EOF
		if r.ffmpegStdin != nil {
			r.ffmpegStdin.Close()
			r.ffmpegStdin = nil
		}

		// Wait for FFmpeg to finish and finalize playlist
		if r.ffmpegCmd != nil {
			if err := r.ffmpegCmd.Wait(); err != nil {
				log.Printf("[VideoRecorder] FFmpeg error: %v", err)
			}
		}

		// Update database record with final metadata
		if r.playlistID != "" {
			duration := time.Since(r.startTime).Seconds()
			metadata := map[string]interface{}{
				"status":           "completed",
				"duration_seconds": duration,
				"start_time":       r.startTime.Format(time.RFC3339),
				"end_time":         time.Now().Format(time.RFC3339),
			}

			// Count segments and get total size
			playlistDir := filepath.Join(r.storageManager.backend.(*LocalStorage).baseDir,
				"hls", r.playlistID)
			if segments, err := filepath.Glob(filepath.Join(playlistDir, "segment_*.ts")); err == nil {
				metadata["segment_count"] = len(segments)

				// Calculate total size
				var totalSize int64
				for _, seg := range segments {
					if info, err := os.Stat(seg); err == nil {
						totalSize += info.Size()
					}
				}
				metadata["total_bytes"] = totalSize

				if err := r.storageTable.UpdateStorage(r.playlistID, totalSize, metadata); err != nil {
					log.Printf("[VideoRecorder] Failed to update video record: %v", err)
				}
			}
		}

		log.Printf("[VideoRecorder] Closed HLS recorder for service %s", r.serviceID)
	})
}
