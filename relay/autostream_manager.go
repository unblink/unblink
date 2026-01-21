package relay

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AutoStreamManager coordinates automatic frame streaming to OpenAI base_urls
type AutoStreamManager struct {
	relay  *Relay
	config *Config

	// Service streams (one per service being monitored)
	serviceStreams map[string]*ServiceStream // key: serviceID
	streamsMu      sync.RWMutex

	// Agent registry (in-memory cache from DB)
	agentRegistry map[string]*Agent // key: agentID
	agentsMu      sync.RWMutex

	// Frame batching (per agent:service)
	frameBatches map[string][]*Frame // key: agentID + ":" + serviceID
	batchesMu    sync.Mutex

	// OpenAI clients (one per agent, since each agent can have different config)
	openaiClients map[string]*OpenAIClient // key: agentID
	clientsMu     sync.RWMutex

	// Storage manager for frame/video persistence
	storageManager *StorageManager
	StorageTable   *table_storage
}

// NewAutoStreamManager creates a new auto-stream manager
func NewAutoStreamManager(relay *Relay, config *Config, storageManager *StorageManager, storageTable *table_storage) *AutoStreamManager {
	manager := &AutoStreamManager{
		relay:          relay,
		config:         config,
		serviceStreams: make(map[string]*ServiceStream),
		agentRegistry:  make(map[string]*Agent),
		frameBatches:   make(map[string][]*Frame),
		openaiClients:  make(map[string]*OpenAIClient),
		storageManager: storageManager,
		StorageTable:   storageTable,
	}

	// Load agents from database
	if err := manager.LoadAgentsFromDB(); err != nil {
		log.Printf("[AutoStreamManager] Warning: failed to load agents: %v", err)
	}

	return manager
}

// LoadAgentsFromDB loads all agents from the database into the registry
func (m *AutoStreamManager) LoadAgentsFromDB() error {
	agents, err := m.relay.AgentTable.GetAllAgents()
	if err != nil {
		return fmt.Errorf("failed to get agents: %w", err)
	}

	m.agentsMu.Lock()
	defer m.agentsMu.Unlock()

	for _, agent := range agents {
		m.agentRegistry[agent.ID] = agent
	}

	log.Printf("[AutoStreamManager] Loaded %d agents from database", len(agents))

	return nil
}

// ensureStreamsForAgents ensures ServiceStreams exist for all services needed by agents
func (m *AutoStreamManager) ensureStreamsForAgents() {
	// Get unique set of service IDs needed by openai_compat agents
	neededServices := make(map[string]bool)

	m.agentsMu.RLock()
	for _, agent := range m.agentRegistry {
		effectiveConfig := m.getEffectiveConfig(agent)
		if agent.Type == "openai_compat" && effectiveConfig.BaseUrl != "" {
			for _, serviceID := range agent.ServiceIDs {
				neededServices[serviceID] = true
			}
		}
	}
	m.agentsMu.RUnlock()

	// Create streams for needed services
	for serviceID := range neededServices {
		if err := m.ensureServiceStream(serviceID); err != nil {
			log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
		}
	}
}

