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

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, err
	}

	return &Database{db}, nil
}

// initSchema creates the necessary tables if they don't exist
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		owner_id TEXT,
		hostname TEXT,
		mac_addresses TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		authorized_at DATETIME,
		last_connected_at DATETIME,
		FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_token ON nodes(token);
	CREATE INDEX IF NOT EXISTS idx_nodes_owner ON nodes(owner_id);

	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		node_id TEXT NOT NULL,
		service_url TEXT NOT NULL,
		tags TEXT,
		status TEXT DEFAULT 'active',
		owner_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_services_node ON services(node_id);
	CREATE INDEX IF NOT EXISTS idx_services_owner ON services(owner_id);
	CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
	CREATE INDEX IF NOT EXISTS idx_services_owner_status ON services(owner_id, status);

	CREATE TABLE IF NOT EXISTS nodes_users (
		node_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
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
		type TEXT NOT NULL DEFAULT 'openai_compat',
		config TEXT NOT NULL,
		service_ids TEXT,
		user_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);
	CREATE INDEX IF NOT EXISTS idx_agents_type ON agents(type);

	CREATE TABLE IF NOT EXISTS agent_events (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		service_id TEXT,
		data TEXT NOT NULL,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_agent_events_agent_id ON agent_events(agent_id);
	CREATE INDEX IF NOT EXISTS idx_agent_events_service_id ON agent_events(service_id);
	CREATE INDEX IF NOT EXISTS idx_agent_events_created_at ON agent_events(created_at);

	CREATE TABLE IF NOT EXISTS storage (
		id TEXT PRIMARY KEY,
		service_id TEXT NOT NULL,
		type TEXT NOT NULL,
		storage_path TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		file_size INTEGER,
		content_type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT,
		FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_storage_service_id ON storage(service_id);
	CREATE INDEX IF NOT EXISTS idx_storage_type ON storage(type);
	CREATE INDEX IF NOT EXISTS idx_storage_timestamp ON storage(timestamp);
	CREATE INDEX IF NOT EXISTS idx_storage_service_type ON storage(service_id, type);
	`
	_, err := db.Exec(schema)
	return err
}

// runMigrations runs database migrations for schema changes
func runMigrations(db *sql.DB) error {
	// Migration 1: Change worker_id to type in agents table
	var columnExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('agents')
		WHERE name = 'worker_id'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check for worker_id column: %w", err)
	}

	if columnExists {
		// worker_id column exists, need to migrate
		// SQLite doesn't support dropping columns easily, so we'll:
		// 1. Create new table with correct schema
		// 2. Copy data (set type to 'openai_compat' for all existing agents)
		// 3. Drop old table
		// 4. Rename new table

		_, err := db.Exec(`
			-- Create new table with correct schema
			CREATE TABLE agents_new (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				type TEXT NOT NULL DEFAULT 'openai_compat',
				config TEXT NOT NULL,
				service_ids TEXT,
				user_id INTEGER NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);

			-- Copy data from old table (set all to openai_compat type)
			INSERT INTO agents_new (id, name, type, config, service_ids, user_id, created_at, updated_at)
			SELECT id, name, 'openai_compat', config, service_ids, user_id, created_at, updated_at
			FROM agents;

			-- Drop old table
			DROP TABLE agents;

			-- Rename new table
			ALTER TABLE agents_new RENAME TO agents;

			-- Recreate indexes
			CREATE INDEX idx_agents_user ON agents(user_id);
			CREATE INDEX idx_agents_type ON agents(type);
		`)

		if err != nil {
			return fmt.Errorf("failed to migrate agents table: %w", err)
		}
	}

	// Migration 2: Rename name -> hostname and add mac_addresses to nodes table
	err = db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('nodes')
		WHERE name = 'name'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check for name column: %w", err)
	}

	if columnExists {
		// name column exists, rename it to hostname and add mac_addresses
		_, err := db.Exec(`
			-- Rename name column to hostname
			ALTER TABLE nodes RENAME COLUMN name TO hostname;

			-- Add mac_addresses column
			ALTER TABLE nodes ADD COLUMN mac_addresses TEXT;
		`)

		if err != nil {
			return fmt.Errorf("failed to migrate nodes table: %w", err)
		}
	}

	// Migration 3: Change user IDs from INTEGER to TEXT (UUID)
	var userIDType string
	err = db.QueryRow(`
		SELECT type
		FROM pragma_table_info('users')
		WHERE name = 'id'
	`).Scan(&userIDType)

	if err != nil {
		return fmt.Errorf("failed to check users id type: %w", err)
	}

	if userIDType == "INTEGER" {
		// Old schema with INTEGER user IDs - migrate to TEXT (UUID)
		// This is a complex migration that touches multiple tables
		_, err := db.Exec(`
			-- Create new users table with TEXT id
			CREATE TABLE users_new (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				name TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			-- Migrate users with generated UUIDs (using hex of rowid as deterministic UUID)
			INSERT INTO users_new (id, email, password_hash, name, created_at)
			SELECT
				lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))),
				email,
				password_hash,
				name,
				created_at
			FROM users;

			-- Create new nodes_users table with TEXT user_id
			CREATE TABLE nodes_users_new (
				node_id TEXT NOT NULL,
				user_id TEXT NOT NULL,
				role TEXT NOT NULL DEFAULT 'owner',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (node_id, user_id),
				FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
				FOREIGN KEY (user_id) REFERENCES users_new(id) ON DELETE CASCADE
			);

			-- Migrate nodes_users using the new user IDs (match by rowid)
			INSERT INTO nodes_users_new (node_id, user_id, role, created_at)
			SELECT nu.node_id, u_new.id, nu.role, nu.created_at
			FROM nodes_users nu
			JOIN users u_old ON nu.user_id = u_old.rowid
			JOIN users_new u_new ON u_new.email = u_old.email;

			-- Create new agents table with TEXT user_id
			CREATE TABLE agents_new (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				type TEXT NOT NULL DEFAULT 'openai_compat',
				config TEXT NOT NULL,
				service_ids TEXT,
				user_id TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users_new(id) ON DELETE CASCADE
			);

			-- Migrate agents using the new user IDs
			INSERT INTO agents_new (id, name, type, config, service_ids, user_id, created_at, updated_at)
			SELECT a.id, a.name, a.type, a.config, a.service_ids, u_new.id, a.created_at, a.updated_at
			FROM agents a
			JOIN users u_old ON a.user_id = u_old.rowid
			JOIN users_new u_new ON u_new.email = u_old.email;

			-- Create new services table with TEXT owner_id
			CREATE TABLE services_new (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				node_id TEXT NOT NULL,
				service_url TEXT NOT NULL,
				tags TEXT,
				status TEXT DEFAULT 'active',
				owner_id TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
				FOREIGN KEY (owner_id) REFERENCES users_new(id) ON DELETE CASCADE
			);

			-- Migrate services using the new user IDs
			INSERT INTO services_new (id, name, description, node_id, service_url, tags, status, owner_id, created_at, updated_at)
			SELECT s.id, s.name, s.description, s.node_id, s.service_url, s.tags, s.status, u_new.id, s.created_at, s.updated_at
			FROM services s
			JOIN users u_old ON s.owner_id = u_old.rowid
			JOIN users_new u_new ON u_new.email = u_old.email;

			-- Create new nodes table with TEXT owner_id
			CREATE TABLE nodes_new (
				id TEXT PRIMARY KEY,
				token TEXT UNIQUE NOT NULL,
				owner_id TEXT,
				hostname TEXT,
				mac_addresses TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				authorized_at DATETIME,
				last_connected_at DATETIME,
				FOREIGN KEY (owner_id) REFERENCES users_new(id) ON DELETE SET NULL
			);

			-- Migrate nodes using the new user IDs (owner_id can be NULL)
			INSERT INTO nodes_new (id, token, owner_id, hostname, mac_addresses, created_at, authorized_at, last_connected_at)
			SELECT n.id, n.token, u_new.id, n.hostname, n.mac_addresses, n.created_at, n.authorized_at, n.last_connected_at
			FROM nodes n
			LEFT JOIN users u_old ON n.owner_id = u_old.rowid
			LEFT JOIN users_new u_new ON u_new.email = u_old.email;

			-- Drop old tables
			DROP TABLE nodes_users;
			DROP TABLE agents;
			DROP TABLE services;
			DROP TABLE nodes;
			DROP TABLE users;

			-- Rename new tables
			ALTER TABLE users_new RENAME TO users;
			ALTER TABLE nodes_users_new RENAME TO nodes_users;
			ALTER TABLE agents_new RENAME TO agents;
			ALTER TABLE services_new RENAME TO services;
			ALTER TABLE nodes_new RENAME TO nodes;

			-- Recreate indexes
			CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
			CREATE INDEX IF NOT EXISTS idx_nodes_token ON nodes(token);
			CREATE INDEX IF NOT EXISTS idx_nodes_owner ON nodes(owner_id);
			CREATE INDEX IF NOT EXISTS idx_services_node ON services(node_id);
			CREATE INDEX IF NOT EXISTS idx_services_owner ON services(owner_id);
			CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
			CREATE INDEX IF NOT EXISTS idx_services_owner_status ON services(owner_id, status);
			CREATE INDEX IF NOT EXISTS idx_nodes_users_user ON nodes_users(user_id);
			CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);
			CREATE INDEX IF NOT EXISTS idx_agents_type ON agents(type);
		`)

		if err != nil {
			return fmt.Errorf("failed to migrate user IDs to UUID: %w", err)
		}
	}

	// Migration 4: Add service_id to agent_events table
	err = db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('agent_events')
		WHERE name = 'service_id'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check for service_id column: %w", err)
	}

	if !columnExists {
		_, err := db.Exec(`ALTER TABLE agent_events ADD COLUMN service_id TEXT;`)
		if err != nil {
			return fmt.Errorf("failed to add service_id column to agent_events: %w", err)
		}
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_events_service_id ON agent_events(service_id);`)
		if err != nil {
			return fmt.Errorf("failed to create index on service_id: %w", err)
		}
	}

	return nil
}

