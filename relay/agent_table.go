package relay

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
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
func (at *AgentTable) CreateAgent(name, instruction, workerID string, serviceIDs []string, userID int64) (*Agent, error) {
	// Validation
	if name == "" {
		return nil, errors.New("agent name is required")
	}
	if instruction == "" {
		return nil, errors.New("agent instruction is required")
	}
	if workerID == "" {
		return nil, errors.New("worker ID is required")
	}

	// Generate UUID
	id := uuid.New().String()

	// Prepare config as JSON
	config := AgentConfig{Instruction: instruction}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	// Prepare service_ids as JSON
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	serviceIDsJSON, err := json.Marshal(serviceIDs)
	if err != nil {
		return nil, err
	}

	// Insert into database
	_, err = at.db.Exec(`
		INSERT INTO agents (id, name, worker_id, config, service_ids, user_id, created_at, updated_at)
		VALUES (?, ?, ?, json(?), json(?), ?, datetime('now'), datetime('now'))
	`, id, name, workerID, string(configJSON), string(serviceIDsJSON), userID)

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

// UpdateAgent performs a partial update on an agent (name, instruction, and/or service_ids)
// Pass nil for fields that should not be updated
func (at *AgentTable) UpdateAgent(agentID string, name, instruction *string, serviceIDs *[]string) error {
	// Build dynamic SQL update
	var updates []string
	var args []interface{}

	if name != nil {
		if *name == "" {
			return errors.New("agent name cannot be empty")
		}
		updates = append(updates, "name = ?")
		args = append(args, *name)
	}

	if instruction != nil {
		if *instruction == "" {
			return errors.New("agent instruction cannot be empty")
		}
		config := AgentConfig{Instruction: *instruction}
		configJSON, err := json.Marshal(config)
		if err != nil {
			return err
		}
		updates = append(updates, "config = json(?)")
		args = append(args, string(configJSON))
	}

	if serviceIDs != nil {
		serviceIDsJSON, err := json.Marshal(*serviceIDs)
		if err != nil {
			return err
		}
		updates = append(updates, "service_ids = json(?)")
		args = append(args, string(serviceIDsJSON))
	}

	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	// Add updated_at
	updates = append(updates, "updated_at = datetime('now')")

	// Add agentID to args
	args = append(args, agentID)

	// Execute update
	query := "UPDATE agents SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	result, err := at.db.Exec(query, args...)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("agent not found")
	}

	log.Printf("[AgentTable] Updated agent %s", agentID)
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