// ensureServiceStream ensures a ServiceStream exists for the given service
func (m *AutoStreamManager) ensureServiceStream(serviceID string) error {
	log.Printf("[AutoStreamManager] ensureServiceStream: serviceID=%s", serviceID)

	m.streamsMu.Lock()
	defer m.streamsMu.Unlock()

	// Check if stream already exists
	if _, exists := m.serviceStreams[serviceID]; exists {
		log.Printf("[AutoStreamManager] Stream already exists for service %s", serviceID)
		return nil
	}

	// Get service from database
	service, err := m.relay.ServiceTable.GetService(serviceID)
	if err != nil {
		log.Printf("[AutoStreamManager] Failed to get service %s: %v", serviceID, err)
		return fmt.Errorf("failed to get service: %w", err)
	}
	log.Printf("[AutoStreamManager] Got service %s (name=%s, nodeID=%s, url=%s)", service.ID, service.Name, service.NodeID, service.ServiceURL)
	log.Printf("[AutoStreamManager] Getting node connection for %s...", service.NodeID)

	// Get node connection
	nodeConn, ok := m.relay.GetNodeConnection(service.NodeID)
	log.Printf("[AutoStreamManager] GetNodeConnection returned for %s: ok=%v", service.NodeID, ok)
	if !ok {
		log.Printf("[AutoStreamManager] Node %s not connected", service.NodeID)
		return fmt.Errorf("node not connected: %s", service.NodeID)
	}
	log.Printf("[AutoStreamManager] Node %s is connected", service.NodeID)

	// Check if node is ready (has sent node_ready message)
	if !nodeConn.isReady() {
		log.Printf("[AutoStreamManager] Node %s not ready yet, skipping stream creation", service.NodeID)
		return nil // silently skip, not an error
	}

	// Create service stream
	log.Printf("[AutoStreamManager] Creating ServiceStream for %s...", serviceID)
	stream, err := NewServiceStream(serviceID, service, nodeConn, m, m.config)
	if err != nil {
		log.Printf("[AutoStreamManager] Failed to create ServiceStream: %v", err)
		return fmt.Errorf("failed to create service stream: %w", err)
	}
	log.Printf("[AutoStreamManager] ServiceStream created for %s", serviceID)

	// Start the stream
	log.Printf("[AutoStreamManager] Starting ServiceStream for %s...", serviceID)
	if err := stream.Start(); err != nil {
		log.Printf("[AutoStreamManager] Failed to start ServiceStream: %v", err)
		return fmt.Errorf("failed to start service stream: %w", err)
	}
	log.Printf("[AutoStreamManager] ServiceStream started for %s", serviceID)

	m.serviceStreams[serviceID] = stream
	log.Printf("[AutoStreamManager] Created service stream for %s", serviceID)

	return nil
}

// removeServiceStream removes and closes a ServiceStream
func (m *AutoStreamManager) removeServiceStream(serviceID string) {
	m.streamsMu.Lock()
	stream, exists := m.serviceStreams[serviceID]
	if exists {
		delete(m.serviceStreams, serviceID)
	}
	m.streamsMu.Unlock()

	if stream != nil {
		stream.Close()
		log.Printf("[AutoStreamManager] Removed service stream for %s", serviceID)
	}
}

// onFrame is called when a frame is extracted from a service
// This is the fan-out logic that distributes frames to agents
func (m *AutoStreamManager) onFrame(serviceID string, frame *Frame) {
	m.agentsMu.RLock()
	defer m.agentsMu.RUnlock()

	// log.Printf("[AutoStreamManager] onFrame serviceID=%s seq=%d agents=%d", serviceID, frame.Sequence, len(m.agentRegistry))

	// Fan-out: iterate all agents
	for _, agent := range m.agentRegistry {
		// Skip if not OpenAI-compatible type
		if agent.Type != "openai_compat" {
			log.Printf("[AutoStreamManager] Skipping agent %s: type %s != openai_compat", agent.ID, agent.Type)
			continue
		}

		// Skip if no base_url configured (checking effective config)
		effectiveConfig := m.getEffectiveConfig(agent)
		if effectiveConfig.BaseUrl == "" {
			log.Printf("[AutoStreamManager] Skipping agent %s: empty base_url (effective)", agent.ID)
			continue
		}

		// Check if agent can see this service
		if !contains(agent.ServiceIDs, serviceID) {
			log.Printf("[AutoStreamManager] Skipping agent %s: service %s not in %v", agent.ID, serviceID, agent.ServiceIDs)
			continue
		}

		// Batch and send to OpenAI base_url
		log.Printf("[AutoStreamManager] Batching frame %d for agent %s", frame.Sequence, agent.ID)
		m.batchAndSend(agent, frame)
	}
}

