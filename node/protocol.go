package node

import (
	"encoding/json"
	"os"

	"github.com/fxamacker/cbor/v2"
)

// Message is the top-level protocol message.
// Every message has a unique msg_id and either control or data payload.
type Message struct {
	MsgID   string      `cbor:"msg_id"`
	Control *ControlMsg `cbor:"control,omitempty"`
	Data    *DataMsg    `cbor:"data,omitempty"`
}

// ControlMsg represents control plane messages (register, announce, ack, etc.)
type ControlMsg struct {
	Type         string    `cbor:"type"`                    // register, announce, ack, open_bridge, close_bridge, connection_ready, auth_token
	NodeID       string    `cbor:"node_id,omitempty"`       // For register, connection_ready
	AckMsgID     string    `cbor:"ack_msg_id,omitempty"`    // For ack
	BridgeID     string    `cbor:"bridge_id,omitempty"`     // For open_bridge, close_bridge
	Services     []Service `cbor:"services,omitempty"`      // For announce
	Service      *Service  `cbor:"service,omitempty"`       // For open_bridge
	DashboardURL string    `cbor:"dashboard_url,omitempty"` // For connection_ready
	Token        string    `cbor:"token,omitempty"`         // For register, auth_token
	AuthURL      string    `cbor:"auth_url,omitempty"`      // For auth_token
	ErrorCode    string    `cbor:"error_code,omitempty"`    // For register_error
	ErrorMsg     string    `cbor:"error_msg,omitempty"`     // For register_error
}

// DataMsg represents data plane messages (payload forwarding)
type DataMsg struct {
	BridgeID string `cbor:"bridge_id"`
	Payload  []byte `cbor:"payload"`
}

// Auth represents authentication configuration for a service
type Auth struct {
	Type     string `cbor:"type" json:"type"` // "username_and_password"
	Username string `cbor:"username,omitempty" json:"username,omitempty"`
	Password string `cbor:"password,omitempty" json:"password,omitempty"`
}

// Service represents a target service reachable by a node
type Service struct {
	ID   string `cbor:"id" json:"id"`
	Name string `cbor:"name,omitempty" json:"name,omitempty"` // Human-readable name
	Type string `cbor:"type,omitempty" json:"type,omitempty"` // Examples: "rtsp", "mjpeg", etc.
	Addr string `cbor:"addr" json:"addr"`
	Port int    `cbor:"port" json:"port"`
	Path string `cbor:"path,omitempty" json:"path,omitempty"`
	Auth *Auth  `cbor:"auth,omitempty" json:"auth,omitempty"`
}

// AllowList is a node's allowlist of services it can proxy (for production)
type AllowList struct {
	Services []Service `json:"services"`
}

// LoadAllowList loads an allowlist from a JSON file
func LoadAllowList(path string) (*AllowList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var allowList AllowList
	if err := json.Unmarshal(data, &allowList); err != nil {
		return nil, err
	}
	return &allowList, nil
}

// Auth type constants
const (
	AuthTypeUsernamePassword = "username_and_password"
)

// Control message type constants
const (
	MsgTypeRegister            = "register"
	MsgTypeAnnounce            = "announce"
	MsgTypeAck                 = "ack"
	MsgTypeOpenBridge          = "open_bridge"
	MsgTypeCloseBridge         = "close_bridge"
	MsgTypeConnectionReady     = "connection_ready"
	MsgTypeReqAuthorizationURL = "req_authorization_url" // Node requests authorization URL
	MsgTypeResAuthorizationURL = "res_authorization_url" // Relay responds with authorization URL
	MsgTypeAuthToken           = "auth_token"            // Relay sends token after authorization
	MsgTypeRegisterError       = "register_error"        // Relay sends registration error
)

// NewRegisterMsg creates a REGISTER message
func NewRegisterMsg(msgID, nodeID string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:   MsgTypeRegister,
			NodeID: nodeID,
		},
	}
}

// NewAnnounceMsg creates an ANNOUNCE message
func NewAnnounceMsg(msgID string, services []Service) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:     MsgTypeAnnounce,
			Services: services,
		},
	}
}

// NewAckMsg creates an ACK message
func NewAckMsg(msgID, ackMsgID string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:     MsgTypeAck,
			AckMsgID: ackMsgID,
		},
	}
}

// NewOpenBridgeMsg creates an OPEN_BRIDGE message
func NewOpenBridgeMsg(msgID, bridgeID string, service *Service) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:     MsgTypeOpenBridge,
			BridgeID: bridgeID,
			Service:  service,
		},
	}
}

// NewCloseBridgeMsg creates a CLOSE_BRIDGE message
func NewCloseBridgeMsg(msgID, bridgeID string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:     MsgTypeCloseBridge,
			BridgeID: bridgeID,
		},
	}
}

// NewConnectionReadyMsg creates a CONNECTION_READY message (sent by relay to node)
func NewConnectionReadyMsg(msgID, nodeID, dashboardURL string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:         MsgTypeConnectionReady,
			NodeID:       nodeID,
			DashboardURL: dashboardURL,
		},
	}
}

// NewReqAuthorizationURLMsg creates a REQ_AUTHORIZATION_URL message (sent by node when it has no token)
func NewReqAuthorizationURLMsg(msgID, nodeID string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:   MsgTypeReqAuthorizationURL,
			NodeID: nodeID,
		},
	}
}

// NewResAuthorizationURLMsg creates a RES_AUTHORIZATION_URL message (sent by relay to node)
func NewResAuthorizationURLMsg(msgID, authURL string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:    MsgTypeResAuthorizationURL,
			AuthURL: authURL,
		},
	}
}

// NewAuthTokenMsg creates an AUTH_TOKEN message (sent by relay to node after authorization)
func NewAuthTokenMsg(msgID, token string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:  MsgTypeAuthToken,
			Token: token,
		},
	}
}

// NewRegisterErrorMsg creates a REGISTER_ERROR message (sent by relay to node)
func NewRegisterErrorMsg(msgID, errorCode, errorMsg string) *Message {
	return &Message{
		MsgID: msgID,
		Control: &ControlMsg{
			Type:      MsgTypeRegisterError,
			ErrorCode: errorCode,
			ErrorMsg:  errorMsg,
		},
	}
}

// NewDataMsg creates a DATA message
func NewDataMsg(msgID, bridgeID string, payload []byte) *Message {
	return &Message{
		MsgID: msgID,
		Data: &DataMsg{
			BridgeID: bridgeID,
			Payload:  payload,
		},
	}
}

// Encode serializes a message to CBOR
func (m *Message) Encode() ([]byte, error) {
	return cbor.Marshal(m)
}

// DecodeMessage deserializes a CBOR message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := cbor.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// IsControl returns true if this is a control message
func (m *Message) IsControl() bool {
	return m.Control != nil
}

// IsData returns true if this is a data message
func (m *Message) IsData() bool {
	return m.Data != nil
}

// ControlType returns the control message type, or empty string if not a control message
func (m *Message) ControlType() string {
	if m.Control == nil {
		return ""
	}
	return m.Control.Type
}
