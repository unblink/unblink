package relay

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	webrtcv1 "github.com/unblink/unblink/relay/gen/unblink/webrtc/v1"
)

// WebRTCService implements the WebRTCServiceHandler interface
type WebRTCService struct {
	relay *Relay
}

func NewWebRTCService(relay *Relay) *WebRTCService {
	return &WebRTCService{relay: relay}
}

// CreateWebRTCSession creates a WebRTC session for streaming
func (s *WebRTCService) CreateWebRTCSession(
	ctx context.Context,
	req *connect.Request[webrtcv1.CreateWebRTCSessionRequest],
) (*connect.Response[webrtcv1.CreateWebRTCSessionResponse], error) {
	nodeID := req.Msg.NodeId
	serviceID := req.Msg.ServiceId
	sdp := req.Msg.Sdp

	log.Printf("[WebRTCService] WebRTC offer for service %s (node %s)", serviceID, nodeID)

	// Validate request
	if sdp == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("sdp is required"))
	}
	if serviceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("service_id is required"))
	}
	if nodeID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("node_id is required"))
	}

	// Get service
	service, err := s.relay.ServiceTable.GetService(serviceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("service not found: %s", serviceID))
	}

	// Verify service belongs to this node
	if service.NodeID != nodeID {
		return nil, connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("service does not belong to this node"))
	}

	// Get WebRTC session manager
	sessionMgr := s.relay.WebRTCSessionMgr
	if sessionMgr == nil {
		log.Printf("[WebRTCService] WebRTC session manager not initialized")
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("WebRTC not available"))
	}

	// Create WebRTC session
	sessionID, answerSDP, err := sessionMgr.NewSession(sdp, service)
	if err != nil {
		log.Printf("[WebRTCService] WebRTC session failed: %v", err)
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("failed to create session: %w", err))
	}

	log.Printf("[WebRTCService] WebRTC session %s created for service %s", sessionID, serviceID)

	return connect.NewResponse(&webrtcv1.CreateWebRTCSessionResponse{
		Type: "answer",
		Sdp:  answerSDP,
	}), nil
}