// batchAndSend batches frames and sends them to the agent's OpenAI base_url
func (m *AutoStreamManager) batchAndSend(agent *Agent, frame *Frame) {
	m.batchesMu.Lock()

	// Composite key for batching per agent per service
	batchKey := agent.ID + ":" + frame.ServiceID

	// Add frame to batch
	if m.frameBatches[batchKey] == nil {
		m.frameBatches[batchKey] = make([]*Frame, 0, m.config.FrameBatchSize)
	}
	m.frameBatches[batchKey] = append(m.frameBatches[batchKey], frame)

	// Check if batch is ready
	batch := m.frameBatches[batchKey]
	shouldSend := len(batch) >= m.config.FrameBatchSize

	if shouldSend {
		// Copy batch and clear
		framesToSend := make([]*Frame, len(batch))
		copy(framesToSend, batch)
		m.frameBatches[batchKey] = nil
		m.batchesMu.Unlock()

		// Store frames and collect UUIDs
		frameUUIDs := make([]string, len(framesToSend))
		for i, f := range framesToSend {
			frameUUIDs[i] = uuid.New().String()
			// Store to disk (fast)
			storagePath, err := m.storageManager.Store(frameUUIDs[i], f.ServiceID, f.Data)
			if err != nil {
				log.Printf("[AutoStreamManager] Failed to store frame %s: %v", frameUUIDs[i], err)
				continue
			}
			// Store to DB (fast, naturally serialized in loop)
			metadata := map[string]interface{}{"captured_at": f.Timestamp.Format(time.RFC3339)}
			if err := m.relay.StorageTable.CreateStorage(frameUUIDs[i], f.ServiceID, "frame", storagePath, f.Timestamp, int64(len(f.Data)), "image/jpeg", metadata); err != nil {
				log.Printf("[AutoStreamManager] Failed to store frame metadata %s: %v", frameUUIDs[i], err)
			}
		}

		// Send asynchronously (don't block frame processing)
		// TODO: There is probably a bug here, where the agent event fordwarded to browser sooner than the frames being written to DB / disks
		// Leaving some frames cannot be shown in frontend right away (can be shown after several seconds).
		// Need to confirm again.
		go m.sendToOpenAI(agent, frame.ServiceID, framesToSend, frameUUIDs)
	} else {
		m.batchesMu.Unlock()
	}
}

// sendToOpenAI sends frames to the agent's OpenAI base_url
func (m *AutoStreamManager) sendToOpenAI(agent *Agent, serviceID string, frames []*Frame, frameUUIDs []string) {
	log.Printf("[AutoStreamManager] Sending %d frames to OpenAI for agent %s (service %s)", len(frames), agent.ID, serviceID)

	// Get or create OpenAI client for this agent
	client := m.getOrCreateClient(agent)

	// Get effective config (includes defaults)
	effectiveConfig := m.getEffectiveConfig(agent)
	log.Printf("[AutoStreamManager] Using base_url=%s, model=%s for agent %s", effectiveConfig.BaseUrl, effectiveConfig.Model, agent.ID)

	// Send request
	startTime := time.Now()
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.config.AutoStreamTimeout)*time.Second)
	defer cancel()

	response, err := client.SendFrameBatch(ctx, frames, agent.Config.Instruction, effectiveConfig.Model)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[AutoStreamManager] Failed to send frames to OpenAI for agent %s: %v", agent.ID, err)
		// Drop frames silently as per requirements
		return
	}

	// Store agent event in database and broadcast to clients
	// Convert []string to []interface{} for structpb compatibility
	frameUUIDsInterface := make([]interface{}, len(frameUUIDs))
	for i, uuid := range frameUUIDs {
		frameUUIDsInterface[i] = uuid
	}
	eventData := map[string]interface{}{
		"content":     response.Choices[0].Message.Content,
		"frame_uuids": frameUUIDsInterface,
	}

	metadata := map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}

	// Persist to database
	if m.relay.AgentEventTable != nil {
		if _, err := m.relay.AgentEventTable.CreateEvent(agent.ID, serviceID, eventData, metadata, time.Now()); err != nil {
			log.Printf("[AutoStreamManager] Failed to store agent event: %v", err)
		}
	}

	// Broadcast to connected clients via realtime manager
	if m.relay.ClientRealtimeMgr != nil {
		m.relay.ClientRealtimeMgr.BroadcastAgentEvent(agent.ID, serviceID, eventData, metadata, time.Now().Unix())
	}

	log.Printf("[AutoStreamManager] Sent %d frames to agent %s (duration=%v, tokens=%d)",
		len(frames), agent.ID, duration, response.Usage.TotalTokens)
}

