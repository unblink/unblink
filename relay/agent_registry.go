package relay

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/unblink/unblink/relay/cv"
)

// AgentInfo holds the essential agent information for the in-memory registry
type AgentInfo struct {
	ID          string
	Name        string
	WorkerID    string
	Instruction string
	ServiceIDs  []string
}

// AgentRegistry maintains an in-memory index of agents to avoid database queries
// during high-frequency frame_batch event processing
type AgentRegistry struct {
	// serviceID → agents monitoring that service
	serviceAgents map[string][]*AgentInfo

	// agentID → agent details (for quick lookups)
	agents map[string]*AgentInfo

	mu sync.RWMutex
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		serviceAgents: make(map[string][]*AgentInfo),
		agents:        make(map[string]*AgentInfo),
	}
}

// LoadFromDatabase loads all agents from the database into memory
func (r *AgentRegistry) LoadFromDatabase(agentTable *AgentTable) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Query all agents (no user filter - we need all agents for all users)
	rows, err := agentTable.db.Query(`
		SELECT id, name, worker_id, config, service_ids
		FROM agents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var agent Agent
		var configJSON, serviceIDsJSON string

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.WorkerID, &configJSON, &serviceIDsJSON,
		)
		if err != nil {
			log.Printf("[AgentRegistry] Error scanning agent: %v", err)
			continue
		}

		// Parse config JSON
		if err := unmarshalJSON([]byte(configJSON), &agent.Config); err != nil {
			log.Printf("[AgentRegistry] Error parsing config for agent %s: %v", agent.ID, err)
			continue
		}

		// Parse service_ids JSON
		if serviceIDsJSON == "" || serviceIDsJSON == "null" {
			agent.ServiceIDs = []string{}
		} else {
			if err := unmarshalJSON([]byte(serviceIDsJSON), &agent.ServiceIDs); err != nil {
				log.Printf("[AgentRegistry] Error parsing service_ids for agent %s: %v", agent.ID, err)
				continue
			}
		}

		// Convert to AgentInfo and store
		info := &AgentInfo{
			ID:          agent.ID,
			Name:        agent.Name,
			WorkerID:    agent.WorkerID,
			Instruction: agent.Config.Instruction,
			ServiceIDs:  agent.ServiceIDs,
		}

		r.agents[info.ID] = info
		count++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Build service index
	r.buildServiceIndexLocked()

	log.Printf("[AgentRegistry] Loaded %d agents from database", count)
	log.Printf("[AgentRegistry] Built service index: %d services monitored", len(r.serviceAgents))

	return nil
}

// RegisterAgent adds or updates an agent in the registry
func (r *AgentRegistry) RegisterAgent(info *AgentInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store agent
	r.agents[info.ID] = info

	// Rebuild service index
	r.buildServiceIndexLocked()

	log.Printf("[AgentRegistry] Registered agent %s for worker %s", info.ID, info.WorkerID)
	log.Printf("[AgentRegistry] Agent monitors %d services", len(info.ServiceIDs))
}

// RemoveAgent removes an agent from the registry
func (r *AgentRegistry) RemoveAgent(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentID]; !exists {
		return
	}

	delete(r.agents, agentID)

	// Rebuild service index
	r.buildServiceIndexLocked()

	log.Printf("[AgentRegistry] Removed agent %s", agentID)
}

// GetAgentsForService returns all agents monitoring a specific service
// This is an O(1) lookup used during frame_batch event broadcasting
// Returns cv.AgentInfo type for compatibility with CV subsystem
func (r *AgentRegistry) GetAgentsForService(serviceID string) []*cv.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents, exists := r.serviceAgents[serviceID]
	if !exists {
		return nil
	}

	// Convert to cv.AgentInfo type
	result := make([]*cv.AgentInfo, len(agents))
	for i, agent := range agents {
		result[i] = &cv.AgentInfo{
			ID:          agent.ID,
			Name:        agent.Name,
			WorkerID:    agent.WorkerID,
			Instruction: agent.Instruction,
			ServiceIDs:  agent.ServiceIDs,
		}
	}
	return result
}

// buildServiceIndexLocked rebuilds the serviceID → agents index
// Must be called with r.mu write lock held
func (r *AgentRegistry) buildServiceIndexLocked() {
	// Clear existing index
	r.serviceAgents = make(map[string][]*AgentInfo)

	// Build new index
	for _, agent := range r.agents {
		for _, serviceID := range agent.ServiceIDs {
			r.serviceAgents[serviceID] = append(r.serviceAgents[serviceID], agent)
		}
	}
}

// Helper function to unmarshal JSON (reusing existing unmarshal patterns)
func unmarshalJSON(data []byte, v interface{}) error {
	// Use the standard json.Unmarshal
	// This is a helper to keep code consistent with agent_table.go
	return json.Unmarshal(data, v)
}
