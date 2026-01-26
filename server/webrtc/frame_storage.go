package webrtc

import (
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// FrameStorage handles saving frames to disk
type FrameStorage struct {
	baseDir string // Base directory for frame storage (empty = no storage)
}

// NewFrameStorage creates a new frame storage handler
func NewFrameStorage(baseDir string) *FrameStorage {
	return &FrameStorage{
		baseDir: baseDir,
	}
}

// Save saves a frame to disk if baseDir is configured
func (fs *FrameStorage) Save(serviceID string, frame *Frame) {
	if fs.baseDir == "" {
		log.Printf("[FrameStorage] Skipping frame save for service %s - no baseDir configured", serviceID)
		return
	}

	serviceFramesDir := filepath.Join(fs.baseDir, serviceID)
	if err := os.MkdirAll(serviceFramesDir, 0755); err != nil {
		log.Printf("[FrameStorage] Failed to create frames directory %s: %v", serviceFramesDir, err)
		return
	}

	frameID := uuid.New().String()
	framePath := filepath.Join(serviceFramesDir, frameID+".jpg")
	if err := os.WriteFile(framePath, frame.Data, 0644); err != nil {
		log.Printf("[FrameStorage] Failed to save frame %s: %v", frameID, err)
	} else {
		log.Printf("[FrameStorage] Saved frame seq=%d service=%s size=%d path=%s", frame.Sequence, serviceID, len(frame.Data), framePath)
	}
}