// getEffectiveConfig returns the agent's config with global defaults applied for empty values
func (m *AutoStreamManager) getEffectiveConfig(agent *Agent) AgentConfig {
	config := agent.Config

	baseUrl := config.BaseUrl
	if baseUrl == "" {
		baseUrl = m.config.AutoStreamBaseURL
	}

	model := config.Model
	if model == "" {
		model = m.config.AutoStreamModel
	}

	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = m.config.AutoStreamAPIKey
	}

	return AgentConfig{
		Instruction: config.Instruction,
		BaseUrl:     baseUrl,
		Model:       model,
		APIKey:      apiKey,
	}
}

// getOrCreateClient gets or creates an OpenAI client for the given agent
func (m *AutoStreamManager) getOrCreateClient(agent *Agent) *OpenAIClient {
	m.clientsMu.RLock()
	client, exists := m.openaiClients[agent.ID]
	m.clientsMu.RUnlock()

	if exists {
		log.Printf("[AutoStreamManager] Using cached OpenAI client for agent %s", agent.ID)
		return client
	}

	// Create new client
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	// Double-check (another goroutine might have created it)
	if client, exists := m.openaiClients[agent.ID]; exists {
		return client
	}

	// Get effective config with defaults applied
	effectiveConfig := m.getEffectiveConfig(agent)

	timeout := time.Duration(m.config.AutoStreamTimeout) * time.Second
	client = NewOpenAIClient(effectiveConfig.BaseUrl, timeout, effectiveConfig.Model, effectiveConfig.APIKey)
	m.openaiClients[agent.ID] = client

	log.Printf("[AutoStreamManager] Created OpenAI client for agent %s (base_url=%s)", agent.ID, effectiveConfig.BaseUrl)
	return client
}

// OnAgentCreated is called when a new agent is created
func (m *AutoStreamManager) OnAgentCreated(agent *Agent) {
	if !m.config.AutostreamEnable {
		return
	}

	if agent.Type != "openai_compat" {
		return
	}

	effectiveConfig := m.getEffectiveConfig(agent)
	if effectiveConfig.BaseUrl == "" {
		log.Printf("[AutoStreamManager] Agent created but no base_url configured (even with defaults): %s", agent.ID)
		return
	}

	log.Printf("[AutoStreamManager] Agent created: %s (type=%s, base_url=%s)", agent.ID, agent.Type, effectiveConfig.BaseUrl)

	// Add to registry
	m.agentsMu.Lock()
	m.agentRegistry[agent.ID] = agent
	m.agentsMu.Unlock()

	// Ensure streams exist for agent's services
	for _, serviceID := range agent.ServiceIDs {
		if err := m.ensureServiceStream(serviceID); err != nil {
			log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
		}
	}
}

