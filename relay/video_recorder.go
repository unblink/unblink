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
	"github.com/AlexxIT/go2rtc/pkg/mpegts"
	"github.com/google/uuid"
)

// VideoRecorder records continuous video segments from H.264 streams
type VideoRecorder struct {
	serviceID       string
	segmentDuration time.Duration
	storageManager  *StorageManager
	storageTable    *table_storage
	closeChan       chan struct{}
	closeOnce       sync.Once
	mu              sync.Mutex

	// FFmpeg pipeline
	ffmpegCmd        *exec.Cmd
	ffmpegStdin      io.WriteCloser
	currentSegment   *segmentWriter
	segmentStartTime time.Time
}

// segmentWriter manages a single video segment
type segmentWriter struct {
	videoID   string
	filePath  string
	cmd       *exec.Cmd
	startTime time.Time
}

// NewVideoRecorder creates a new video recorder
func NewVideoRecorder(serviceID string, segmentDuration time.Duration,
	storageManager *StorageManager, storageTable *table_storage) *VideoRecorder {
	return &VideoRecorder{
		serviceID:       serviceID,
		segmentDuration: segmentDuration,
		storageManager:  storageManager,
		storageTable:    storageTable,
		closeChan:       make(chan struct{}),
	}
}

// Start begins recording from the media source
func (r *VideoRecorder) Start(mediaSource MediaSource) error {
	log.Printf("[VideoRecorder] Starting video recording for service %s (segment_duration=%v)",
		r.serviceID, r.segmentDuration)

	// Get producer from media source
	producer := mediaSource.GetProducer()
	if producer == nil {
		return fmt.Errorf("media source has no producer")
	}

	// Start first segment
	if err := r.startNewSegment(); err != nil {
		return fmt.Errorf("failed to start first segment: %w", err)
	}

	// Start segment rotation goroutine
	go r.rotateSegments()

	// Start H.264 consumer that pipes to current FFmpeg
	go r.consumeH264ToFFmpeg(producer)

	return nil
}

// startNewSegment starts a new video segment
func (r *VideoRecorder) startNewSegment() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close previous segment if exists
	if r.currentSegment != nil {
		r.finalizeSegment()
	}

	videoID := uuid.New().String()

	// Get base directory from storage manager (read from backend)
	baseDir := r.storageManager.backend.(*LocalStorage).baseDir
	tempPath := filepath.Join(baseDir, "videos", videoID+".tmp.mp4")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
		return fmt.Errorf("failed to create video directory: %w", err)
	}

	// FFmpeg command for MP4 segment recording (remux only, no re-encoding)
	cmd := exec.Command("ffmpeg",
		"-loglevel", "error",
		"-f", "mpegts", // Input format
		"-i", "pipe:0", // Read from stdin
		"-c:v", "copy", // Copy video codec (no re-encoding)
		"-c:a", "aac",  // Convert audio to AAC (if present)
		"-b:a", "128k", // Audio bitrate
		"-movflags", "frag_keyframe+empty_moov+default_base_moof", // Streaming-friendly MP4
		"-f", "mp4",   // Output format
		tempPath,      // Output file
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	r.ffmpegCmd = cmd
	r.ffmpegStdin = stdin
	r.segmentStartTime = time.Now()

	r.currentSegment = &segmentWriter{
		videoID:   videoID,
		filePath:  tempPath,
		cmd:       cmd,
		startTime: time.Now(),
	}

	log.Printf("[VideoRecorder] Started new segment %s for service %s", videoID, r.serviceID)
	return nil
}

// rotateSegments handles segment rotation
func (r *VideoRecorder) rotateSegments() {
	ticker := time.NewTicker(r.segmentDuration)
	defer ticker.Stop()

	for {
		select {
		case <-r.closeChan:
			return
		case <-ticker.C:
			log.Printf("[VideoRecorder] Rotating segment for service %s", r.serviceID)
			if err := r.startNewSegment(); err != nil {
				log.Printf("[VideoRecorder] Failed to rotate segment: %v", err)
			}
		}
	}
}

