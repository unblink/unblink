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

// normalizePath ensures the path has local:// prefix, adds it if missing (for legacy paths)
func normalizePath(storagePath string) string {
	if strings.Contains(storagePath, "://") {
		return storagePath
	}
	return "local://" + storagePath
}

// parsePath extracts the filesystem path from a storage path (e.g., local://frames/xxx.jpg -> frames/xxx.jpg)
func parsePath(storagePath string) (string, error) {
	if !strings.HasPrefix(storagePath, "local://") {
		return "", fmt.Errorf("storage path must have local:// prefix, got: %s", storagePath)
	}
	return strings.TrimPrefix(storagePath, "local://"), nil
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

// Retrieve retrieves a file from the local filesystem
func (s *LocalStorage) Retrieve(storagePath string) ([]byte, error) {
	// Normalize legacy paths (add local:// prefix if missing)
	storagePath = normalizePath(storagePath)

	// Parse storage path to get filesystem path
	filePath, err := parsePath(storagePath)
	if err != nil {
		return nil, err
	}

	// For HLS paths, we can't retrieve the whole thing (it's a directory with files)
	// HLS is handled directly by StorageHandler.serveHLS()
	if strings.HasPrefix(filePath, "hls/") {
		return nil, fmt.Errorf("HLS storage must be served directly, not retrieved: %s", storagePath)
	}

	fullPath := filepath.Join(s.baseDir, filePath)

	// Read file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// Delete deletes a file from the local filesystem
func (s *LocalStorage) Delete(storagePath string) error {
	// Normalize legacy paths (add local:// prefix if missing)
	storagePath = normalizePath(storagePath)

	// Parse storage path to get filesystem path
	filePath, err := parsePath(storagePath)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(s.baseDir, filePath)

	// Delete file or directory
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

// StoreVideo stores a video file to the local filesystem
func (s *LocalStorage) StoreVideo(videoID string, serviceID string, data []byte) (string, error) {
	// Create videos directory if it doesn't exist
	videosDir := filepath.Join(s.baseDir, "videos")
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
	storagePath := fmt.Sprintf("local://videos/%s", fileName)
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

// Retrieve retrieves a file by storage path
func (m *StorageManager) Retrieve(storagePath string) ([]byte, error) {
	return m.backend.Retrieve(storagePath)
}

// Delete deletes a file by storage path
func (m *StorageManager) Delete(storagePath string) error {
	return m.backend.Delete(storagePath)
}

// StoreVideo stores a video and returns the storage path
func (m *StorageManager) StoreVideo(videoID string, serviceID string, data []byte) (string, error) {
	return m.backend.StoreVideo(videoID, serviceID, data)
}
