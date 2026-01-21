package relay

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/unblink/unblink/shared"
)

const (
	dataChanBufferSize = 100 // Number of chunks to buffer per bridge
)

// Bridge represents an active data bridge with streaming capability
type Bridge struct {
	ID        string
	ServiceID string
	Service   *Service
	ByteCount int64
	MsgCount  int64
	DataChan  chan []byte // Channel for receiving data FROM the node
}

// NodeConn handles a single node connection using CBOR messages
type NodeConn struct {
	wsConn      *websocket.Conn
	transport   MessageTransport
	relay       *Relay
	nodeID      string
	authToken   string
	bridges     map[string]*Bridge
	bridgeMu    sync.RWMutex
	bridgeChans map[string]chan []byte
	shutdown    chan struct{}
	closed      bool
	closeMu     sync.Mutex

	// Message ID counter and pending requests
	msgID           uint64
	msgMu           sync.Mutex
	pendingRequests map[uint64]chan interface{}
	pendingMu       sync.Mutex

	// Ready state (set when node sends node_ready message)
	ready int32 // atomic: 0 = not ready, 1 = ready
}

// NewNodeConn creates a new node connection from an HTTP WebSocket upgrade
func NewNodeConn(wsConn *websocket.Conn, relay *Relay) *NodeConn {
	return &NodeConn{
		wsConn:          wsConn,
		relay:           relay,
		transport:       NewMessageTransport(wsConn),
		bridges:         make(map[string]*Bridge),
		bridgeChans:     make(map[string]chan []byte),
		shutdown:        make(chan struct{}),
		msgID:           1,
		pendingRequests: make(map[uint64]chan interface{}),
	}
}

// Run starts the message loop for handling the node connection
func (nc *NodeConn) Run() error {
	log.Printf("[NodeConn] Starting message loop...")

	// Start message pump
	go nc.messageLoop()

	// Monitor for shutdown
	go nc.monitorConnection()

	return nil
}

// monitorConnection monitors the connection and handles shutdown
func (nc *NodeConn) monitorConnection() {
	select {
	case <-nc.shutdown:
		log.Printf("[NodeConn %s] Shutdown requested", nc.nodeID)
		nc.Close()
	}
}

