package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/turso-go"
)

// Database wraps sql.DB with Turso-specific functionality
type Database struct {
	*sql.DB
}

// NewDatabase creates a new database connection and initializes the schema
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("turso", dbPath)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		return nil, err
	}

	return &Database{db}, nil
}

// initSchema creates the necessary tables if they don't exist
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		owner_id INTEGER,
		name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		authorized_at DATETIME,
		last_connected_at DATETIME,
		FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_token ON nodes(token);
	CREATE INDEX IF NOT EXISTS idx_nodes_owner ON nodes(owner_id);

	CREATE TABLE IF NOT EXISTS nodes_users (
		node_id TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		role TEXT NOT NULL DEFAULT 'owner',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (node_id, user_id),
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_users_user ON nodes_users(user_id);

	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		worker_id TEXT NOT NULL,
		config TEXT NOT NULL,
		service_ids TEXT,
		user_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);
	CREATE INDEX IF NOT EXISTS idx_agents_worker ON agents(worker_id);

	CREATE TABLE IF NOT EXISTS agent_events (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		data TEXT NOT NULL,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_agent_events_agent_id ON agent_events(agent_id);
	CREATE INDEX IF NOT EXISTS idx_agent_events_created_at ON agent_events(created_at);
	`
	_, err := db.Exec(schema)
	return err
}

// AgentEvent represents an event emitted by an agent
type AgentEvent struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// StoreAgentEvent stores an agent event in the database
func (db *Database) StoreAgentEvent(agentID string, data map[string]interface{}, metadata map[string]interface{}, createdAt time.Time) error {
	eventID := uuid.New().String()

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal agent event data: %w", err)
	}

	var metadataJSON sql.NullString
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal agent event metadata: %w", err)
		}
		metadataJSON = sql.NullString{String: string(metadataBytes), Valid: true}
	}

	_, err = db.Exec(
		"INSERT INTO agent_events (id, agent_id, data, metadata, created_at) VALUES (?, ?, ?, ?, ?)",
		eventID, agentID, string(dataJSON), metadataJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store agent event: %w", err)
	}

	return nil
}

// GetAgentEvents retrieves events for a specific agent
func (db *Database) GetAgentEvents(agentID string, limit int) ([]*AgentEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, agent_id, data, metadata, created_at
		FROM agent_events
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent events: %w", err)
	}
	defer rows.Close()

	var events []*AgentEvent
	for rows.Next() {
		var event AgentEvent
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}

		if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent event data: %w", err)
		}

		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal agent event metadata: %w", err)
			}
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent events: %w", err)
	}

	return events, nil
}

// GetAgentEventsByService retrieves agent events for agents monitoring a specific service
func (db *Database) GetAgentEventsByService(serviceID string, userID int64, limit int) ([]*AgentEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT DISTINCT ae.id, ae.agent_id, ae.data, ae.metadata, ae.created_at
		FROM agent_events ae
		JOIN agents a ON ae.agent_id = a.id
		WHERE a.user_id = ?
		AND EXISTS (
			SELECT 1 FROM json_each(a.service_ids)
			WHERE json_each.value = ?
		)
		ORDER BY ae.created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, userID, serviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent events by service: %w", err)
	}
	defer rows.Close()

	var events []*AgentEvent
	for rows.Next() {
		var event AgentEvent
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}

		if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent event data: %w", err)
		}

		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal agent event metadata: %w", err)
			}
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent events: %w", err)
	}

	return events, nil
}

// GetRecentAgentEvents retrieves recent agent events across all agents
func (db *Database) GetRecentAgentEvents(limit int) ([]*AgentEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, agent_id, data, metadata, created_at
		FROM agent_events
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent agent events: %w", err)
	}
	defer rows.Close()

	var events []*AgentEvent
	for rows.Next() {
		var event AgentEvent
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}

		if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent event data: %w", err)
		}

		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal agent event metadata: %w", err)
			}
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent events: %w", err)
	}

	return events, nil
}
