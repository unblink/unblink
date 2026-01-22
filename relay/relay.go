package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/unblink/unblink/chat"
)

// Relay represents the relay server
type Relay struct {
	Config            *Config
	NodeTable         *table_node
	ServiceTable      *table_service
	UserTable         *table_user
	AgentTable        *table_agent
	AgentEventTable   *table_agent_event // NEW for historical events
	StorageTable      *table_storage     // Unified storage for frames and videos
	JWTManager        *JWTManager
	StorageManager    *StorageManager // NEW for frame storage backend
	WriteMgr          *WriteManager   // Serializes DB writes to prevent lock errors
	WebRTCSessionMgr  *WebRTCSessionManager
	AutoStreamMgr     *AutoStreamManager
	ClientRealtimeMgr *ClientRealtimeStreamManager // NEW - general client realtime streams
	ChatService       *chat.Service
	nodes             map[string]*NodeConn
	nodesMu           sync.Mutex
}

// NewRelay creates a new Relay instance
func NewRelay(cfg *Config, nodeTable *table_node, serviceTable *table_service, userTable *table_user, agentTable *table_agent, agentEventTable *table_agent_event, storageTable *table_storage, jwtManager *JWTManager, writeMgr *WriteManager) *Relay {
	r := &Relay{
		Config:          cfg,
		NodeTable:       nodeTable,
		ServiceTable:    serviceTable,
		UserTable:       userTable,
		AgentTable:      agentTable,
		AgentEventTable: agentEventTable,
		StorageTable:    storageTable,
		JWTManager:      jwtManager,
		WriteMgr:        writeMgr,
		nodes:           make(map[string]*NodeConn),
	}
	// Initialize client realtime manager
	r.ClientRealtimeMgr = NewClientRealtimeStreamManager(r)
	return r
}

// EnsureAdminUser creates the admin user if configured and doesn't exist
func (r *Relay) EnsureAdminUser() {
	if r.Config.AdminEmail == "" || r.Config.AdminPassword == "" {
		return
	}

	// Check if user already exists
	user, err := r.UserTable.GetUserByEmail(r.Config.AdminEmail)
	if err == nil {
		// User exists, check if password matches .env
		_, err = r.UserTable.ValidatePassword(r.Config.AdminEmail, r.Config.AdminPassword)
		if err != nil {
			// Password doesn't match, update it
			log.Printf("[INFO] Updating admin password for %s to match .env", r.Config.AdminEmail)
			if err := r.UserTable.UpdatePassword(user.ID, r.Config.AdminPassword); err != nil {
				log.Printf("[ERROR] Failed to update admin password: %v", err)
			}
		}
		r.ensureAdminNodes(user.ID)
		return
	}

	// Create user
	user, err = r.UserTable.CreateUser(r.Config.AdminEmail, r.Config.AdminPassword, "Admin")
	if err != nil {
		log.Printf("[ERROR] Failed to ensure admin user: %v", err)
		return
	}

	log.Printf("[INFO] Admin user created: %s (ID: %s)", user.Email, user.ID)
	r.ensureAdminNodes(user.ID)
}

func (r *Relay) ensureAdminNodes(userID string) {
	if r.Config.AdminNodes == "" {
		// Fallback to defaults if legacy env vars missing
		r.authorizeNode(userID, "dev-node-0000", "dev-token-0000", "admin-node")
		r.ensureAdminServices(userID, "dev-node-0000")
		r.ensureAdminAgents(userID)
		return
	}

	var nodes []struct {
		ID       string `json:"id"`
		Token    string `json:"token"`
		Hostname string `json:"hostname"`
	}

	if err := json.Unmarshal([]byte(r.Config.AdminNodes), &nodes); err != nil {
		log.Printf("[ERROR] Failed to parse ADMIN_NODES: %v", err)
		return
	}

	for _, n := range nodes {
		r.authorizeNode(userID, n.ID, n.Token, n.Hostname)
	}

	// We still need a "default" node ID for services that don't specify one
	defaultNodeID := "dev-node-0000"
	if len(nodes) > 0 {
		defaultNodeID = nodes[0].ID
	}

	r.ensureAdminServices(userID, defaultNodeID)
	r.ensureAdminAgents(userID)
}

func (r *Relay) authorizeNode(userID, nodeID, token, hostname string) {
	if nodeID == "" || token == "" {
		return
	}

	if hostname == "" {
		hostname = "admin-node"
	}

	err := r.NodeTable.AuthorizeNode(nodeID, userID, hostname, token)
	if err != nil {
		log.Printf("[ERROR] Failed to authorize node %s: %v", nodeID, err)
		return
	}

	log.Printf("[INFO] Node %s authorized for user %s", nodeID, userID)
}

func (r *Relay) ensureAdminServices(userID, defaultNodeID string) {
	if r.Config.AdminServices == "" {
		return
	}

	var services []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ServiceURL  string `json:"url"`
		Description string `json:"description"`
		NodeID      string `json:"node_id"`
	}

	if err := json.Unmarshal([]byte(r.Config.AdminServices), &services); err != nil {
		log.Printf("[ERROR] Failed to parse ADMIN_SERVICES: %v", err)
		return
	}

	for _, s := range services {
		if s.ID == "" {
			log.Printf("[ERROR] Admin service %q is missing a fixed ID in ADMIN_SERVICES", s.Name)
			continue
		}

		// Check if service exists ONLY by ID
		_, err := r.ServiceTable.GetService(s.ID)
		if err == nil {
			// Already exists, skip
			continue
		}

		nodeID := s.NodeID
		if nodeID == "" {
			nodeID = defaultNodeID
		}

		log.Printf("[INFO] Creating admin service for node %s: %s (ID: %s, URL: %s)", nodeID, s.Name, s.ID, s.ServiceURL)
		req := &CreateServiceRequest{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			NodeID:      nodeID,
			ServiceURL:  s.ServiceURL,
		}
		if _, err := r.ServiceTable.CreateService(req, userID); err != nil {
			log.Printf("[ERROR] Failed to create admin service %s: %v", s.Name, err)
		}
	}
}

func (r *Relay) ensureAdminAgents(userID string) {
	if r.Config.AdminAgents == "" {
		return
	}

	var agents []struct {
		ID         string      `json:"id"`
		Name       string      `json:"name"`
		Type       string      `json:"type"`
		Config     AgentConfig `json:"config"`
		ServiceIDs []string    `json:"service_ids"`
	}

	if err := json.Unmarshal([]byte(r.Config.AdminAgents), &agents); err != nil {
		log.Printf("[ERROR] Failed to parse ADMIN_AGENTS: %v", err)
		return
	}

	for _, s := range agents {
		if s.ID == "" {
			log.Printf("[ERROR] Admin agent %q is missing a fixed ID in ADMIN_AGENTS", s.Name)
			continue
		}

		// Check if agent exists ONLY by ID
		_, err := r.AgentTable.GetAgentByID(s.ID)
		if err == nil {
			// Already exists, skip
			continue
		}

		log.Printf("[INFO] Creating admin agent: %s (ID: %s)", s.Name, s.ID)
		req := CreateAgentRequest{
			ID:         s.ID,
			Name:       s.Name,
			Type:       s.Type,
			Config:     s.Config,
			ServiceIDs: s.ServiceIDs,
		}
		if _, err := r.AgentTable.CreateAgent(req, userID); err != nil {
			log.Printf("[ERROR] Failed to create admin agent %s: %v", s.Name, err)
		}
	}
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
