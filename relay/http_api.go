package relay

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func StartHTTPAPIServer(relay *Relay, addr string, cfg *Config) (*http.Server, error) {
	mux := http.NewServeMux()

	// Wrap with CORS middleware
	handler := corsMiddleware(mux)

	sessionManager := NewWebRTCSessionManager()
	authStore := NewAuthStore(relay.db.DB)

	mux.HandleFunc("/api/authorize", func(w http.ResponseWriter, r *http.Request) {
		handleAuthorizeAPI(w, r, cfg, relay)
	})

	// Feature flags endpoint (no auth required)
	mux.HandleFunc("/flags", func(w http.ResponseWriter, r *http.Request) {
		handleFlags(w, r, cfg)
	})

	// Browser client WebSocket connection (auth via registration message)
	mux.HandleFunc("/client/connect", relay.clientEvents.HandleWebSocket)

	// List all nodes (protected)
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		userID, err := requireAuth(w, r, cfg, authStore)
		if err != nil {
			return
		}
		handleListNodes(w, r, relay, userID)
	})

	// Node-specific endpoints: /nodes/{nodeId}/services and /nodes/{nodeId}/offer (protected)
	mux.HandleFunc("/nodes/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /nodes/{nodeId}/{endpoint}
		path := strings.TrimPrefix(r.URL.Path, "/nodes/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "Invalid path. Expected /nodes/{nodeId}/{services|offer}", http.StatusBadRequest)
			return
		}

		nodeID := parts[0]
		endpoint := parts[1]

		// Check authentication
		userID, err := requireAuth(w, r, cfg, authStore)
		if err != nil {
			return
		}

		// Verify node ownership
		if !relay.nodeTable.UserOwnsNode(userID, nodeID) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}

		switch endpoint {
		case "services":
			handleNodeServices(w, r, relay, nodeID)
		case "offer":
			handleNodeOffer(w, r, relay, nodeID, sessionManager)
		case "name":
			handleUpdateNodeName(w, r, relay, nodeID)
		case "delete":
			handleDeleteNode(w, r, relay, nodeID)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	// Authentication endpoints
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		handleRegister(w, r, authStore)
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		handleLogin(w, r, authStore, cfg)
	})

	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		handleLogout(w, r)
	})

	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		handleMe(w, r, authStore, cfg)
	})

	// Agent endpoints (protected)
	mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		userID, err := requireAuth(w, r, cfg, authStore)
		if err != nil {
			return
		}

		switch r.Method {
		case http.MethodPost:
			handleCreateAgent(w, r, relay, userID)
		case http.MethodGet:
			handleListAgents(w, r, relay, userID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Agent-specific endpoints: /agents/{agentId} (protected)
	mux.HandleFunc("/agents/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /agents/{agentId}/{endpoint}
		path := strings.TrimPrefix(r.URL.Path, "/agents/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 1 || parts[0] == "" {
			http.Error(w, "Invalid path. Expected /agents/{agentId}", http.StatusBadRequest)
			return
		}

		agentID := parts[0]
		endpoint := ""
		if len(parts) > 1 {
			endpoint = parts[1]
		}

		// Check authentication
		userID, err := requireAuth(w, r, cfg, authStore)
		if err != nil {
			return
		}

		// Verify agent ownership
		if !relay.agentTable.UserOwnsAgent(userID, agentID) {
			http.Error(w, "Agent not found", http.StatusNotFound)
			return
		}

		// Route to specific handler
		if endpoint == "" {
			// GET/PATCH/DELETE /agents/{agentId}
			switch r.Method {
			case http.MethodGet:
				handleGetAgent(w, r, relay, agentID)
			case http.MethodPatch:
				handlePatchAgent(w, r, relay, agentID)
			case http.MethodDelete:
				handleDeleteAgent(w, r, relay, agentID)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	log.Printf("[HTTP] Starting HTTP API on %s", addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	return server, nil
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Dev-Impersonate")
		w.Header().Set("Access-Control-Allow-Credentials", "true") // Required for credentials mode

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// handleListNodes returns a list of nodes owned by the authenticated user
func handleListNodes(w http.ResponseWriter, r *http.Request, relay *Relay, userID int64) {

	// Get nodes owned by this user
	nodes, err := relay.nodeTable.GetNodesByUser(userID)
	if err != nil {
		log.Printf("[HTTP] handleListNodes: error getting nodes: %v", err)
		http.Error(w, "Failed to fetch nodes", http.StatusInternalServerError)
		return
	}

	// Return nodes with status information
	type NodeResponse struct {
		ID              string  `json:"id"`
		Status          string  `json:"status"`
		Name            *string `json:"name,omitempty"`
		LastConnectedAt *string `json:"last_connected_at,omitempty"`
	}

	result := make([]NodeResponse, len(nodes))
	for i, node := range nodes {
		// Check if node is currently connected
		status := "offline"
		if relay.GetNode(node.ID) != nil {
			status = "online"
		}

		var lastConnected *string
		if node.LastConnectedAt != nil {
			formatted := node.LastConnectedAt.Format("2006-01-02T15:04:05Z07:00")
			lastConnected = &formatted
		}

		result[i] = NodeResponse{
			ID:              node.ID,
			Status:          status,
			Name:            node.Name,
			LastConnectedAt: lastConnected,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleNodeServices returns services filtered by node ID
func handleNodeServices(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := relay.Services().ListByNode(nodeID)

	// Check if node exists (either connected or has registered services)
	node := relay.GetNode(nodeID)
	nodeExists := node != nil || len(services) > 0

	if !nodeExists {

		http.Error(w, "Node not found: "+nodeID, http.StatusNotFound)
		return
	}

	type ServiceInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		NodeID string `json:"node_id"`
		Addr   string `json:"addr"`
		Port   int    `json:"port"`
		Path   string `json:"path"`
	}

	result := make([]ServiceInfo, len(services))
	for i, s := range services {
		result[i] = ServiceInfo{
			ID:     s.Service.ID,
			Name:   s.Service.Name,
			Type:   s.Service.Type,
			NodeID: s.NodeID,
			Addr:   s.Service.Addr,
			Port:   s.Service.Port,
			Path:   s.Service.Path,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleNodeOffer handles WebRTC offers for a specific node's services
func handleNodeOffer(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string, sessionManager *WebRTCSessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SDP       string `json:"sdp"`
		ServiceID string `json:"serviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[HTTP] WebRTC offer for service %s (node %s) from %s", req.ServiceID, nodeID, r.RemoteAddr)

	// Find service
	regService := relay.Services().Get(req.ServiceID)
	if regService == nil {
		http.Error(w, "Service not found: "+req.ServiceID, http.StatusNotFound)
		return
	}

	// Verify service belongs to this node
	if regService.NodeID != nodeID {
		http.Error(w, "Service does not belong to this node", http.StatusForbidden)
		return
	}

	service := regService.Service

	// Create WebRTC session
	sessionID, answerSDP, err := sessionManager.NewSession(req.SDP, service, relay)
	if err != nil {
		log.Printf("[HTTP] WebRTC session failed: %v", err)
		http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[HTTP] WebRTC session %s created for service %s", sessionID, req.ServiceID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"type": "answer",
		"sdp":  answerSDP,
	})
}

// handleUpdateNodeName handles updating a node's name
func handleUpdateNodeName(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate name
	if len(req.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	// Update node name
	if err := relay.nodeTable.UpdateNodeName(nodeID, req.Name); err != nil {
		log.Printf("[HTTP] Failed to update node name: %v", err)
		http.Error(w, "Failed to update node name", http.StatusInternalServerError)
		return
	}

	log.Printf("[HTTP] Updated name for node %s to '%s'", nodeID, req.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": "true",
		"name":    req.Name,
	})
}

// handleDeleteNode handles deleting/unauthorizing a node
func handleDeleteNode(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete node from database
	if err := relay.nodeTable.DeleteNode(nodeID); err != nil {
		log.Printf("[HTTP] Failed to delete node: %v", err)
		http.Error(w, "Failed to delete node", http.StatusInternalServerError)
		return
	}

	// If node is currently connected, disconnect it
	if nc := relay.GetNode(nodeID); nc != nil {
		nc.Close()
	}

	log.Printf("[HTTP] Deleted node %s", nodeID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": "true",
	})
}

// Agent handler functions

// AgentResponse represents the agent data returned in API responses
type AgentResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Instruction string   `json:"instruction"`
	WorkerID    string   `json:"worker_id"`
	ServiceIDs  []string `json:"service_ids"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// handleCreateAgent handles POST /agents
func handleCreateAgent(w http.ResponseWriter, r *http.Request, relay *Relay, userID int64) {
	var req struct {
		Name        string   `json:"name"`
		Instruction string   `json:"instruction"`
		WorkerID    string   `json:"worker_id"`
		ServiceIDs  []string `json:"service_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Agent name is required", http.StatusBadRequest)
		return
	}
	if req.Instruction == "" {
		http.Error(w, "Agent instruction is required", http.StatusBadRequest)
		return
	}
	if req.WorkerID == "" {
		http.Error(w, "Worker ID is required", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}
	if len(req.Instruction) > 5000 {
		http.Error(w, "Instruction too long (max 5000 characters)", http.StatusBadRequest)
		return
	}

	// Create agent
	agent, err := relay.agentTable.CreateAgent(req.Name, req.Instruction, req.WorkerID, req.ServiceIDs, userID)
	if err != nil {
		log.Printf("[HTTP] Failed to create agent: %v", err)
		http.Error(w, "Failed to create agent", http.StatusInternalServerError)
		return
	}

	// Register agent in memory registry
	relay.agentRegistry.RegisterAgent(&AgentInfo{
		ID:          agent.ID,
		Name:        agent.Name,
		WorkerID:    agent.WorkerID,
		Instruction: agent.Config.Instruction,
		ServiceIDs:  agent.ServiceIDs,
	})

	log.Printf("[HTTP] Agent %s created by user %d", agent.ID, userID)

	// Format response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      agent.ID,
		"agent": AgentResponse{
			ID:          agent.ID,
			Name:        agent.Name,
			Instruction: agent.Config.Instruction,
			WorkerID:    agent.WorkerID,
			ServiceIDs:  agent.ServiceIDs,
			CreatedAt:   agent.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

// handleListAgents handles GET /agents
func handleListAgents(w http.ResponseWriter, r *http.Request, relay *Relay, userID int64) {
	agents, err := relay.agentTable.GetAgentsByUser(userID)
	if err != nil {
		log.Printf("[HTTP] Failed to list agents: %v", err)
		http.Error(w, "Failed to fetch agents", http.StatusInternalServerError)
		return
	}

	result := make([]AgentResponse, len(agents))
	for i, agent := range agents {
		result[i] = AgentResponse{
			ID:          agent.ID,
			Name:        agent.Name,
			Instruction: agent.Config.Instruction,
			WorkerID:    agent.WorkerID,
			ServiceIDs:  agent.ServiceIDs,
			CreatedAt:   agent.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   agent.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleGetAgent handles GET /agents/{agentId}
func handleGetAgent(w http.ResponseWriter, r *http.Request, relay *Relay, agentID string) {
	agent, err := relay.agentTable.GetAgentByID(agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentResponse{
		ID:          agent.ID,
		Name:        agent.Name,
		Instruction: agent.Config.Instruction,
		WorkerID:    agent.WorkerID,
		ServiceIDs:  agent.ServiceIDs,
		CreatedAt:   agent.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   agent.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// handlePatchAgent handles PATCH /agents/{agentId}
func handlePatchAgent(w http.ResponseWriter, r *http.Request, relay *Relay, agentID string) {
	var req struct {
		Name        *string   `json:"name"`
		Instruction *string   `json:"instruction"`
		ServiceIDs  *[]string `json:"service_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate fields if provided
	if req.Name != nil && len(*req.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}
	if req.Instruction != nil && len(*req.Instruction) > 5000 {
		http.Error(w, "Instruction too long (max 5000 characters)", http.StatusBadRequest)
		return
	}

	// Update agent with partial fields
	if err := relay.agentTable.UpdateAgent(agentID, req.Name, req.Instruction, req.ServiceIDs); err != nil {
		log.Printf("[HTTP] Failed to update agent: %v", err)
		http.Error(w, "Failed to update agent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Reload agent from database and update registry
	agent, err := relay.agentTable.GetAgentByID(agentID)
	if err != nil {
		log.Printf("[HTTP] Warning: Failed to reload agent for registry update: %v", err)
	} else {
		relay.agentRegistry.RegisterAgent(&AgentInfo{
			ID:          agent.ID,
			Name:        agent.Name,
			WorkerID:    agent.WorkerID,
			Instruction: agent.Config.Instruction,
			ServiceIDs:  agent.ServiceIDs,
		})
	}

	log.Printf("[HTTP] Updated agent %s", agentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentResponse{
		ID:          agent.ID,
		Name:        agent.Name,
		Instruction: agent.Config.Instruction,
		WorkerID:    agent.WorkerID,
		ServiceIDs:  agent.ServiceIDs,
		CreatedAt:   agent.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   agent.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// handleDeleteAgent handles DELETE /agents/{agentId}
func handleDeleteAgent(w http.ResponseWriter, r *http.Request, relay *Relay, agentID string) {
	if err := relay.agentTable.DeleteAgent(agentID); err != nil {
		log.Printf("[HTTP] Failed to delete agent: %v", err)
		http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
		return
	}

	// Remove agent from memory registry
	relay.agentRegistry.RemoveAgent(agentID)

	log.Printf("[HTTP] Deleted agent %s", agentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
