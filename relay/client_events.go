package relay

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents a connected browser client
type Client struct {
	ID        string
	UserID    int64
	Conn      *websocket.Conn
	sendChan  chan []byte
	closeChan chan struct{}
	closeOnce sync.Once
}

// Close safely closes the client's close channel
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeChan)
	})
}

// ClientEventRegistry manages browser WebSocket connections
type ClientEventRegistry struct {
	clients    map[string]*Client // clientID → client
	mu         sync.RWMutex
	agentTable *AgentTable
	cfg        *Config
	upgrader   websocket.Upgrader
}

// WSClientMessage represents WebSocket messages from client
type WSClientMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// RegisterClientMessage is sent by client to authenticate
type RegisterClientMessage struct {
	Token string `json:"token"`
}

// RegisteredClientMessage confirms successful registration
type RegisteredClientMessage struct {
	UserID int64 `json:"user_id"`
}

// ErrorClientMessage represents an error response
type ErrorClientMessage struct {
	Message string `json:"message"`
}

// NewClientEventRegistry creates a new client event registry
func NewClientEventRegistry(agentTable *AgentTable, cfg *Config) *ClientEventRegistry {
	return &ClientEventRegistry{
		clients:    make(map[string]*Client),
		agentTable: agentTable,
		cfg:        cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: Add origin validation for production
			},
		},
	}
}

// HandleWebSocket handles browser WebSocket connections at /client/connect
func (r *ClientEventRegistry) HandleWebSocket(w http.ResponseWriter, req *http.Request) {
	// Upgrade to WebSocket
	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("[ClientEvents] Failed to upgrade: %v", err)
		return
	}

	log.Printf("[ClientEvents] Client connected from %s, awaiting registration...", req.RemoteAddr)

	// Handle registration in a goroutine with timeout
	registered := make(chan *Client, 1)
	go r.handleRegistration(conn, req.RemoteAddr, registered)

	// Wait for registration (with 10 second timeout)
	select {
	case client := <-registered:
		if client != nil {
			// Registration successful, start send/receive loops
			go r.sendLoop(client)
			go r.receiveLoop(client)
		}
		// If client is nil, registration failed and connection was already closed
	case <-time.After(10 * time.Second):
		log.Printf("[ClientEvents] Registration timeout from %s", req.RemoteAddr)
		conn.Close()
	}
}

func (r *ClientEventRegistry) handleRegistration(conn *websocket.Conn, remoteAddr string, registered chan<- *Client) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read registration message
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[ClientEvents] Failed to read registration from %s: %v", remoteAddr, err)
		r.sendError(conn, "Failed to read registration message")
		conn.Close()
		registered <- nil
		return
	}

	var msg WSClientMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("[ClientEvents] Failed to parse registration from %s: %v", remoteAddr, err)
		r.sendError(conn, "Invalid message format")
		conn.Close()
		registered <- nil
		return
	}

	if msg.Type != "register" {
		log.Printf("[ClientEvents] Expected 'register' message, got '%s' from %s", msg.Type, remoteAddr)
		r.sendError(conn, "Expected registration message")
		conn.Close()
		registered <- nil
		return
	}

	var registerMsg RegisterClientMessage
	if err := json.Unmarshal(msg.Data, &registerMsg); err != nil {
		log.Printf("[ClientEvents] Failed to parse register data from %s: %v", remoteAddr, err)
		r.sendError(conn, "Invalid registration data")
		conn.Close()
		registered <- nil
		return
	}

	if registerMsg.Token == "" {
		log.Printf("[ClientEvents] Empty token from %s", remoteAddr)
		r.sendError(conn, "Missing token")
		conn.Close()
		registered <- nil
		return
	}

	// Validate JWT and get userID
	claims, err := ValidateToken(registerMsg.Token, r.cfg.JWTSecret)
	if err != nil {
		log.Printf("[ClientEvents] Invalid token from %s: %v", remoteAddr, err)
		r.sendError(conn, "Invalid token")
		conn.Close()
		registered <- nil
		return
	}

	// Create client
	client := &Client{
		ID:        uuid.New().String(),
		UserID:    claims.UserID,
		Conn:      conn,
		sendChan:  make(chan []byte, 100),
		closeChan: make(chan struct{}),
	}

	// Register client
	r.mu.Lock()
	r.clients[client.ID] = client
	r.mu.Unlock()

	log.Printf("[ClientEvents] Client registered: %s (user %d) from %s", client.ID, client.UserID, remoteAddr)

	// Send registered confirmation
	registeredMsg := map[string]interface{}{
		"type": "registered",
		"data": RegisteredClientMessage{
			UserID: claims.UserID,
		},
	}
	if msgBytes, err := json.Marshal(registeredMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, msgBytes)
	}

	// Clear read deadline for normal operation
	conn.SetReadDeadline(time.Time{})

	// Signal successful registration
	registered <- client
}

func (r *ClientEventRegistry) sendError(conn *websocket.Conn, message string) {
	errorMsg := map[string]interface{}{
		"type": "error",
		"data": ErrorClientMessage{
			Message: message,
		},
	}
	if msgBytes, err := json.Marshal(errorMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, msgBytes)
	}
}

func (r *ClientEventRegistry) sendLoop(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-client.sendChan:
			if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[ClientEvents] Write error for client %s: %v", client.ID, err)
				r.closeClient(client)
				return
			}
		case <-ticker.C:
			// Ping to keep connection alive
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				r.closeClient(client)
				return
			}
		case <-client.closeChan:
			return
		}
	}
}

func (r *ClientEventRegistry) receiveLoop(client *Client) {
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			r.closeClient(client)
			return
		}
		// Ignore client messages for now (just pings/pongs)
	}
}

func (r *ClientEventRegistry) closeClient(client *Client) {
	client.Close()
	client.Conn.Close()

	r.mu.Lock()
	delete(r.clients, client.ID)
	r.mu.Unlock()

	log.Printf("[ClientEvents] Client disconnected: %s", client.ID)
}

// BroadcastAgentEvent sends event to browsers whose users own the agent
func (r *ClientEventRegistry) BroadcastAgentEvent(agentID string, data map[string]interface{}, metadata map[string]interface{}, createdAt time.Time) {
	// Get agent info
	agent, err := r.agentTable.GetAgentByID(agentID)
	if err != nil {
		log.Printf("[ClientEvents] Failed to get agent %s: %v", agentID, err)
		return
	}

	// Build message
	message := map[string]interface{}{
		"type":       "agent_event",
		"id":         uuid.New().String(),
		"created_at": createdAt.Format(time.RFC3339),
		"data": map[string]interface{}{
			"id":          uuid.New().String(),
			"agent_id":    agentID,
			"agent_name":  agent.Name,
			"service_ids": agent.ServiceIDs,
			"data":        data,
			"metadata":    metadata,
			"created_at":  createdAt.Format(time.RFC3339),
		},
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("[ClientEvents] Failed to marshal message: %v", err)
		return
	}

	// Send to clients who own this agent
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.clients {
		if client.UserID == agent.UserID {
			select {
			case client.sendChan <- msgBytes:
				// Message sent successfully
			default:
				log.Printf("[ClientEvents] Client %s buffer full, dropping message", client.ID)
			}
		}
	}
}