// consumeH264ToFFmpeg reads H.264 packets and pipes to FFmpeg
func (r *VideoRecorder) consumeH264ToFFmpeg(producer core.Producer) {
	defer log.Printf("[VideoRecorder] Stopped H.264 consumer for service %s", r.serviceID)

	// Create MPEG-TS consumer from go2rtc
	consumer := mpegts.NewConsumer()
	defer consumer.Stop()

	// Find H.264 track (same as FrameExtractor)
	var videoMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindVideo {
			videoMedia = media
			break
		}
	}

	if videoMedia == nil {
		log.Printf("[VideoRecorder] No video media found for service %s", r.serviceID)
		return
	}

	var videoCodec *core.Codec
	for _, codec := range videoMedia.Codecs {
		if codec.Name == core.CodecH264 {
			videoCodec = codec
			break
		}
	}

	if videoCodec == nil {
		log.Printf("[VideoRecorder] No H.264 codec found for service %s", r.serviceID)
		return
	}

	receiver, err := producer.GetTrack(videoMedia, videoCodec)
	if err != nil {
		log.Printf("[VideoRecorder] Failed to get track: %v", err)
		return
	}

	if err := consumer.AddTrack(videoMedia, videoCodec, receiver); err != nil {
		log.Printf("[VideoRecorder] Failed to add track: %v", err)
		return
	}

	log.Printf("[VideoRecorder] Starting MPEG-TS consumer for service %s", r.serviceID)

	// Pipe to current FFmpeg stdin, switching when segment rotates
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

		// Write to current stdin
		// Note: We need to handle the case where stdin changes during WriteTo
		// For now, we'll use a simpler approach: read a chunk and write
		n, err := consumer.WriteTo(stdin)
		if err != nil {
			log.Printf("[VideoRecorder] MPEG-TS writer finished: %v", err)
			return
		}
		if n == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// finalizeSegment finalizes the current segment
func (r *VideoRecorder) finalizeSegment() {
	if r.currentSegment == nil {
		return
	}

	segment := r.currentSegment

	// Close FFmpeg stdin to signal EOF
	if r.ffmpegStdin != nil {
		r.ffmpegStdin.Close()
		r.ffmpegStdin = nil
	}

	// Wait for FFmpeg to finish
	if r.ffmpegCmd != nil {
		if err := r.ffmpegCmd.Wait(); err != nil {
			log.Printf("[VideoRecorder] FFmpeg error: %v", err)
		}
	}

	// Read file data
	data, err := os.ReadFile(segment.filePath)
	if err != nil {
		log.Printf("[VideoRecorder] Failed to read segment file: %v", err)
		r.currentSegment = nil
		return
	}

	fileSize := int64(len(data))
	duration := time.Since(segment.startTime).Seconds()

	// Store via storage manager
	storagePath, err := r.storageManager.StoreVideo(segment.videoID, r.serviceID, data)
	if err != nil {
		log.Printf("[VideoRecorder] Failed to store segment: %v", err)
		r.currentSegment = nil
		return
	}

	// Create database record with metadata
	metadata := map[string]interface{}{
		"duration_seconds": duration,
		"start_time":       segment.startTime.Format(time.RFC3339),
		"end_time":         time.Now().Format(time.RFC3339),
	}

	if err := r.storageTable.CreateStorage(segment.videoID, r.serviceID, "video", storagePath,
		segment.startTime, fileSize, "video/mp4", metadata); err != nil {
		log.Printf("[VideoRecorder] Failed to create video record: %v", err)
	}

	log.Printf("[VideoRecorder] Finalized segment %s (%.2fs, %d bytes)",
		segment.videoID, duration, fileSize)

	r.currentSegment = nil
}

// Close stops the recorder
func (r *VideoRecorder) Close() {
	r.closeOnce.Do(func() {
		log.Printf("[VideoRecorder] Closing video recorder for service %s", r.serviceID)
		close(r.closeChan)

		r.mu.Lock()
		r.finalizeSegment()
		r.mu.Unlock()
	})
}
