package relay

import (
	"fmt"
	"log"
	"net"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/rtsp"
)

// RTSPSource handles RTSP stream sources
type RTSPSource struct {
	client    *rtsp.Conn
	receivers []*core.Receiver
}

// NewRTSPSourceWithBridge creates a new RTSP source using a direct bridge connection
func NewRTSPSourceWithBridge(service *Service, bridgeID string, bridgeConn net.Conn) (*RTSPSource, error) {

	// Log service record for debugging
	log.Printf("[RTSP] Starting RTSP source with service record:")
	log.Printf("[RTSP]   ID: %s", service.ID)
	log.Printf("[RTSP]   Name: %s", service.Name)
	log.Printf("[RTSP]   Type: %s", service.Type())
	log.Printf("[RTSP]   NodeID: %s", service.NodeID)
	log.Printf("[RTSP]   Addr: %s", service.Addr())
	log.Printf("[RTSP]   Port: %d", service.Port())
	log.Printf("[RTSP]   Path: %s", service.Path())
	log.Printf("[RTSP]   AuthUsername: %s", service.AuthUsername())

	// Use stored service URL directly
	rtspURL := service.ServiceURL

	log.Printf("[RTSP] Using direct bridge connection (BridgeID: %s)", bridgeID)
	log.Printf("[RTSP] RTSP URL: %s", rtspURL)

	// Create RTSP client
	client := rtsp.NewClient(rtspURL)

	// Inject custom bridge connection
	log.Printf("[RTSP] Injecting custom bridge connection")
	client.SetConn(bridgeConn)

	log.Printf("[RTSP] Dialing RTSP server...")

	// Connect to RTSP server
	if err := client.Dial(); err != nil {
		log.Printf("[RTSP] Dial failed: %v", err)
		return nil, fmt.Errorf("RTSP dial: %w", err)
	}

	log.Printf("[RTSP] Dial successful, sending DESCRIBE...")

	// Describe to get media info
	if err := client.Describe(); err != nil {
		client.Close()
		log.Printf("[RTSP] Describe failed: %v", err)
		return nil, fmt.Errorf("RTSP describe: %w", err)
	}

	// Get medias from RTSP
	rtspMedias := client.GetMedias()
	if len(rtspMedias) == 0 {
		client.Close()
		return nil, fmt.Errorf("no media streams in RTSP")
	}

	// Log available media
	for _, media := range rtspMedias {
		for _, codec := range media.Codecs {
			log.Printf("[RTSP] Available media: %s/%s", media.Kind, codec.Name)
		}
	}

	// Get receivers for all media/codecs (first codec per media)
	var receivers []*core.Receiver
	for _, media := range rtspMedias {
		for _, codec := range media.Codecs {
			receiver, err := client.GetTrack(media, codec)
			if err != nil {
				log.Printf("[RTSP] GetTrack error for %s/%s: %v", media.Kind, codec.Name, err)
				continue
			}
			receivers = append(receivers, receiver)
			log.Printf("[RTSP] Added track: %s/%s", media.Kind, codec.Name)
			break // Use first codec per media
		}
	}

	if len(receivers) == 0 {
		client.Close()
		return nil, fmt.Errorf("no codecs found")
	}

	// Start RTSP playback
	log.Printf("[RTSP] Starting PLAY...")
	if err := client.Play(); err != nil {
		client.Close()
		log.Printf("[RTSP] Play failed: %v", err)
		return nil, fmt.Errorf("RTSP play: %w", err)
	}

	log.Printf("[RTSP] Connected successfully, %d track(s)", len(receivers))

	return &RTSPSource{
		client:    client,
		receivers: receivers,
	}, nil
}

// GetProducer returns the RTSP client as a producer
func (s *RTSPSource) GetProducer() core.Producer {
	return s.client
}

// GetReceivers returns the receivers for the RTSP tracks
func (s *RTSPSource) GetReceivers() []*core.Receiver {
	return s.receivers
}

// Close stops the RTSP client
func (s *RTSPSource) Close() {
	if s.client != nil {
		s.client.Close()
		log.Printf("[RTSP] Closed")
	}
}
