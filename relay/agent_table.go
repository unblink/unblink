package relay

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
)

// Agent represents an agent in the system
type Agent struct {
	ID         string
	Name       string
	WorkerID   string
	Config     AgentConfig
	ServiceIDs []string
	UserID     int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AgentConfig holds the agent configuration (stored as JSONB)
type AgentConfig struct {
	Instruction string `json:"instruction"`
	// Future fields can be added here
}

// AgentTable handles agent data access
type AgentTable struct {
	db *sql.DB
}

// NewAgentTable creates a new AgentTable
func NewAgentTable(db *sql.DB) *AgentTable {
	return &AgentTable{db: db}
}

// CreateAgent creates a new agent
func (at *AgentTable) CreateAgent(name, instruction, workerID string, userID int64) (*Agent, error) {
	// Validation
	if name == "" {
		return nil, errors.New("agent name is required")
	}
	if instruction == "" {
		return nil, errors.New("agent instruction is required")
	}
	if workerID == "" {
		workerID = "unblink/qwen3-vl" // Default worker
	}

	// Generate UUID
	id := uuid.New().String()

	// Prepare config as JSON
	config := AgentConfig{Instruction: instruction}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	// Empty service_ids array
	serviceIDsJSON := "[]"

	// Insert into database
	_, err = at.db.Exec(`
		INSERT INTO agents (id, name, worker_id, config, service_ids, user_id, created_at, updated_at)
		VALUES (?, ?, ?, json(?), json(?), ?, datetime('now'), datetime('now'))
	`, id, name, workerID, string(configJSON), serviceIDsJSON, userID)

	if err != nil {
		return nil, err
	}

	log.Printf("[AgentTable] Agent %s created by user %d", id, userID)

	// Return created agent
	return at.GetAgentByID(id)
}

// GetAgentByID retrieves an agent by ID
func (at *AgentTable) GetAgentByID(agentID string) (*Agent, error) {
	var agent Agent
	var configJSON, serviceIDsJSON string

	err := at.db.QueryRow(`
		SELECT id, name, worker_id, config, service_ids, user_id, created_at, updated_at
		FROM agents WHERE id = ?
	`, agentID).Scan(
		&agent.ID, &agent.Name, &agent.WorkerID, &configJSON, &serviceIDsJSON,
		&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("agent not found")
	}
	if err != nil {
		return nil, err
	}

	// Parse config JSON
	if err := json.Unmarshal([]byte(configJSON), &agent.Config); err != nil {
		return nil, err
	}

	// Parse service_ids JSON
	if serviceIDsJSON == "" || serviceIDsJSON == "null" {
		agent.ServiceIDs = []string{}
	} else {
		if err := json.Unmarshal([]byte(serviceIDsJSON), &agent.ServiceIDs); err != nil {
			return nil, err
		}
	}

	return &agent, nil
}

// GetAgentsByUser retrieves all agents owned by a user
func (at *AgentTable) GetAgentsByUser(userID int64) ([]*Agent, error) {
	rows, err := at.db.Query(`
		SELECT id, name, worker_id, config, service_ids, user_id, created_at, updated_at
		FROM agents WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var configJSON, serviceIDsJSON string

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.WorkerID, &configJSON, &serviceIDsJSON,
			&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSONs
		if err := json.Unmarshal([]byte(configJSON), &agent.Config); err != nil {
			return nil, err
		}

		// Parse service_ids JSON
		if serviceIDsJSON == "" || serviceIDsJSON == "null" {
			agent.ServiceIDs = []string{}
		} else {
			if err := json.Unmarshal([]byte(serviceIDsJSON), &agent.ServiceIDs); err != nil {
				return nil, err
			}
		}

		agents = append(agents, &agent)
	}

	return agents, nil
}

// UpdateAgentServiceIDs updates the service_ids array for an agent
func (at *AgentTable) UpdateAgentServiceIDs(agentID string, serviceIDs []string) error {
	serviceIDsJSON, err := json.Marshal(serviceIDs)
	if err != nil {
		return err
	}

	result, err := at.db.Exec(`
		UPDATE agents
		SET service_ids = json(?), updated_at = datetime('now')
		WHERE id = ?
	`, string(serviceIDsJSON), agentID)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("agent not found")
	}

	log.Printf("[AgentTable] Updated services for agent %s", agentID)
	return nil
}

// DeleteAgent removes an agent
func (at *AgentTable) DeleteAgent(agentID string) error {
	result, err := at.db.Exec("DELETE FROM agents WHERE id = ?", agentID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("agent not found")
	}

	log.Printf("[AgentTable] Deleted agent %s", agentID)
	return nil
}

// UserOwnsAgent checks if a user owns a specific agent
func (at *AgentTable) UserOwnsAgent(userID int64, agentID string) bool {
	var ownerID int64
	err := at.db.QueryRow("SELECT user_id FROM agents WHERE id = ?", agentID).Scan(&ownerID)
	return err == nil && ownerID == userID
}
