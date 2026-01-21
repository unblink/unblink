package node

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/unblink/unblink/shared"
)

const MaxMessageSize = 16 * 1024 * 1024 // 16 MB

// MessageTransport defines the interface for reading/writing messages
type MessageTransport interface {
	ReadMessage() (interface{}, error)
	WriteMessage(interface{}) error
	Close() error
}

// Conn manages the connection to the relay
type Conn struct {
	config *Config
	wsConn *websocket.Conn

	// Transport
	transport MessageTransport

	// State management
	state   ConnState
	stateMu sync.RWMutex

	// Node state
	nodeID     string
	tokenReady chan struct{}
	shutdown   chan struct{}
	closed     bool
	closeMu    sync.Mutex

	// Bridges
	bridges  map[string]*Bridge
	bridgeMu sync.RWMutex

	// Message ID counter
	msgID uint64
	msgMu sync.Mutex
}

// Bridge represents an active bridge from node to service
type Bridge struct {
	ID        string
	ServiceID string
	Addr      string
	Port      int
	Path      string
	Conn      net.Conn
	shutdown  chan struct{}
	byteCount int64
	msgCount  int64
	closeOnce sync.Once
}

// NewConn creates a new node connection
func NewConn(config *Config) *Conn {
	c := &Conn{
		config:     config,
		shutdown:   make(chan struct{}),
		tokenReady: make(chan struct{}),
		bridges:    make(map[string]*Bridge),
		msgID:      1,
	}
	// Start in disconnected state
	c.state = &DisconnectedState{}
	return c
}

// setState atomically transitions to a new state
func (c *Conn) setState(newState ConnState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	log.Printf("[State] %s -> %s", c.state.Name(), newState.Name())
	c.state = newState
}

// Connect establishes a connection to the relay
func (c *Conn) Connect() error {
	c.stateMu.RLock()
	state := c.state
	c.stateMu.RUnlock()

	return state.Connect(c)
}