// messageLoop reads messages from the node and dispatches them
func (nc *NodeConn) messageLoop() {
	for {
		select {
		case <-nc.shutdown:
			return
		default:
		}

		msg, err := nc.transport.ReadMessage()
		if err != nil {
			log.Printf("[NodeConn] Read error: %v", err)
			nc.Close()
			return
		}

		// Log message type for debugging
		if m, ok := msg.(*shared.RegisterRequest); ok {
			log.Printf("[NodeConn] Received register_request, node=%s, msgID=%d", m.NodeID, m.ID)
		}

		if err := nc.handleMessage(msg); err != nil {
			log.Printf("[NodeConn] Error handling message: %v", err)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler
func (nc *NodeConn) handleMessage(msg interface{}) error {
	// Extract message info using the Messager interface
	m, err := shared.GetMessager(msg)
	if err != nil {
		return fmt.Errorf("invalid message type: %w", err)
	}

	msgType := m.GetType()
	msgID := m.GetID()

	// Check if this is a response to a pending request
	nc.pendingMu.Lock()
	if ch, exists := nc.pendingRequests[msgID]; exists {
		delete(nc.pendingRequests, msgID)
		nc.pendingMu.Unlock()
		select {
		case ch <- msg:
		default:
		}
		return nil
	}
	nc.pendingMu.Unlock()

	// Handle other message types
	switch msgType {
	case shared.MessageTypeRegisterRequest:
		return nc.handleRegisterRequest(msg.(*shared.RegisterRequest))

	case shared.MessageTypeNodeReady:
		return nc.handleNodeReady(msg.(*shared.NodeReadyMessage))

	case shared.MessageTypeRequestAuthRequest:
		return nc.handleRequestAuth(msg.(*shared.RequestAuthRequest))

	case shared.MessageTypeOpenBridgeResponse:
		return nc.handleOpenBridgeResponse(msg.(*shared.OpenBridgeResponse))

	case shared.MessageTypeCloseBridgeResponse:
		return nc.handleCloseBridgeResponse(msg.(*shared.CloseBridgeResponse))

	case shared.MessageTypeBridgeData:
		return nc.handleBridgeData(msg.(*shared.BridgeDataMessage))

	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

// handleRegisterRequest handles a registration request from the node
func (nc *NodeConn) handleRegisterRequest(req *shared.RegisterRequest) error {
	log.Printf("[NodeConn] Node %s registering with token (hostname=%q, macs=%v)", req.NodeID, req.Hostname, req.MACAddresses)

	// Validate token
	dbNode, err := nc.relay.NodeTable.GetNodeByID(req.NodeID)
	if err != nil {
		log.Printf("[NodeConn] Node %s not found in database: %v", req.NodeID, err)

		resp := &shared.RegisterResponse{
			Message: shared.Message{
				Type:  shared.MessageTypeRegisterResponse,
				ID:    req.ID,
				Error: "node not found",
			},
			Success: false,
		}
		_ = nc.transport.WriteMessage(resp)
		return nil
	}

	if dbNode.Token != req.Token {
		log.Printf("[NodeConn] Node %s provided invalid token", req.NodeID)

		resp := &shared.RegisterResponse{
			Message: shared.Message{
				Type:  shared.MessageTypeRegisterResponse,
				ID:    req.ID,
				Error: "invalid token",
			},
			Success: false,
		}
		_ = nc.transport.WriteMessage(resp)
		return nil
	}

	// Store node info
	nc.nodeID = req.NodeID
	nc.authToken = req.Token

	// Register this connection in the relay's nodes map
	nc.relay.RegisterNodeConnection(req.NodeID, nc)

	// Update last connected timestamp and store system info (hostname, MACs)
	if err := nc.relay.NodeTable.UpdateLastConnected(req.NodeID, req.Hostname, req.MACAddresses); err != nil {
		log.Printf("[NodeConn] Failed to update last connected: %v", err)
	}

	log.Printf("[NodeConn] Node %s successfully registered (hostname=%q, macs=%v)", req.NodeID, req.Hostname, req.MACAddresses)

	// Send success response
	resp := &shared.RegisterResponse{
		Message: shared.Message{
			Type: shared.MessageTypeRegisterResponse,
			ID:   req.ID,
		},
		Success: true,
	}
	log.Printf("[NodeConn] Sending register response for message ID %d", req.ID)
	if err := nc.transport.WriteMessage(resp); err != nil {
		log.Printf("[NodeConn] Failed to send register response: %v", err)
		return err
	}
	log.Printf("[NodeConn] Sent register response successfully")
	return nil
}

// handleNodeReady handles a ready signal from the node after registration
func (nc *NodeConn) handleNodeReady(msg *shared.NodeReadyMessage) error {
	if nc.nodeID == "" {
		log.Printf("[NodeConn] Received node_ready but no nodeID set")
		return nil
	}

	log.Printf("[NodeConn] Node %s is ready for bridges", nc.nodeID)

	// Mark node as ready
	nc.setReady()

	// Notify AutoStreamManager that node is ready to start streams
	if nc.relay.AutoStreamMgr != nil {
		go nc.relay.AutoStreamMgr.OnNodeReady(nc.nodeID)
	}

	return nil
}

// handleRequestAuth handles an authorization request from the node
func (nc *NodeConn) handleRequestAuth(req *shared.RequestAuthRequest) error {
	log.Printf("[NodeConn] Node %s requesting authorization", req.NodeID)

	// Check if node is already authorized
	existingNode, err := nc.relay.NodeTable.GetNodeByID(req.NodeID)
	if err == nil && existingNode.OwnerID != nil {
		// Node is already authorized - send the token via ReceiveTokenMessage push
		log.Printf("[NodeConn] Node %s already authorized (owner_id=%s), sending token",
			req.NodeID, *existingNode.OwnerID)

		// Send auth response (without token, will be sent separately)
		resp := &shared.RequestAuthResponse{
			Message: shared.Message{
				Type: shared.MessageTypeRequestAuthResponse,
				ID:   req.ID,
			},
			Success: true,
			AuthURL: "",
		}
		if err := nc.transport.WriteMessage(resp); err != nil {
			return err
		}

		// Send token via push message
		tokenMsg := &shared.ReceiveTokenMessage{
			Message: shared.Message{
				Type: shared.MessageTypeReceiveToken,
				ID:   nc.getMsgID(),
			},
			Token: existingNode.Token,
		}
		return nc.transport.WriteMessage(tokenMsg)
	}

	// Store the node ID temporarily
	nc.nodeID = req.NodeID

	// Register this connection in the relay's nodes map
	nc.relay.RegisterNodeConnection(req.NodeID, nc)

	// Generate authorization URL
	authURL := fmt.Sprintf("%s/authorize?node=%s", nc.relay.Config.DashboardURL, req.NodeID)

	log.Printf("[NodeConn] Generated auth URL for node %s: %s", req.NodeID, authURL)

	// Send response
	resp := &shared.RequestAuthResponse{
		Message: shared.Message{
			Type: shared.MessageTypeRequestAuthResponse,
			ID:   req.ID,
		},
		Success: true,
		AuthURL: authURL,
	}
	return nc.transport.WriteMessage(resp)
}

// handleOpenBridgeResponse handles the response to an open bridge request
func (nc *NodeConn) handleOpenBridgeResponse(resp *shared.OpenBridgeResponse) error {
	// This is handled via the pending requests mechanism
	return nil
}

// handleCloseBridgeResponse handles the response to a close bridge request
func (nc *NodeConn) handleCloseBridgeResponse(resp *shared.CloseBridgeResponse) error {
	// This is handled via the pending requests mechanism
	return nil
}

// handleBridgeData handles data messages from the node
func (nc *NodeConn) handleBridgeData(msg *shared.BridgeDataMessage) error {
	nc.bridgeMu.RLock()
	bridge, exists := nc.bridges[msg.BridgeID]
	nc.bridgeMu.RUnlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", msg.BridgeID)
	}

	// Forward data to bridge channel
	select {
	case bridge.DataChan <- msg.Data:
		atomic.AddInt64(&bridge.ByteCount, int64(len(msg.Data)))
		atomic.AddInt64(&bridge.MsgCount, 1)
	case <-time.After(5 * time.Second):
		log.Printf("[NodeConn] Bridge %s: data channel full, dropping packet", msg.BridgeID)
	}

	return nil
}

// =============================================================================
// Bridge Operations
// =============================================================================

// OpenBridge requests the node to open a bridge to a service
func (nc *NodeConn) OpenBridge(ctx context.Context, service *Service) (string, chan []byte, error) {
	bridgeID := uuid.New().String()

	if nc.nodeID == "" {
		return "", nil, fmt.Errorf("node not registered")
	}

	// Create data channel for receiving data from node
	dataChan := make(chan []byte, dataChanBufferSize)

	// Register data channel before opening bridge
	nc.bridgeMu.Lock()
	nc.bridgeChans[bridgeID] = dataChan
	nc.bridgeMu.Unlock()

	// Create open bridge request
	req := &shared.OpenBridgeRequest{
		Message: shared.Message{
			Type: shared.MessageTypeOpenBridgeRequest,
			ID:   nc.getMsgID(),
		},
		BridgeID:  bridgeID,
		ServiceID: service.ID,
		Service: shared.Service{
			ServiceURL: service.ServiceURL,
		},
	}

	// Send request
	if err := nc.transport.WriteMessage(req); err != nil {
		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)
		return "", nil, fmt.Errorf("send open bridge request: %w", err)
	}

	// Wait for response
	resp, err := nc.waitResponse(req.ID)
	if err != nil {
		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)
		return "", nil, fmt.Errorf("wait for open bridge response: %w", err)
	}

	openResp, ok := resp.(*shared.OpenBridgeResponse)
	if !ok || !openResp.Success {
		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)
		if openResp != nil {
			return "", nil, fmt.Errorf("open bridge failed")
		}
		return "", nil, fmt.Errorf("invalid response type")
	}

	// Store bridge
	nc.bridgeMu.Lock()
	nc.bridges[bridgeID] = &Bridge{
		ID:        bridgeID,
		ServiceID: service.ID,
		Service:   service,
		DataChan:  dataChan,
	}
	nc.bridgeMu.Unlock()

	log.Printf("[NodeConn %s] Opened bridge %s to %s",
		nc.nodeID, bridgeID, service.ServiceURL)

	return bridgeID, dataChan, nil
}

// OpenBridgeLegacy requests the node to open a bridge (legacy, without returning data channel)
func (nc *NodeConn) OpenBridgeLegacy(ctx context.Context, service *Service) (string, error) {
	bridgeID, _, err := nc.OpenBridge(ctx, service)
	return bridgeID, err
}

// CloseBridge requests the node to close a bridge
func (nc *NodeConn) CloseBridge(ctx context.Context, bridgeID string) error {
	if nc.nodeID == "" {
		return fmt.Errorf("node not registered")
	}

	// Get and remove bridge
	nc.bridgeMu.Lock()
	_, exists := nc.bridges[bridgeID]
	if exists {
		delete(nc.bridges, bridgeID)
	}
	if ch, chanExists := nc.bridgeChans[bridgeID]; chanExists {
		close(ch)
		delete(nc.bridgeChans, bridgeID)
	}
	nc.bridgeMu.Unlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", bridgeID)
	}

	// Create close bridge request
	req := &shared.CloseBridgeRequest{
		Message: shared.Message{
			Type: shared.MessageTypeCloseBridgeRequest,
			ID:   nc.getMsgID(),
		},
		BridgeID: bridgeID,
	}

	// Send request
	if err := nc.transport.WriteMessage(req); err != nil {
		return fmt.Errorf("send close bridge request: %w", err)
	}

	// Wait for response
	resp, err := nc.waitResponse(req.ID)
	if err != nil {
		return fmt.Errorf("wait for close bridge response: %w", err)
	}

	closeResp, ok := resp.(*shared.CloseBridgeResponse)
	if !ok || !closeResp.Success {
		return fmt.Errorf("close bridge failed")
	}

	log.Printf("[NodeConn %s] Closed bridge %s", nc.nodeID, bridgeID)
	return nil
}