// OnAgentUpdated is called when an agent is updated
func (m *AutoStreamManager) OnAgentUpdated(agent *Agent) {
	log.Printf("[AutoStreamManager] Agent updated: %s", agent.ID)

	// Get old agent to compare service IDs
	m.agentsMu.Lock()
	oldAgent := m.agentRegistry[agent.ID]
	m.agentRegistry[agent.ID] = agent
	m.agentsMu.Unlock()

	if !m.config.AutostreamEnable {
		return
	}

	effectiveConfig := m.getEffectiveConfig(agent)
	if agent.Type != "openai_compat" || effectiveConfig.BaseUrl == "" {
		// Agent no longer needs streaming, clean up streams if needed
		m.cleanupUnusedStreams()
		return
	}

	// Create streams for new services
	for _, serviceID := range agent.ServiceIDs {
		if oldAgent == nil || !contains(oldAgent.ServiceIDs, serviceID) {
			if err := m.ensureServiceStream(serviceID); err != nil {
				log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
			}
		}
	}

	// Clean up streams for removed services
	m.cleanupUnusedStreams()
}

// OnAgentDeleted is called when an agent is deleted
func (m *AutoStreamManager) OnAgentDeleted(agentID string) {
	log.Printf("[AutoStreamManager] Agent deleted: %s", agentID)

	// Remove from registry
	m.agentsMu.Lock()
	delete(m.agentRegistry, agentID)
	m.agentsMu.Unlock()

	// Clear batches
	m.batchesMu.Lock()
	for k := range m.frameBatches {
		id, _ := splitKey(k)
		if id == agentID {
			delete(m.frameBatches, k)
		}
	}
	m.batchesMu.Unlock()

	// Clean up client
	m.clientsMu.Lock()
	delete(m.openaiClients, agentID)
	m.clientsMu.Unlock()

	// Clean up unused streams
	m.cleanupUnusedStreams()
}

// OnServiceCreated is called when a new service is created
func (m *AutoStreamManager) OnServiceCreated(serviceID string, nodeID string) {
	if !m.config.AutostreamEnable {
		return
	}
	log.Printf("[AutoStreamManager] Service created: %s (node=%s)", serviceID, nodeID)

	// Check if any agent needs this service
	m.agentsMu.RLock()
	needsStream := false
	for _, agent := range m.agentRegistry {
		effectiveConfig := m.getEffectiveConfig(agent)
		if agent.Type == "openai_compat" && effectiveConfig.BaseUrl != "" {
			if contains(agent.ServiceIDs, serviceID) {
				needsStream = true
				break
			}
		}
	}
	m.agentsMu.RUnlock()

	if needsStream {
		if err := m.ensureServiceStream(serviceID); err != nil {
			log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
		}
	}
}

// OnServiceUpdated is called when a service is updated
func (m *AutoStreamManager) OnServiceUpdated(serviceID string, nodeID string) {
	log.Printf("[AutoStreamManager] Service updated: %s (node=%s)", serviceID, nodeID)

	if !m.config.AutostreamEnable {
		return
	}

	m.streamsMu.RLock()
	existingStream, exists := m.serviceStreams[serviceID]
	m.streamsMu.RUnlock()

	if exists {
		// Check if node changed
		if existingStream.service.NodeID != nodeID {
			log.Printf("[AutoStreamManager] Service %s node changed from %s to %s, recreating stream", serviceID, existingStream.service.NodeID, nodeID)
			// Remove old stream (node changed, need to reconnect)
			m.removeServiceStream(serviceID)
		} else {
			// Node didn't change, but URL might have - recreate to pick up new config
			log.Printf("[AutoStreamManager] Service %s updated, recreating stream to pick up new config", serviceID)
			m.removeServiceStream(serviceID)
		}
	}

	// Check if any agent needs this service
	m.agentsMu.RLock()
	needsStream := false
	for _, agent := range m.agentRegistry {
		effectiveConfig := m.getEffectiveConfig(agent)
		if agent.Type == "openai_compat" && effectiveConfig.BaseUrl != "" {
			if contains(agent.ServiceIDs, serviceID) {
				needsStream = true
				break
			}
		}
	}
	m.agentsMu.RUnlock()

	if needsStream {
		if err := m.ensureServiceStream(serviceID); err != nil {
			log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
		}
	}
}