// AgentEvent represents an event emitted by an agent
type AgentEvent struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	ServiceID string                 `json:"service_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// StoreAgentEvent stores an agent event in the database
func (db *Database) StoreAgentEvent(agentID string, serviceID string, data map[string]interface{}, metadata map[string]interface{}, createdAt time.Time) error {
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
		"INSERT INTO agent_events (id, agent_id, service_id, data, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		eventID, agentID, serviceID, string(dataJSON), metadataJSON, createdAt,
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
		SELECT id, agent_id, service_id, data, metadata, created_at
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
		var serviceID sql.NullString
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &serviceID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}
		if serviceID.Valid {
			event.ServiceID = serviceID.String
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

// GetAgentEventsByService retrieves agent events for a specific service
func (db *Database) GetAgentEventsByService(serviceID string, userID string, limit int) ([]*AgentEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT ae.id, ae.agent_id, ae.service_id, ae.data, ae.metadata, ae.created_at
		FROM agent_events ae
		JOIN agents a ON ae.agent_id = a.id
		WHERE a.user_id = ?
		AND ae.service_id = ?
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
		var serviceID sql.NullString
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &serviceID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}
		if serviceID.Valid {
			event.ServiceID = serviceID.String
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
		SELECT id, agent_id, service_id, data, metadata, created_at
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
		var serviceID sql.NullString
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&event.ID, &event.AgentID, &serviceID, &dataJSON, &metadataJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}
		if serviceID.Valid {
			event.ServiceID = serviceID.String
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
