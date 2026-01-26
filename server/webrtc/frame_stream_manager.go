package webrtc

import (
	"log"
	"sync"
	"time"
)

// FrameStreamManager manages shared frame extractors per service
// Multiple sessions can view the same service, and they share one frame extractor
type FrameStreamManager struct {
	// Service extractors (one per service being monitored)
	serviceExtractors map[string]*serviceExtractorEntry // key: serviceID
	mu                sync.RWMutex

	// Configuration
	frameInterval time.Duration
}

// serviceExtractorEntry holds the frame extractor and reference count
type serviceExtractorEntry struct {
	extractor      *FrameExtractor
	mediaSource    MediaSource
	refCount       int
	closeChan      chan struct{}
	closeOnce      sync.Once
}

// NewFrameStreamManager creates a new frame stream manager
func NewFrameStreamManager(frameInterval time.Duration) *FrameStreamManager {
	return &FrameStreamManager{
		serviceExtractors: make(map[string]*serviceExtractorEntry),
		frameInterval:     frameInterval,
	}
}

// RegisterSession adds a session to receive frames from the service
// Creates a new extractor if one doesn't exist for this service
func (m *FrameStreamManager) RegisterSession(serviceID string, mediaSource MediaSource, onFrame func(*Frame)) error {
	m.mu.Lock()

	// Check if extractor already exists for this service
	entry, exists := m.serviceExtractors[serviceID]
	if exists {
		// Increment ref count and add callback
		entry.refCount++
		log.Printf("[FrameStreamManager] Registered additional session for service %s (refcount=%d)", serviceID, entry.refCount)
		m.mu.Unlock()

		// Add callback to existing extractor
		// Note: This is a simple implementation - for multiple callbacks, we'd need a fan-out mechanism
		// For now, we just log that frames are being extracted
		return nil
	}

	// Create new frame extractor
	log.Printf("[FrameStreamManager] Creating new frame extractor for service %s", serviceID)

	extractor := NewFrameExtractor(serviceID, m.frameInterval, onFrame)

	entry = &serviceExtractorEntry{
		extractor:   extractor,
		mediaSource: mediaSource,
		refCount:    1,
		closeChan:   make(chan struct{}),
	}

	m.serviceExtractors[serviceID] = entry
	m.mu.Unlock()

	// Start the extractor (outside the lock)
	if err := extractor.Start(mediaSource); err != nil {
		// Remove entry on failure
		m.mu.Lock()
		delete(m.serviceExtractors, serviceID)
		m.mu.Unlock()
		return err
	}

	log.Printf("[FrameStreamManager] Started frame extractor for service %s", serviceID)
	return nil
}

// UnregisterSession removes a session from receiving frames
// Cleans up the extractor if no more sessions are using it
func (m *FrameStreamManager) UnregisterSession(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.serviceExtractors[serviceID]
	if !exists {
		log.Printf("[FrameStreamManager] No extractor found for service %s", serviceID)
		return
	}

	entry.refCount--
	log.Printf("[FrameStreamManager] Unregistered session for service %s (refcount=%d)", serviceID, entry.refCount)

	if entry.refCount <= 0 {
		// No more sessions using this extractor, clean it up
		log.Printf("[FrameStreamManager] No more sessions for service %s, closing extractor", serviceID)
		entry.closeOnce.Do(func() {
			close(entry.closeChan)
			entry.extractor.Close()
		})
		delete(m.serviceExtractors, serviceID)
	}
}

// Close cleans up all frame extractors
func (m *FrameStreamManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[FrameStreamManager] Closing all frame extractors")

	for serviceID, entry := range m.serviceExtractors {
		log.Printf("[FrameStreamManager] Closing extractor for service %s", serviceID)
		entry.closeOnce.Do(func() {
			close(entry.closeChan)
			entry.extractor.Close()
		})
	}

	m.serviceExtractors = make(map[string]*serviceExtractorEntry)
}