// messageLoop reads messages from relay and dispatches them to handlers
func (c *Conn) messageLoop() {
	defer log.Printf("[Conn] Message loop ended")

	for {
		select {
		case <-c.shutdown:
			return
		default:
		}

		msg, err := c.transport.ReadMessage()
		if err != nil {
			log.Printf("[Conn] Read error: %v", err)
			return
		}

		if err := c.handleMessage(msg); err != nil {
			log.Printf("[Conn] Error handling message: %v", err)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler based on its type
func (c *Conn) handleMessage(msg interface{}) error {
	// Extract message info using the Messager interface
	m, err := shared.GetMessager(msg)
	if err != nil {
		return fmt.Errorf("invalid message type: %w", err)
	}

	msgType := m.GetType()

	// Get current state
	c.stateMu.RLock()
	state := c.state
	c.stateMu.RUnlock()

	// Delegate to state
	switch msgType {
	case shared.MessageTypeRegisterResponse:
		return state.OnRegisterResponse(c, msg.(*shared.RegisterResponse))

	case shared.MessageTypeRequestAuthResponse:
		return state.OnAuthResponse(c, msg.(*shared.RequestAuthResponse))

	case shared.MessageTypeReceiveToken:
		return state.OnTokenReceived(c, msg.(*shared.ReceiveTokenMessage))

	case shared.MessageTypeOpenBridgeRequest:
		return state.HandleOpenBridge(c, msg.(*shared.OpenBridgeRequest))

	case shared.MessageTypeCloseBridgeRequest:
		return state.HandleCloseBridge(c, msg.(*shared.CloseBridgeRequest))

	case shared.MessageTypeBridgeData:
		return state.HandleBridgeData(c, msg.(*shared.BridgeDataMessage))

	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

// handleOpenBridge handles an open_bridge_request message from relay
func (c *Conn) handleOpenBridge(req *shared.OpenBridgeRequest) error {
	// Parse the service URL to extract components
	parsed, err := shared.ParseServiceURL(req.Service.ServiceURL)
	if err != nil {
		log.Printf("[Conn] Failed to parse service URL %s: %v", req.Service.ServiceURL, err)

		// Send error response
		resp := &shared.OpenBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeOpenBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("parse service URL: %w", err)
	}

	log.Printf("[Conn] OpenBridge request: bridgeID=%s, service=%s:%d%s",
		req.BridgeID, parsed.Host, parsed.Port, parsed.Path)

	// Connect to the service
	serviceAddr := fmt.Sprintf("%s:%d", parsed.Host, parsed.Port)
	svcConn, err := net.DialTimeout("tcp", serviceAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Conn] Failed to connect to service %s: %v", serviceAddr, err)

		// Send error response
		resp := &shared.OpenBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeOpenBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("connect to service: %w", err)
	}

	log.Printf("[Conn] Connected to service")

	// Create and track bridge
	bridge := &Bridge{
		ID:        req.BridgeID,
		ServiceID: req.ServiceID,
		Addr:      parsed.Host,
		Port:      parsed.Port,
		Path:      parsed.Path,
		Conn:      svcConn,
		shutdown:  make(chan struct{}),
		byteCount: 0,
		msgCount:  0,
	}

	c.bridgeMu.Lock()
	c.bridges[req.BridgeID] = bridge
	c.bridgeMu.Unlock()

	// Send success response
	resp := &shared.OpenBridgeResponse{
		Message: shared.Message{
			Type: shared.MessageTypeOpenBridgeResponse,
			ID:   req.ID,
		},
		Success: true,
	}

	if err := c.transport.WriteMessage(resp); err != nil {
		c.closeBridge(bridge)
		return fmt.Errorf("send open bridge response: %w", err)
	}

	log.Printf("[Conn] Opened bridge %s to %s:%d%s", req.BridgeID, parsed.Host, parsed.Port, parsed.Path)

	// Start forwarding data from service to relay in a goroutine
	go c.forwardServiceToRelay(bridge)

	return nil
}

// handleCloseBridge handles a close_bridge_request message from relay
func (c *Conn) handleCloseBridge(req *shared.CloseBridgeRequest) error {
	log.Printf("[Conn] CloseBridge request: bridgeID=%s", req.BridgeID)

	c.bridgeMu.RLock()
	bridge, exists := c.bridges[req.BridgeID]
	if !exists {
		c.bridgeMu.RUnlock()
		// Send error response
		resp := &shared.CloseBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeCloseBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("bridge not found: %s", req.BridgeID)
	}
	c.bridgeMu.RUnlock()

	// Close the bridge
	c.closeBridge(bridge)

	// Send success response
	resp := &shared.CloseBridgeResponse{
		Message: shared.Message{
			Type: shared.MessageTypeCloseBridgeResponse,
			ID:   req.ID,
		},
		Success: true,
	}

	if err := c.transport.WriteMessage(resp); err != nil {
		return fmt.Errorf("send close bridge response: %w", err)
	}

	log.Printf("[Conn] Closed bridge %s", req.BridgeID)
	return nil
}

// handleBridgeData handles bridge_data messages from relay
func (c *Conn) handleBridgeData(msg *shared.BridgeDataMessage) error {
	c.bridgeMu.RLock()
	bridge, exists := c.bridges[msg.BridgeID]
	c.bridgeMu.RUnlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", msg.BridgeID)
	}

	// Forward data to service
	_, err := bridge.Conn.Write(msg.Data)
	if err != nil {
		log.Printf("[Conn] Write to service failed: %v", err)
		// Close the bridge on write failure
		c.closeBridge(bridge)
		return fmt.Errorf("write to service: %w", err)
	}

	// Update statistics
	atomic.AddInt64(&bridge.byteCount, int64(len(msg.Data)))
	atomic.AddInt64(&bridge.msgCount, 1)

	return nil
}

// forwardServiceToRelay reads data from service and forwards to relay via bridge_data messages
func (c *Conn) forwardServiceToRelay(bridge *Bridge) {
	defer log.Printf("[Conn] Stopped forwarding for bridge %s", bridge.ID)

	buf := make([]byte, 4096)
	var sequence uint64

	log.Printf("[Conn] Started forwarding for bridge %s", bridge.ID)

	for {
		select {
		case <-bridge.shutdown:
			log.Printf("[Conn] Shutdown signal received for bridge %s", bridge.ID)
			return

		default:
			// Set read deadline to allow periodic shutdown checks
			bridge.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, err := bridge.Conn.Read(buf)

			if err != nil {
				// Check if it's a timeout (expected for shutdown checks)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					log.Printf("[Conn] Bridge %s waiting for data...", bridge.ID)
					continue // Try again
				}
				if err != io.EOF {
					log.Printf("[Conn] Read from service error: %v", err)
				}
				return
			}

			if n > 0 {
				sequence++

				// Send data to relay via bridge_data message
				msg := &shared.BridgeDataMessage{
					Message: shared.Message{
						Type: shared.MessageTypeBridgeData,
						ID:   c.getMsgID(),
					},
					BridgeID: bridge.ID,
					Sequence: sequence,
					Data:     buf[:n],
				}

				if err := c.transport.WriteMessage(msg); err != nil {
					log.Printf("[Conn] Failed to send data to relay: %v", err)
					return
				}
			}

			// Log the first few bytes for debugging
			log.Printf("[Conn] Forwarded %d bytes from service on bridge %s: %x", n, bridge.ID, buf[:min(n, 16)])
		}
	}
}

// closeBridge closes a bridge and cleans up resources
func (c *Conn) closeBridge(bridge *Bridge) {
	bridge.closeOnce.Do(func() {
		// Close the TCP connection
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
		}

		// Remove from tracking
		c.bridgeMu.Lock()
		delete(c.bridges, bridge.ID)
		c.bridgeMu.Unlock()
	})
}

// Close closes the connection to the relay
func (c *Conn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	close(c.shutdown)

	// Close all bridges
	c.bridgeMu.Lock()
	for _, bridge := range c.bridges {
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
		}
	}
	c.bridges = make(map[string]*Bridge)
	c.bridgeMu.Unlock()

	// Close transport
	if c.transport != nil {
		c.transport.Close()
	}

	return nil
}

