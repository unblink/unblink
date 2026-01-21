package relay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StorageBackend defines the interface for frame storage
type StorageBackend interface {
	Store(frameID string, serviceID string, data []byte) (string, error)
	StoreVideo(videoID string, serviceID string, data []byte) (string, error)
	Retrieve(storagePath string) ([]byte, error)
	Delete(storagePath string) error
}

// LocalStorage implements StorageBackend for local filesystem
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a new LocalStorage backend
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// Store stores a frame to the local filesystem
func (s *LocalStorage) Store(frameID string, serviceID string, data []byte) (string, error) {
	// Create frames directory if it doesn't exist
	framesDir := filepath.Join(s.baseDir, "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create frames directory: %w", err)
	}

	// Create file path
	fileName := fmt.Sprintf("%s.jpg", frameID)
	filePath := filepath.Join(framesDir, fileName)

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write frame file: %w", err)
	}

	// Return storage path in local:// format
	storagePath := fmt.Sprintf("local://frames/%s", fileName)
	return storagePath, nil
}

// Retrieve retrieves a frame from the local filesystem
func (s *LocalStorage) Retrieve(storagePath string) ([]byte, error) {
	// Parse storage path
	if !strings.HasPrefix(storagePath, "local://") {
		return nil, fmt.Errorf("invalid storage path format: %s", storagePath)
	}

	// Extract file path from storage path
	filePath := strings.TrimPrefix(storagePath, "local://")
	fullPath := filepath.Join(s.baseDir, filePath)

	// Read file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read frame file: %w", err)
	}

	return data, nil
}

// Delete deletes a frame from the local filesystem
func (s *LocalStorage) Delete(storagePath string) error {
	// Parse storage path
	if !strings.HasPrefix(storagePath, "local://") {
		return fmt.Errorf("invalid storage path format: %s", storagePath)
	}

	// Extract file path from storage path
	filePath := strings.TrimPrefix(storagePath, "local://")
	fullPath := filepath.Join(s.baseDir, filePath)

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete frame file: %w", err)
	}

	return nil
}

// StoreVideo stores a video file to the local filesystem
func (s *LocalStorage) StoreVideo(videoID string, serviceID string, data []byte) (string, error) {
	// Create videos directory if it doesn't exist
	videosDir := filepath.Join(s.baseDir, "videos", serviceID)
	if err := os.MkdirAll(videosDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create videos directory: %w", err)
	}

	// Create file path
	fileName := fmt.Sprintf("%s.mp4", videoID)
	filePath := filepath.Join(videosDir, fileName)

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write video file: %w", err)
	}

	// Return storage path in local:// format
	storagePath := fmt.Sprintf("local://videos/%s/%s", serviceID, fileName)
	return storagePath, nil
}

// StorageManager manages frame storage operations
type StorageManager struct {
	backend StorageBackend
}

// NewStorageManager creates a new StorageManager
func NewStorageManager(backend StorageBackend) *StorageManager {
	return &StorageManager{backend: backend}
}

// Store stores a frame and returns the storage path
func (m *StorageManager) Store(frameID string, serviceID string, data []byte) (string, error) {
	return m.backend.Store(frameID, serviceID, data)
}

// Retrieve retrieves a frame by storage path
func (m *StorageManager) Retrieve(storagePath string) ([]byte, error) {
	return m.backend.Retrieve(storagePath)
}

// Delete deletes a frame by storage path
func (m *StorageManager) Delete(storagePath string) error {
	return m.backend.Delete(storagePath)
}

// StoreVideo stores a video and returns the storage path
func (m *StorageManager) StoreVideo(videoID string, serviceID string, data []byte) (string, error) {
	return m.backend.StoreVideo(videoID, serviceID, data)
}
