package webrtc

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

// Frame represents a single extracted JPEG frame
type Frame struct {
	Data      []byte    // JPEG bytes
	Timestamp time.Time // When the frame was extracted
	ServiceID string    // Service identifier (e.g., camera name)
	Sequence  int64     // Monotonically increasing sequence number
}

// FrameExtractor extracts JPEG frames from H.264 streams using FFmpeg
type FrameExtractor struct {
	serviceID string
	interval  time.Duration
	onFrame   func(*Frame) // Callback when frame is ready
	closeChan chan struct{}
	closeOnce sync.Once

	// FFmpeg pipeline
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser

	// Frame sequencing
	sequence int64
	mu       sync.Mutex
}

// NewFrameExtractor creates a new frame extractor
func NewFrameExtractor(serviceID string, interval time.Duration, onFrame func(*Frame)) *FrameExtractor {
	return &FrameExtractor{
		serviceID: serviceID,
		interval:  interval,
		onFrame:   onFrame,
		closeChan: make(chan struct{}),
	}
}

// Start begins extracting frames from the media source
func (e *FrameExtractor) Start(mediaSource MediaSource) error {
	log.Printf("[FrameExtractor] Starting frame extraction for service %s (interval=%v)", e.serviceID, e.interval)

	// Start FFmpeg process
	if err := e.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Get producer from media source
	producer := mediaSource.GetProducer()
	if producer == nil {
		return fmt.Errorf("media source has no producer")
	}

	// Start H.264 packet consumer that pipes to FFmpeg
	go e.consumeH264ToFFmpeg(producer)

	// Start JPEG frame reader from FFmpeg
	go e.readFramesFromFFmpeg()

	return nil
}

// startFFmpeg starts the FFmpeg process for H.264 to JPEG conversion
func (e *FrameExtractor) startFFmpeg() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate fps from interval (e.g., 5s interval = 1/5 fps = 0.2 fps)
	fps := 1.0 / e.interval.Seconds()

	// FFmpeg command: H.264 stdin → JPEG frames at specified fps
	// Simplified command matching working unbLink implementation
	e.ffmpegCmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error", // Only log errors
		"-f", "mpegts", // Input format (MPEG-TS from go2rtc)
		"-i", "pipe:0", // Read from stdin
		"-vf", fmt.Sprintf("fps=%.3f", fps), // Extract frames at interval
		"-f", "image2pipe", // Output image stream
		"-c:v", "mjpeg", // JPEG codec
		"-q:v", "2", // Quality (1-31, lower is better)
		"pipe:1", // Write to stdout
	)

	var err error
	e.ffmpegStdin, err = e.ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	e.ffmpegStdout, err = e.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	e.ffmpegCmd.Stderr = os.Stderr

	if err := e.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	log.Printf("[FrameExtractor] Started FFmpeg for service %s (fps=%.3f)", e.serviceID, fps)
	return nil
}

// consumeH264ToFFmpeg reads H.264 packets and pipes them to FFmpeg using MPEG-TS
func (e *FrameExtractor) consumeH264ToFFmpeg(producer core.Producer) {
	defer log.Printf("[FrameExtractor] Stopped H.264 consumer for service %s", e.serviceID)

	// Create MPEG-TS consumer using shared helper
	tsConsumer, err := NewMPEGTSConsumer(producer, e.serviceID)
	if err != nil {
		log.Printf("[FrameExtractor] Failed to create MPEG-TS consumer: %v", err)
		return
	}
	defer tsConsumer.Stop()

	log.Printf("[FrameExtractor] Starting to write MPEG-TS to FFmpeg for service %s", e.serviceID)

	// Start a goroutine to monitor write progress
	writtenCh := make(chan int64, 1)
	go func() {
		var lastWritten int64
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-e.closeChan:
				return
			case w := <-writtenCh:
				lastWritten = w
				log.Printf("[FrameExtractor] Total MPEG-TS written: %d bytes", w)
			case <-ticker.C:
				if lastWritten > 0 {
					log.Printf("[FrameExtractor] Still writing, total so far: %d bytes", lastWritten)
				} else {
					log.Printf("[FrameExtractor] WARNING: No MPEG-TS data written yet for service %s", e.serviceID)
				}
			}
		}
	}()

	// Wrap ffmpegStdin to monitor writes
	watchedWriter := &watchWriter{
		w:         e.ffmpegStdin,
		writtenCh: writtenCh,
	}

	// consumer.WriteTo blocks until error
	written, err := tsConsumer.WriteTo(watchedWriter)
	if err != nil {
		log.Printf("[FrameExtractor] MPEG-TS writer finished: written=%d err=%v", written, err)
	} else {
		log.Printf("[FrameExtractor] MPEG-TS writer finished: written=%d", written)
	}
}