// getMsgID returns the next message ID
func (c *Conn) getMsgID() uint64 {
	c.msgMu.Lock()
	defer c.msgMu.Unlock()

	id := c.msgID
	c.msgID++
	return id
}

// NodeID returns the node's ID
func (c *Conn) NodeID() string {
	return c.config.NodeID
}

// WaitForToken waits for the token to be received
func (c *Conn) WaitForToken(ctx context.Context) bool {
	select {
	case <-c.tokenReady:
		return true
	case <-ctx.Done():
		return false
	}
}

// getSystemInfo collects hostname and MAC addresses
func getSystemInfo() (string, []string, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("[Conn] Failed to get hostname: %v", err)
		hostname = "unknown"
	}

	// Get MAC addresses from network interfaces
	var macs []string
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Skip loopback and down interfaces
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			if len(iface.HardwareAddr) > 0 {
				macs = append(macs, iface.HardwareAddr.String())
			}
		}
	} else {
		log.Printf("[Conn] Failed to get network interfaces: %v", err)
	}

	log.Printf("[Conn] System info: hostname=%q macs=%v", hostname, macs)
	return hostname, macs, nil
}

// sendRegisterRequest sends a register request to the relay
func (c *Conn) sendRegisterRequest() error {
	hostname, macs, _ := getSystemInfo()

	msg := &shared.RegisterRequest{
		Message: shared.Message{
			Type: shared.MessageTypeRegisterRequest,
			ID:   c.getMsgID(),
		},
		NodeID:       c.config.NodeID,
		Token:        c.config.Token,
		Hostname:     hostname,
		MACAddresses: macs,
	}

	log.Printf("[Conn] Sending register request: nodeID=%s", c.config.NodeID)
	return c.transport.WriteMessage(msg)
}

// sendRequestAuth sends an auth request to the relay
func (c *Conn) sendRequestAuth() error {
	msg := &shared.RequestAuthRequest{
		Message: shared.Message{
			Type: shared.MessageTypeRequestAuthRequest,
			ID:   c.getMsgID(),
		},
		NodeID: c.config.NodeID,
	}

	log.Printf("[Conn] Sending auth request")
	return c.transport.WriteMessage(msg)
}

// CreateBridge creates a new bridge to a service (placeholder - not implemented in v2)
func (c *Conn) CreateBridge(serviceID, relayServiceID, addr string, port int, path string) (*Bridge, error) {
	return nil, fmt.Errorf("CreateBridge not implemented - services are database-driven in v2")
}

// GetBridge retrieves a bridge by ID (placeholder - not implemented in v2)
func (c *Conn) GetBridge(bridgeID string) (*Bridge, error) {
	return nil, fmt.Errorf("GetBridge not implemented - services are database-driven in v2")
}

// CloseBridge closes a bridge by ID
func (c *Conn) CloseBridge(bridgeID string) error {
	// Find and remove bridge from tracking
	c.bridgeMu.Lock()
	bridge, exists := c.bridges[bridgeID]
	if exists {
		delete(c.bridges, bridgeID)
	}
	c.bridgeMu.Unlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", bridgeID)
	}

	// Close the bridge if it exists
	if bridge.Conn != nil {
		close(bridge.shutdown)
		bridge.Conn.Close()
		log.Printf("[Conn] Closed bridge %s", bridgeID)
	}

	return nil
}

// CloseAllBridges closes all active bridges
func (c *Conn) CloseAllBridges() error {
	c.bridgeMu.Lock()
	defer c.bridgeMu.Unlock()

	// Close all bridges
	for _, bridge := range c.bridges {
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
			log.Printf("[Conn] Closed bridge %s", bridge.ID)
		}
	}

	c.bridges = make(map[string]*Bridge)
	return nil
}
