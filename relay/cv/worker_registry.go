package cv

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// CVWorker represents a connected worker
type CVWorker struct {
	ID           string
	Conn         *websocket.Conn
	RegisteredAt time.Time
	LastSeen     time.Time
	sendChan     chan []byte
	closeChan    chan struct{}
	closeOnce    sync.Once
}

// Close safely closes the worker's close channel
func (w *CVWorker) Close() {
	w.closeOnce.Do(func() {
		close(w.closeChan)
	})
}

// AgentRegistry interface for dependency injection
type AgentRegistry interface {
	GetAgentsForService(serviceID string) []*AgentInfo
}

// AgentInfo represents agent information from the registry
type AgentInfo struct {
	ID          string
	Name        string
	WorkerID    string
	Instruction string
	ServiceIDs  []string
}

// broadcastTarget represents a worker and associated data for broadcasting
type broadcastTarget struct {
	Worker    *CVWorker
	ServiceID string
	Agents    []*AgentEventInfo // Agent info for this worker
}

// CVWorkerRegistry manages worker connections and event distribution
type CVWorkerRegistry struct {
	workers        map[string]*CVWorker // workerID → worker
	workerKeys     map[string]string    // key → workerID (for authentication)
	mu             sync.RWMutex
	eventBus       *CVEventBus
	storageManager *StorageManager
	agentRegistry  AgentRegistry
	upgrader       websocket.Upgrader
	db             AgentEventStorage // Database for storing agent events
}

// AgentEventStorage interface for storing agent events
type AgentEventStorage interface {
	StoreAgentEvent(agentID string, data map[string]interface{}, metadata map[string]interface{}, createdAt time.Time) error
}

// WebSocket message types
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type RegisterMessage struct {
	WorkerID string `json:"worker_id"`
}

type RegisteredMessage struct {
	WorkerID string `json:"worker_id"`
}

type HeartbeatMessage struct {
	// No fields needed - relay tracks workers by connection
}

// NewCVWorkerRegistry creates a new worker registry
func NewCVWorkerRegistry(eventBus *CVEventBus, storageManager *StorageManager) *CVWorkerRegistry {
	registry := &CVWorkerRegistry{
		workers:        make(map[string]*CVWorker),
		workerKeys:     make(map[string]string),
		eventBus:       eventBus,
		storageManager: storageManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}

	// Register event listeners to broadcast to workers
	eventBus.OnFrameEvent(func(event *FrameEvent) {
		registry.BroadcastFrameEvent(event, time.Now())
	})
	eventBus.OnFrameBatchEvent(func(event *FrameBatchEvent) {
		registry.BroadcastFrameBatchEvent(event, time.Now())
	})

	return registry
}

// generateWorkerKey generates a unique cryptographic key for worker authentication
func generateWorkerKey() string {
	keyBytes := make([]byte, 32) // 256-bit key
	rand.Read(keyBytes)
	return hex.EncodeToString(keyBytes)
}

// GetWorkerIDByKey retrieves the worker ID associated with a key
func (r *CVWorkerRegistry) GetWorkerIDByKey(key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	workerID, exists := r.workerKeys[key]
	return workerID, exists
}

// registerWorkerKey registers a worker's authentication key
func (r *CVWorkerRegistry) registerWorkerKey(key string, workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workerKeys[key] = workerID
}

// SetStorageManager sets the storage manager (used to resolve circular dependency)
func (r *CVWorkerRegistry) SetStorageManager(sm *StorageManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.storageManager = sm
}

// SetAgentRegistry sets the agent registry (used to filter frame_batch events)
func (r *CVWorkerRegistry) SetAgentRegistry(ar AgentRegistry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agentRegistry = ar
}

// SetDatabase sets the database for storing agent events
func (r *CVWorkerRegistry) SetDatabase(db AgentEventStorage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = db
}

// groupAgentsByWorker groups agents by their worker_id
func (r *CVWorkerRegistry) groupAgentsByWorker(agents []*AgentInfo) map[string][]*AgentEventInfo {
	workerAgents := make(map[string][]*AgentEventInfo)
	for _, agent := range agents {
		if _, exists := workerAgents[agent.WorkerID]; !exists {
			workerAgents[agent.WorkerID] = []*AgentEventInfo{}
		}
		workerAgents[agent.WorkerID] = append(workerAgents[agent.WorkerID], &AgentEventInfo{
			ID:          agent.ID,
			Name:        agent.Name,
			Instruction: agent.Instruction,
		})
	}
	return workerAgents
}