// SendData sends data to a bridge via bridge_data message
func (nc *NodeConn) SendData(bridgeID string, payload []byte) error {
	nc.bridgeMu.RLock()
	_, exists := nc.bridges[bridgeID]
	nc.bridgeMu.RUnlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", bridgeID)
	}

	msg := &shared.BridgeDataMessage{
		Message: shared.Message{
			Type: shared.MessageTypeBridgeData,
			ID:   nc.getMsgID(),
		},
		BridgeID: bridgeID,
		Sequence: 0,
		Data:     payload,
	}

	if err := nc.transport.WriteMessage(msg); err != nil {
		return fmt.Errorf("send bridge data: %w", err)
	}

	return nil
}

// GetBridgeDataChan returns the data channel for a bridge
func (nc *NodeConn) GetBridgeDataChan(bridgeID string) (chan []byte, bool) {
	nc.bridgeMu.RLock()
	defer nc.bridgeMu.RUnlock()
	ch, exists := nc.bridgeChans[bridgeID]
	return ch, exists
}

// Close closes the node connection and all bridges
func (nc *NodeConn) Close() {
	nc.closeMu.Lock()
	defer nc.closeMu.Unlock()

	if nc.closed {
		return
	}
	nc.closed = true

	close(nc.shutdown)

	// Close all bridge channels
	nc.bridgeMu.Lock()
	for bridgeID, bridge := range nc.bridges {
		if bridge.DataChan != nil {
			close(bridge.DataChan)
		}
		log.Printf("[NodeConn %s] Closed bridge %s", nc.nodeID, bridgeID)
	}
	for bridgeID, ch := range nc.bridgeChans {
		close(ch)
		log.Printf("[NodeConn %s] Closed bridge channel %s", nc.nodeID, bridgeID)
	}
	nc.bridgeChans = make(map[string]chan []byte)
	nc.bridges = make(map[string]*Bridge)
	nc.bridgeMu.Unlock()

	// Notify AutoStreamManager about node disconnection
	if nc.relay != nil && nc.relay.AutoStreamMgr != nil {
		go nc.relay.AutoStreamMgr.OnNodeDisconnected(nc.nodeID)
	}

	// Close transport
	if nc.transport != nil {
		nc.transport.Close()
	}

	// Close WebSocket
	if nc.wsConn != nil {
		nc.wsConn.Close()
	}
}

