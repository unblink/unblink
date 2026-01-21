package shared

import (
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// Message is the common header for all protocol messages
type Message struct {
	Type  string `json:"type"`  // Message type identifier
	ID    uint64 `json:"id"`    // Unique message ID for request/response matching
	Error string `json:"error"` // Error message if request failed
}

// Message type constants
const (
	// Registration messages
	MessageTypeRegisterRequest     = "register_request"
	MessageTypeRegisterResponse    = "register_response"
	MessageTypeRequestAuthRequest  = "request_auth_request"
	MessageTypeRequestAuthResponse = "request_auth_response"
	MessageTypeReceiveToken        = "receive_token"
	MessageTypeNodeReady           = "node_ready"

	// Bridge messages
	MessageTypeOpenBridgeRequest   = "open_bridge_request"
	MessageTypeOpenBridgeResponse  = "open_bridge_response"
	MessageTypeCloseBridgeRequest  = "close_bridge_request"
	MessageTypeCloseBridgeResponse = "close_bridge_response"

	// Data messages (streaming)
	MessageTypeBridgeData = "bridge_data"
)

// Messager is the interface that all message types implement
type Messager interface {
	GetType() string
	GetID() uint64
}

// GetType returns the message type (implements Messager)
func (m *Message) GetType() string {
	return m.Type
}

// GetID returns the message ID (implements Messager)
func (m *Message) GetID() uint64 {
	return m.ID
}

// GetMessager safely extracts Messager interface from any message type
func GetMessager(msg interface{}) (Messager, error) {
	if m, ok := msg.(Messager); ok {
		return m, nil
	}
	return nil, fmt.Errorf("message %T does not implement Messager interface")
}

// ============================================================
// Registration Messages
// ============================================================

// RegisterRequest is sent by a node to register with the relay
type RegisterRequest struct {
	Message
	NodeID       string   `json:"node_id"`
	Token        string   `json:"token"`
	Hostname     string   `json:"hostname,omitempty"`
	MACAddresses []string `json:"mac_addresses,omitempty"`
}

// RegisterResponse is sent by relay in response to RegisterRequest
type RegisterResponse struct {
	Message
	Success bool `json:"success"`
	// Error field from Message provides failure details
}

// ============================================================
// Authorization Messages
// ============================================================

// RequestAuthRequest is sent by a node when it has no token
type RequestAuthRequest struct {
	Message
	NodeID string `json:"node_id"`
}

// RequestAuthResponse is sent by relay with the authorization URL
type RequestAuthResponse struct {
	Message
	Success bool   `json:"success"`
	AuthURL string `json:"auth_url"`
}

// ReceiveTokenMessage is sent by relay when user authorizes the node
type ReceiveTokenMessage struct {
	Message
	Token string `json:"token"`
}

// NodeReadyMessage is sent by node after registration to signal it's ready for bridges
type NodeReadyMessage struct {
	Message
}

// ============================================================
// Bridge Messages
// ============================================================

// Service represents a target service reachable by a node
type Service struct {
	ServiceURL string `json:"service_url"`
}

// OpenBridgeRequest is sent by relay to request node to open a bridge
type OpenBridgeRequest struct {
	Message
	BridgeID  string  `json:"bridge_id"`
	ServiceID string  `json:"service_id"`
	Service   Service `json:"service"`
}

// OpenBridgeResponse is sent by node in response to OpenBridgeRequest
type OpenBridgeResponse struct {
	Message
	Success bool `json:"success"`
}

// CloseBridgeRequest is sent by relay to request node to close a bridge
type CloseBridgeRequest struct {
	Message
	BridgeID string `json:"bridge_id"`
}

// CloseBridgeResponse is sent by node in response to CloseBridgeRequest
type CloseBridgeResponse struct {
	Message
	Success bool `json:"success"`
}

// ============================================================
// Data Messages
// ============================================================

// BridgeDataMessage carries bidirectional data for a bridge
type BridgeDataMessage struct {
	Message
	BridgeID string `json:"bridge_id"`
	Sequence uint64 `json:"sequence"`
	Data     []byte `json:"data"`
}

// ============================================================
// Encoding/Decoding
// ============================================================

// Encode serializes a message to CBOR
func EncodeMessage(msg interface{}) ([]byte, error) {
	return cbor.Marshal(msg)
}

// DecodeMessage deserializes CBOR data into a Message
func DecodeMessage(data []byte) (interface{}, error) {
	// First decode just the header to get the message type
	var header Message
	if err := cbor.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode message header: %w", err)
	}

	// Map message types to their concrete types
	typeMap := map[string]interface{}{
		MessageTypeRegisterRequest:     &RegisterRequest{},
		MessageTypeRegisterResponse:    &RegisterResponse{},
		MessageTypeRequestAuthRequest:  &RequestAuthRequest{},
		MessageTypeRequestAuthResponse: &RequestAuthResponse{},
		MessageTypeReceiveToken:        &ReceiveTokenMessage{},
		MessageTypeNodeReady:           &NodeReadyMessage{},
		MessageTypeOpenBridgeRequest:   &OpenBridgeRequest{},
		MessageTypeOpenBridgeResponse:  &OpenBridgeResponse{},
		MessageTypeCloseBridgeRequest:  &CloseBridgeRequest{},
		MessageTypeCloseBridgeResponse: &CloseBridgeResponse{},
		MessageTypeBridgeData:          &BridgeDataMessage{},
	}

	prototype, ok := typeMap[header.Type]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %s", header.Type)
	}

	// Decode into the correct type
	msgValue := reflect.New(reflect.TypeOf(prototype).Elem()).Interface()
	if err := cbor.Unmarshal(data, msgValue); err != nil {
		return nil, fmt.Errorf("decode %s: %w", header.Type, err)
	}

	return msgValue, nil
}