// getTargetWorkers returns workers that should receive events based on agent configuration
// Returns nil if no agents configured (gating behavior)
func (r *CVWorkerRegistry) getTargetWorkers(serviceID string) []*broadcastTarget {
	// Check if agent registry is set
	if r.agentRegistry == nil {
		log.Printf("[CVWorkerRegistry] AgentRegistry not set, skipping broadcast")
		return nil
	}

	// Look up agents for this service
	log.Printf("[CVWorkerRegistry] Looking up agents for service %s", serviceID)
	agents := r.agentRegistry.GetAgentsForService(serviceID)
	if len(agents) == 0 {
		log.Printf("[CVWorkerRegistry] No agents configured for service %s, skipping broadcast", serviceID)
		return nil
	}
	log.Printf("[CVWorkerRegistry] Found %d agents for service %s", len(agents), serviceID)

	// Group agents by worker_id
	workerAgents := r.groupAgentsByWorker(agents)
	log.Printf("[CVWorkerRegistry] Agents grouped into %d workers", len(workerAgents))

	// Build targets for matching workers
	targets := make([]*broadcastTarget, 0, len(workerAgents))
	for workerID, agentList := range workerAgents {
		log.Printf("[CVWorkerRegistry] Checking if worker %s is connected (%d agents)", workerID, len(agentList))
		if worker, exists := r.workers[workerID]; exists {
			log.Printf("[CVWorkerRegistry] Worker %s is connected, adding to targets", workerID)
			targets = append(targets, &broadcastTarget{
				Worker:    worker,
				ServiceID: serviceID,
				Agents:    agentList,
			})
		} else {
			log.Printf("[CVWorkerRegistry] Worker %s NOT connected (total workers: %d)", workerID, len(r.workers))
		}
	}

	log.Printf("[CVWorkerRegistry] Returning %d broadcast targets for service %s", len(targets), serviceID)
	return targets
} // sendToWorkers sends a message to all target workers
func (r *CVWorkerRegistry) sendToWorkers(msg map[string]interface{}, targets []*broadcastTarget, eventType string) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CVWorkerRegistry] Failed to marshal %s event: %v", eventType, err)
		return
	}

	for _, target := range targets {
		select {
		case target.Worker.sendChan <- data:
			if len(target.Agents) > 0 {
				log.Printf("[CVWorkerRegistry] Sent %s to worker %s with %d agents", eventType, target.Worker.ID, len(target.Agents))
			}
		default:
			log.Printf("[CVWorkerRegistry] Worker %s send channel full, skipping %s event", target.Worker.ID, eventType)
		}
	}
}

// HandleWebSocket handles WebSocket connection requests
func (r *CVWorkerRegistry) HandleWebSocket(w http.ResponseWriter, req *http.Request) {
	// Upgrade connection
	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("[CVWorkerRegistry] Failed to upgrade connection: %v", err)
		return
	}

	// Handle worker connection
	go r.handleWorkerConnection(conn)
}

// handleWorkerConnection handles a single worker WebSocket connection
func (r *CVWorkerRegistry) handleWorkerConnection(conn *websocket.Conn) {
	defer conn.Close()

	// Wait for registration message
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("[CVWorkerRegistry] Failed to read registration message: %v", err)
		return
	}

	if msg.Type != "register" {
		log.Printf("[CVWorkerRegistry] Expected register message, got: %s", msg.Type)
		return
	}

	// Parse register message to get worker_id (worker-side generated)
	var registerMsg RegisterMessage
	if err := json.Unmarshal(msg.Data, &registerMsg); err != nil {
		log.Printf("[CVWorkerRegistry] Failed to parse register message: %v", err)
		return
	}

	workerID := registerMsg.WorkerID
	if workerID == "" {
		log.Printf("[CVWorkerRegistry] worker_id is required in register message")
		return
	}

	// Create worker
	worker := &CVWorker{
		ID:           workerID,
		Conn:         conn,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		sendChan:     make(chan []byte, 100),
		closeChan:    make(chan struct{}),
	}

	// Generate unique key for this worker
	workerKey := generateWorkerKey()

	// Register worker and key
	r.RegisterWorker(worker)
	r.registerWorkerKey(workerKey, workerID)
	defer r.RemoveWorker(workerID)

	// Send registration confirmation with key only (worker_id already known to worker)
	regConfirm := map[string]interface{}{
		"type": "registered",
		"data": map[string]string{
			"key": workerKey,
		},
	}
	if data, err := json.Marshal(regConfirm); err == nil {
		worker.sendChan <- data
	}

	log.Printf("[CVWorkerRegistry] Worker %s connected", workerID)

	// Start send and receive goroutines
	go r.sendLoop(worker)
	r.receiveLoop(worker)
}