// NodeID returns the node's ID
func (nc *NodeConn) NodeID() string {
	return nc.nodeID
}

// getMsgID returns the next message ID
func (nc *NodeConn) getMsgID() uint64 {
	nc.msgMu.Lock()
	defer nc.msgMu.Unlock()

	id := nc.msgID
	nc.msgID++
	return id
}

// setReady marks the node as ready (called when node_ready message is received)
func (nc *NodeConn) setReady() {
	atomic.StoreInt32(&nc.ready, 1)
}

// isReady returns whether the node is ready (has sent node_ready message)
func (nc *NodeConn) isReady() bool {
	return atomic.LoadInt32(&nc.ready) == 1
}

// waitResponse waits for a response with the given message ID
func (nc *NodeConn) waitResponse(msgID uint64) (interface{}, error) {
	respChan := make(chan interface{}, 1)

	nc.pendingMu.Lock()
	nc.pendingRequests[msgID] = respChan
	nc.pendingMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		nc.pendingMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response")
	case <-nc.shutdown:
		return nil, fmt.Errorf("connection closed")
	}
}

// =============================================================================
// Relay type (partial, needed for compilation)
// =============================================================================

// isDevMode returns true if dev mode is enabled
func (c *Config) isDevMode() bool {
	return c.DevImpersonate != ""
}
