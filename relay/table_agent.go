package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Agent represents an agent in the system
type Agent struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       string      `json:"type"` // Agent type: "openai_compat" or other future types
	Config     AgentConfig `json:"config"`
	ServiceIDs []string    `json:"service_ids"`
	UserID     string      `json:"user_id"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// AgentConfig holds the agent configuration (stored as JSON)
type AgentConfig struct {
	Instruction string `json:"instruction"`
	BaseUrl     string `json:"base_url"` // OpenAI-compatible base URL
	Model       string `json:"model"`    // Optional model override
	APIKey      string `json:"api_key"`  // Optional API key (defaults to global AUTOSTREAM_OPENAI_API_KEY)
}

// table_agent provides CRUD operations for agents
type table_agent struct {
	db *Database
}

// NewAgentTable creates a new agent table accessor
func NewAgentTable(db *Database) *table_agent {
	return &table_agent{db: db}
}

// CreateAgent creates a new agent
func (t *table_agent) CreateAgent(name, agentType string, config AgentConfig, serviceIDs []string, userID string) (*Agent, error) {
	// Validation
	if name == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if config.Instruction == "" {
		return nil, fmt.Errorf("agent instruction is required")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("name too long (max 255 characters)")
	}
	if len(config.Instruction) > 5000 {
		return nil, fmt.Errorf("instruction too long (max 5000 characters)")
	}

	// Default type if not provided
	if agentType == "" {
		agentType = "openai_compat"
	}

	agentID := uuid.New().String()
	now := time.Now()

	// Prepare config as JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Prepare service_ids as JSON
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	serviceIDsJSON, err := json.Marshal(serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service IDs: %w", err)
	}

	// Insert into database
	_, err = t.db.Exec(`
		INSERT INTO agents (id, name, type, config, service_ids, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, agentID, name, agentType, string(configJSON), string(serviceIDsJSON), userID, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return t.GetAgentByID(agentID)
}

// GetAgentByID retrieves an agent by ID
func (t *table_agent) GetAgentByID(agentID string) (*Agent, error) {
	var agent Agent
	var configJSON, serviceIDsJSON sql.NullString

	err := t.db.QueryRow(`
		SELECT id, name, type, config, service_ids, user_id, created_at, updated_at
		FROM agents WHERE id = ?
	`, agentID).Scan(
		&agent.ID, &agent.Name, &agent.Type, &configJSON, &serviceIDsJSON,
		&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Parse config JSON
	if configJSON.Valid {
		if err := json.Unmarshal([]byte(configJSON.String), &agent.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// Parse service_ids JSON
	if serviceIDsJSON.Valid && serviceIDsJSON.String != "" && serviceIDsJSON.String != "null" {
		if err := json.Unmarshal([]byte(serviceIDsJSON.String), &agent.ServiceIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal service IDs: %w", err)
		}
	} else {
		agent.ServiceIDs = []string{}
	}

	return &agent, nil
}

// GetAgentsByUser retrieves all agents owned by a user
func (t *table_agent) GetAgentsByUser(userID string) ([]*Agent, error) {
	rows, err := t.db.Query(`
		SELECT id, name, type, config, service_ids, user_id, created_at, updated_at
		FROM agents WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var configJSON, serviceIDsJSON sql.NullString

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.Type, &configJSON, &serviceIDsJSON,
			&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		// Parse config JSON
		if configJSON.Valid {
			if err := json.Unmarshal([]byte(configJSON.String), &agent.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		}

		// Parse service_ids JSON
		if serviceIDsJSON.Valid && serviceIDsJSON.String != "" && serviceIDsJSON.String != "null" {
			if err := json.Unmarshal([]byte(serviceIDsJSON.String), &agent.ServiceIDs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal service IDs: %w", err)
			}
		} else {
			agent.ServiceIDs = []string{}
		}

		agents = append(agents, &agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// UpdateAgent performs a partial update on an agent
// Pass nil for fields that should not be updated
func (t *table_agent) UpdateAgent(agentID string, name, agentType *string, config *AgentConfig, serviceIDs *[]string) (*Agent, error) {
	// Build dynamic SQL update
	var updates []string
	var args []interface{}

	if name != nil {
		if *name == "" {
			return nil, fmt.Errorf("agent name cannot be empty")
		}
		if len(*name) > 255 {
			return nil, fmt.Errorf("name too long (max 255 characters)")
		}
		updates = append(updates, "name = ?")
		args = append(args, *name)
	}

	if agentType != nil {
		updates = append(updates, "type = ?")
		args = append(args, *agentType)
	}

	if config != nil {
		if config.Instruction == "" {
			return nil, fmt.Errorf("agent instruction cannot be empty")
		}
		if len(config.Instruction) > 5000 {
			return nil, fmt.Errorf("instruction too long (max 5000 characters)")
		}
		configJSON, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config: %w", err)
		}
		updates = append(updates, "config = ?")
		args = append(args, string(configJSON))
	}

	if serviceIDs != nil {
		serviceIDsJSON, err := json.Marshal(*serviceIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal service IDs: %w", err)
		}
		updates = append(updates, "service_ids = ?")
		args = append(args, string(serviceIDsJSON))
	}

	if len(updates) == 0 {
		return t.GetAgentByID(agentID)
	}

	// Add updated_at
	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now())

	// Add agentID to args
	args = append(args, agentID)

	// Execute update
	query := "UPDATE agents SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	result, err := t.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("agent not found")
	}

	return t.GetAgentByID(agentID)
}

// DeleteAgent removes an agent
func (t *table_agent) DeleteAgent(agentID string) error {
	result, err := t.db.Exec("DELETE FROM agents WHERE id = ?", agentID)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// GetAllAgents retrieves all agents (for auto-streaming manager)
func (t *table_agent) GetAllAgents() ([]*Agent, error) {
	rows, err := t.db.Query(`
		SELECT id, name, type, config, service_ids, user_id, created_at, updated_at
		FROM agents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var configJSON, serviceIDsJSON sql.NullString

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.Type, &configJSON, &serviceIDsJSON,
			&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		// Parse config JSON
		if configJSON.Valid {
			if err := json.Unmarshal([]byte(configJSON.String), &agent.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		}

		// Parse service_ids JSON
		if serviceIDsJSON.Valid && serviceIDsJSON.String != "" && serviceIDsJSON.String != "null" {
			if err := json.Unmarshal([]byte(serviceIDsJSON.String), &agent.ServiceIDs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal service IDs: %w", err)
			}
		} else {
			agent.ServiceIDs = []string{}
		}

		agents = append(agents, &agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// UserOwnsAgent checks if a user owns a specific agent
func (t *table_agent) UserOwnsAgent(userID string, agentID string) bool {
	var ownerID string
	err := t.db.QueryRow("SELECT user_id FROM agents WHERE id = ?", agentID).Scan(&ownerID)
	return err == nil && ownerID == userID
}

// RemoveServiceIDFromAllAgents removes a service ID from the service_ids array of all agents
func (t *table_agent) RemoveServiceIDFromAllAgents(serviceID string) error {
	// Use SQLite JSON functions to update in-place:
	// - json_each iterates over the array
	// - json_group_array recreates the array without the filtered value
	_, err := t.db.Exec(`
		UPDATE agents
		SET service_ids = (
			SELECT json_group_array(value)
			FROM json_each(service_ids)
			WHERE value != ?
		),
		updated_at = ?
		WHERE service_ids LIKE ?
	`, serviceID, time.Now(), "%\""+serviceID+"\"%")
	if err != nil {
		return fmt.Errorf("failed to update agents: %w", err)
	}

	return nil
}