// sendLoop handles sending messages to worker
func (r *CVWorkerRegistry) sendLoop(worker *CVWorker) {
	defer worker.Conn.Close()

	for {
		select {
		case <-worker.closeChan:
			return
		case data := <-worker.sendChan:
			if err := worker.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[CVWorkerRegistry] Failed to send to worker %s: %v", worker.ID, err)
				return
			}
		}
	}
}

// receiveLoop handles receiving messages from worker
func (r *CVWorkerRegistry) receiveLoop(worker *CVWorker) {
	defer worker.Close()

	for {
		var msg WSMessage
		if err := worker.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[CVWorkerRegistry] Worker %s connection error: %v", worker.ID, err)
			}
			return
		}

		// Handle message
		switch msg.Type {
		case "heartbeat":
			r.mu.Lock()
			worker.LastSeen = time.Now()
			r.mu.Unlock()

		case "event":
			// Parse event data
			var eventData map[string]interface{}
			if err := json.Unmarshal(msg.Data, &eventData); err != nil {
				log.Printf("[CVWorkerRegistry] Failed to parse event from worker %s: %v", worker.ID, err)
				continue
			}

			// Extract agent_id, data, and metadata for database storage
			agentID, _ := eventData["agent_id"].(string)
			data, _ := eventData["data"].(map[string]interface{})
			metadata, _ := eventData["metadata"].(map[string]interface{})
			createdAt := time.Now()

			// Store agent event in database
			if r.db != nil && agentID != "" && data != nil {
				if err := r.db.StoreAgentEvent(agentID, data, metadata, createdAt); err != nil {
					log.Printf("[CVWorkerRegistry] Failed to store agent event: %v", err)
				} else {
					log.Printf("[CVWorkerRegistry] Stored agent event for agent %s", agentID)
				}
			}

			// Create worker event
			workerEvent := &WorkerEvent{
				EventID:   fmt.Sprintf("%s-%d", worker.ID, time.Now().UnixNano()),
				WorkerID:  worker.ID,
				CreatedAt: createdAt,
				Data:      eventData,
			}

			// Publish to event bus
			r.eventBus.PublishWorkerEvent(workerEvent)

		default:
			log.Printf("[CVWorkerRegistry] Unknown message type from worker %s: %s", worker.ID, msg.Type)
		}
	}
}

// RegisterWorker adds a worker to the registry
func (r *CVWorkerRegistry) RegisterWorker(worker *CVWorker) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old connection if exists
	if old, exists := r.workers[worker.ID]; exists {
		old.Close()
	}

	r.workers[worker.ID] = worker
	log.Printf("[CVWorkerRegistry] Registered worker %s (total=%d)", worker.ID, len(r.workers))
}

// RemoveWorker removes a worker from the registry
func (r *CVWorkerRegistry) RemoveWorker(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if worker, exists := r.workers[workerID]; exists {
		worker.Close()
		delete(r.workers, workerID)

		// Remove all keys associated with this worker
		for key, wID := range r.workerKeys {
			if wID == workerID {
				delete(r.workerKeys, key)
			}
		}

		log.Printf("[CVWorkerRegistry] Removed worker %s (total=%d)", workerID, len(r.workers))
	}
}

// BroadcastFrameEvent broadcasts a frame event to workers with matching agents
func (r *CVWorkerRegistry) BroadcastFrameEvent(event *FrameEvent, timestamp time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.workers) == 0 {
		return
	}

	// Get target workers (applies agent gating)
	targets := r.getTargetWorkers(event.ServiceID)
	if len(targets) == 0 {
		return
	}

	// Create message
	msg := map[string]interface{}{
		"type":       "frame",
		"id":         uuid.New().String(),
		"created_at": timestamp.UTC().Format(time.RFC3339),
		"data":       event,
	}

	// Send to target workers
	r.sendToWorkers(msg, targets, "frame")
}

// BroadcastFrameBatchEvent broadcasts a frame batch event to workers with matching agents
func (r *CVWorkerRegistry) BroadcastFrameBatchEvent(event *FrameBatchEvent, timestamp time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.workers) == 0 {
		return
	}

	// Get target workers (applies agent gating)
	targets := r.getTargetWorkers(event.ServiceID)
	if len(targets) == 0 {
		return
	}

	// Send to each target with their specific agents
	for _, target := range targets {
		// Create event copy with filtered agents for this worker
		eventCopy := *event
		eventCopy.Agents = target.Agents

		msg := map[string]interface{}{
			"type":       "frame_batch",
			"id":         uuid.New().String(),
			"created_at": timestamp.UTC().Format(time.RFC3339),
			"data":       &eventCopy,
		}

		// Send to this specific worker
		r.sendToWorkers(msg, []*broadcastTarget{target}, "frame_batch")
	}
}
