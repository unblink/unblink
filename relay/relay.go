package relay

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/unblink/unblink/chat"
)

// Relay represents the relay server
type Relay struct {
	Config              *Config
	NodeTable           *table_node
	ServiceTable        *table_service
	UserTable           *table_user
	AgentTable          *table_agent
	AgentEventTable     *table_agent_event // NEW for historical events
	StorageTable        *table_storage     // Unified storage for frames and videos
	JWTManager          *JWTManager
	StorageManager      *StorageManager    // NEW for frame storage backend
	WebRTCSessionMgr    *WebRTCSessionManager
	AutoStreamMgr       *AutoStreamManager
	ClientRealtimeMgr   *ClientRealtimeStreamManager // NEW - general client realtime streams
	ChatService         *chat.Service
	nodes               map[string]*NodeConn
	nodesMu             sync.Mutex
}

// NewRelay creates a new Relay instance
func NewRelay(cfg *Config, nodeTable *table_node, serviceTable *table_service, userTable *table_user, agentTable *table_agent, agentEventTable *table_agent_event, storageTable *table_storage, jwtManager *JWTManager) *Relay {
	r := &Relay{
		Config:          cfg,
		NodeTable:       nodeTable,
		ServiceTable:    serviceTable,
		UserTable:       userTable,
		AgentTable:      agentTable,
		AgentEventTable: agentEventTable,
		StorageTable:    storageTable,
		JWTManager:      jwtManager,
		nodes:           make(map[string]*NodeConn),
	}
	// Initialize client realtime manager
	r.ClientRealtimeMgr = NewClientRealtimeStreamManager(r)
	return r
}

// SetChatService sets the chat service
func (r *Relay) SetChatService(s *chat.Service) {
	r.ChatService = s
}

// SetStorageManager sets the storage manager
func (r *Relay) SetStorageManager(sm *StorageManager) {
	r.StorageManager = sm
}

// GetWebSocketHandler returns the HTTP handler for WebSocket connections
func (r *Relay) GetWebSocketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wsConn, err := UpgradeHTTPToWebSocket(w, req)
		if err != nil {
			log.Printf("[WebSocket] Upgrade failed: %v", err)
			return
		}

		nodeConn := NewNodeConn(wsConn, r)
		go func() {
			if err := nodeConn.Run(); err != nil {
				log.Printf("[WebSocket] Node connection error: %v", err)
			}
		}()
	})
}

// RegisterNodeConnection registers a node connection
func (r *Relay) RegisterNodeConnection(nodeID string, conn *NodeConn) {
	r.nodesMu.Lock()
	r.nodes[nodeID] = conn
	r.nodesMu.Unlock()
}

// UnregisterNodeConnection unregisters a node connection
func (r *Relay) UnregisterNodeConnection(nodeID string) {
	r.nodesMu.Lock()
	defer r.nodesMu.Unlock()
	delete(r.nodes, nodeID)
}

// GetNodeConnection gets a node connection by ID
func (r *Relay) GetNodeConnection(nodeID string) (*NodeConn, bool) {
	r.nodesMu.Lock()
	defer r.nodesMu.Unlock()
	conn, ok := r.nodes[nodeID]
	return conn, ok
}

// UpgradeHTTPToWebSocket upgrades an HTTP connection to WebSocket
func UpgradeHTTPToWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}

	return conn, nil
}
