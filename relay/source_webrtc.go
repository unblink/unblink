package relay

import (
	"fmt"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

// Source represents a media source that can provide a producer and receivers
type Source interface {
	GetProducer() core.Producer
	GetReceivers() []*core.Receiver
	Close()
}

// NewMediaSource creates the appropriate source using direct bridge connection
func NewMediaSource(service *Service, bridgeID string, bridgeConn *BridgeConn) (MediaSource, error) {
	switch service.Type() {
	case "rtsp":
		return NewRTSPSourceWithBridge(service, bridgeID, bridgeConn)
	case "http":
		return NewMJPEGSourceWithBridge(service, bridgeID, bridgeConn)
	case "https":
		return NewMJPEGSourceWithBridge(service, bridgeID, bridgeConn)
	default:
		return nil, fmt.Errorf("unsupported service type: %s", service.Type())
	}
}