// watchWriter wraps io.Writer to report write progress
type watchWriter struct {
	w         io.Writer
	writtenCh chan<- int64
	total     int64
}

func (w *watchWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		w.total += int64(n)
		// Send update every ~100KB
		if w.total%100000 < int64(n) {
			select {
			case w.writtenCh <- w.total:
			default:
			}
		}
	}
	return n, err
}

// readFramesFromFFmpeg reads JPEG frames from FFmpeg stdout
func (e *FrameExtractor) readFramesFromFFmpeg() {
	defer log.Printf("[FrameExtractor] Stopped JPEG reader for service %s", e.serviceID)

	// JPEG SOI (Start of Image) marker
	soi := []byte{0xFF, 0xD8}
	// JPEG EOI (End of Image) marker
	eoi := []byte{0xFF, 0xD9}

	var frameBuffer bytes.Buffer
	buf := make([]byte, 4096)
	inFrame := false
	totalRead := 0

	log.Printf("[FrameExtractor] Starting to read JPEG frames from FFmpeg for service %s", e.serviceID)

	for {
		select {
		case <-e.closeChan:
			log.Printf("[FrameExtractor] JPEG reader closing (total read: %d bytes)", totalRead)
			return
		default:
		}

		n, err := e.ffmpegStdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[FrameExtractor] Error reading from FFmpeg output pipe: %v", err)
			} else {
				log.Printf("[FrameExtractor] FFmpeg stdout closed (total read: %d bytes)", totalRead)
			}
			return
		}

		totalRead += n
		if totalRead%100000 < n { // Log every ~100KB
			log.Printf("[FrameExtractor] Read %d bytes from FFmpeg (service %s)", totalRead, e.serviceID)
		}

		for i := 0; i < n; i++ {
			b := buf[i]
			frameBuffer.WriteByte(b)

			// Check for SOI marker
			if frameBuffer.Len() >= 2 {
				last2 := frameBuffer.Bytes()[frameBuffer.Len()-2:]
				if bytes.Equal(last2, soi) && !inFrame {
					inFrame = true
					frameBuffer.Reset()
					frameBuffer.Write(soi)
				}
			}

			// Check for EOI marker
			if inFrame && frameBuffer.Len() >= 2 {
				last2 := frameBuffer.Bytes()[frameBuffer.Len()-2:]
				if bytes.Equal(last2, eoi) {
					// Complete JPEG frame
					frameData := make([]byte, frameBuffer.Len())
					copy(frameData, frameBuffer.Bytes())

					// Increment sequence
					e.mu.Lock()
					e.sequence++
					seq := e.sequence
					e.mu.Unlock()

					// Create frame
					frame := &Frame{
						Data:      frameData,
						Timestamp: time.Now(),
						ServiceID: e.serviceID,
						Sequence:  seq,
					}

					log.Printf("[FrameExtractor] Extracted JPEG frame %d (%d bytes)", seq, len(frameData))

					// Call callback
					if e.onFrame != nil {
						e.onFrame(frame)
					}

					// Reset for next frame
					frameBuffer.Reset()
					inFrame = false
				}
			}
		}
	}
}

// Close stops the frame extractor
func (e *FrameExtractor) Close() {
	e.closeOnce.Do(func() {
		log.Printf("[FrameExtractor] Closing frame extractor for service %s", e.serviceID)
		close(e.closeChan)

		// Wait a bit for goroutines to finish
		time.Sleep(100 * time.Millisecond)

		// Kill FFmpeg if still running
		if e.ffmpegCmd != nil && e.ffmpegCmd.Process != nil {
			e.ffmpegCmd.Process.Kill()
			e.ffmpegCmd.Wait()
		}
	})
}
