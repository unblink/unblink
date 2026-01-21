package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentEventDB represents an agent event in the database
type AgentEventDB struct {
	ID        string
	AgentID   string
	ServiceID string
	Data      map[string]interface{}
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// table_agent_event provides CRUD operations for agent events
type table_agent_event struct {
	db *Database
}

// NewAgentEventTable creates a new agent event table accessor
func NewAgentEventTable(db *Database) *table_agent_event {
	return &table_agent_event{db: db}
}

// CreateEvent creates a new agent event
func (t *table_agent_event) CreateEvent(agentID string, serviceID string, data, metadata map[string]interface{}, createdAt time.Time) (string, error) {
	eventID := uuid.New().String()

	// Prepare data as JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	// Prepare metadata as JSON
	var metadataJSON []byte
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return "", fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Insert into database
	_, err = t.db.Exec(`
		INSERT INTO agent_events (id, agent_id, service_id, data, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, eventID, agentID, serviceID, string(dataJSON), string(metadataJSON), createdAt)

	if err != nil {
		return "", fmt.Errorf("failed to create agent event: %w", err)
	}

	return eventID, nil
}

// ListEvents retrieves agent events for a user, optionally filtered by agent_id or service_id
func (t *table_agent_event) ListEvents(userID, agentID, serviceID string, limit int) ([]*AgentEventDB, error) {
	// Build query with joins to get agent and verify user ownership
	query := `
		SELECT ae.id, ae.agent_id, ae.service_id, ae.data, ae.metadata, ae.created_at
		FROM agent_events ae
		JOIN agents a ON ae.agent_id = a.id
		WHERE a.user_id = ?
	`
	args := []interface{}{userID}

	if agentID != "" {
		query += " AND ae.agent_id = ?"
		args = append(args, agentID)
	}

	if serviceID != "" {
		query += " AND ae.service_id = ?"
		args = append(args, serviceID)
	}

	query += " ORDER BY ae.created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := t.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent events: %w", err)
	}
	defer rows.Close()

	var events []*AgentEventDB
	for rows.Next() {
		var event AgentEventDB
		var dbServiceID sql.NullString
		var dataJSON, metadataJSON sql.NullString

		err := rows.Scan(
			&event.ID, &event.AgentID, &dbServiceID, &dataJSON, &metadataJSON, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}
		if dbServiceID.Valid {
			event.ServiceID = dbServiceID.String
		}

		// Parse data JSON
		if dataJSON.Valid {
			if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal data: %w", err)
			}
		} else {
			event.Data = make(map[string]interface{})
		}

		// Parse metadata JSON
		if metadataJSON.Valid && metadataJSON.String != "" && metadataJSON.String != "null" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			event.Metadata = make(map[string]interface{})
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent events: %w", err)
	}

	return events, nil
}

// getAgentByID is a helper to get agent info for service_id filtering
func (t *table_agent_event) getAgentByID(agentID string) (*Agent, error) {
	var agent Agent
	var configJSON, serviceIDsJSON sql.NullString

	err := t.db.QueryRow(`
		SELECT id, name, type, config, service_ids, user_id, created_at, updated_at
		FROM agents WHERE id = ?
	`, agentID).Scan(
		&agent.ID, &agent.Name, &agent.Type, &configJSON, &serviceIDsJSON,
		&agent.UserID, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Parse service_ids JSON
	if serviceIDsJSON.Valid && serviceIDsJSON.String != "" && serviceIDsJSON.String != "null" {
		if err := json.Unmarshal([]byte(serviceIDsJSON.String), &agent.ServiceIDs); err != nil {
			return nil, err
		}
	} else {
		agent.ServiceIDs = []string{}
	}

	return &agent, nil
}
