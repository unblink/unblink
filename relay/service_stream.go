package relay

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ServiceStream represents a streaming session for one service
// It encapsulates the bridge, media source, and frame extractor
type ServiceStream struct {
	serviceID string
	service   *Service
	nodeConn  *NodeConn
	manager   *AutoStreamManager

	// Streaming components
	bridgeID       string
	mediaSource    MediaSource
	frameExtractor *FrameExtractor
	videoRecorder  *VideoRecorder

	// Lifecycle
	closeChan chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
}

// NewServiceStream creates a new service stream
func NewServiceStream(serviceID string, service *Service, nodeConn *NodeConn, manager *AutoStreamManager, config *Config) (*ServiceStream, error) {
	return &ServiceStream{
		serviceID: serviceID,
		service:   service,
		nodeConn:  nodeConn,
		manager:   manager,
		closeChan: make(chan struct{}),
	}, nil
}

// Start initializes and starts the streaming pipeline
func (s *ServiceStream) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[ServiceStream] Starting stream for service %s", s.serviceID)

	// Open bridge to the service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bridgeID, dataChan, err := s.nodeConn.OpenBridge(ctx, s.service)
	if err != nil {
		return fmt.Errorf("failed to open bridge: %w", err)
	}

	s.bridgeID = bridgeID
	log.Printf("[ServiceStream] Opened bridge %s for service %s", bridgeID, s.serviceID)

	// Create bridge connection wrapper
	bridgeConn := NewBridgeConn(bridgeID, s.nodeConn, dataChan)

	// Create media source from the service
	mediaSource, err := NewMediaSource(s.service, bridgeID, bridgeConn)
	if err != nil {
		// Close bridge on error
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		s.nodeConn.CloseBridge(closeCtx, bridgeID)
		return fmt.Errorf("failed to create media source: %w", err)
	}

	s.mediaSource = mediaSource
	log.Printf("[ServiceStream] Created media source for service %s", s.serviceID)

	// Create frame extractor with callback to manager
	interval := time.Duration(s.manager.config.FrameIntervalSeconds * float64(time.Second))
	s.frameExtractor = NewFrameExtractor(
		s.serviceID,
		interval,
		func(frame *Frame) {
			// Callback to manager for fan-out
			s.manager.onFrame(s.serviceID, frame)
		},
	)

	// Start frame extraction
	if err := s.frameExtractor.Start(s.mediaSource); err != nil {
		// Close everything on error
		s.mediaSource.Close()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		s.nodeConn.CloseBridge(closeCtx, bridgeID)
		return fmt.Errorf("failed to start frame extractor: %w", err)
	}

	// Start video recorder if enabled
	if s.manager.config.VideoRecordingEnabled {
		segmentDuration := time.Duration(s.manager.config.VideoSegmentDurationMinutes) * time.Minute
		s.videoRecorder = NewVideoRecorder(
			s.serviceID,
			segmentDuration,
			s.manager.storageManager,
			s.manager.writeMgr,
		)
		if err := s.videoRecorder.Start(s.mediaSource); err != nil {
			log.Printf("[ServiceStream] Warning: failed to start video recorder: %v", err)
			// Non-fatal: continue without video recording
		} else {
			log.Printf("[ServiceStream] Started video recording for service %s", s.serviceID)
		}
	}

	// Start producer in background (required to pump data from bridge)
	go func() {
		log.Printf("[ServiceStream] Starting producer for service %s", s.serviceID)
		if err := mediaSource.GetProducer().Start(); err != nil {
			log.Printf("[ServiceStream] Producer stopped/error for service %s: %v", s.serviceID, err)
		}
	}()

	log.Printf("[ServiceStream] Started frame extraction for service %s", s.serviceID)
	return nil
}

// Close stops the stream and cleans up all resources
func (s *ServiceStream) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		log.Printf("[ServiceStream] Closing stream for service %s", s.serviceID)
		close(s.closeChan)

		// Close frame extractor
		if s.frameExtractor != nil {
			s.frameExtractor.Close()
		}

		// Close video recorder
		if s.videoRecorder != nil {
			s.videoRecorder.Close()
		}

		// Close media source
		if s.mediaSource != nil {
			s.mediaSource.Close()
		}

		// Close bridge
		if s.bridgeID != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.nodeConn.CloseBridge(ctx, s.bridgeID); err != nil {
				log.Printf("[ServiceStream] Error closing bridge: %v", err)
			}
		}

		log.Printf("[ServiceStream] Closed stream for service %s", s.serviceID)
	})
}