// OnServiceDeleted is called when a service is deleted
func (m *AutoStreamManager) OnServiceDeleted(serviceID string) {
	log.Printf("[AutoStreamManager] Service deleted: %s", serviceID)

	// Remove the stream for this service
	m.removeServiceStream(serviceID)

	// Clear any pending batches for this service
	m.batchesMu.Lock()
	for k := range m.frameBatches {
		_, svcID := splitKey(k)
		if svcID == serviceID {
			delete(m.frameBatches, k)
		}
	}
	m.batchesMu.Unlock()
}

// OnNodeReady is called when a node signals it's ready after registration
func (m *AutoStreamManager) OnNodeReady(nodeID string) {
	if !m.config.AutostreamEnable {
		log.Printf("[AutoStreamManager] Node ready but autostream disabled: %s", nodeID)
		return
	}
	log.Printf("[AutoStreamManager] Node ready: %s (checking for needed services)", nodeID)

	// Find services that are needed by agents and are on this node
	m.agentsMu.RLock()
	neededServices := make(map[string]bool)
	for _, agent := range m.agentRegistry {
		effectiveConfig := m.getEffectiveConfig(agent)
		if agent.Type == "openai_compat" && effectiveConfig.BaseUrl != "" {
			for _, serviceID := range agent.ServiceIDs {
				// Check if service exists and is on this node
				service, err := m.relay.ServiceTable.GetService(serviceID)
				if err == nil && service.NodeID == nodeID {
					neededServices[serviceID] = true
				}
			}
		}
	}
	m.agentsMu.RUnlock()

	log.Printf("[AutoStreamManager] Found %d services to create streams for on node %s: %v", len(neededServices), nodeID, mapKeys(neededServices))

	// Create streams for needed services on this node
	for serviceID := range neededServices {
		if err := m.ensureServiceStream(serviceID); err != nil {
			log.Printf("[AutoStreamManager] Failed to create stream for service %s: %v", serviceID, err)
		}
	}
}

// OnNodeDisconnected is called when a node disconnects
func (m *AutoStreamManager) OnNodeDisconnected(nodeID string) {
	log.Printf("[AutoStreamManager] Node disconnected: %s", nodeID)

	// Find and close all streams for services on this node
	m.streamsMu.RLock()
	streamsToClose := []string{}
	for serviceID, stream := range m.serviceStreams {
		if stream.service.NodeID == nodeID {
			streamsToClose = append(streamsToClose, serviceID)
		}
	}
	m.streamsMu.Unlock()

	for _, serviceID := range streamsToClose {
		m.removeServiceStream(serviceID)
	}
}

// cleanupUnusedStreams closes streams that are no longer needed by any agent
func (m *AutoStreamManager) cleanupUnusedStreams() {
	// Get set of services still needed
	neededServices := make(map[string]bool)
	m.agentsMu.RLock()
	for _, agent := range m.agentRegistry {
		effectiveConfig := m.getEffectiveConfig(agent)
		if agent.Type == "openai_compat" && effectiveConfig.BaseUrl != "" {
			for _, serviceID := range agent.ServiceIDs {
				neededServices[serviceID] = true
			}
		}
	}
	m.agentsMu.RUnlock()

	// Close streams that are no longer needed
	m.streamsMu.RLock()
	streamsToClose := []string{}
	for serviceID := range m.serviceStreams {
		if !neededServices[serviceID] {
			streamsToClose = append(streamsToClose, serviceID)
		}
	}
	m.streamsMu.RUnlock()

	for _, serviceID := range streamsToClose {
		m.removeServiceStream(serviceID)
	}
}

// Helper functions

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// mapKeys returns the keys of a map as a slice
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// splitKey splits a composite key (agentID:serviceID) into its parts
func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
