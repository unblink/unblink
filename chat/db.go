package chat

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"

	_ "github.com/tursodatabase/turso-go"
)

type DB struct {
	*sql.DB
}

func NewDB(unblinkDir string) (*DB, error) {
	dbPath := filepath.Join(unblinkDir, "database", "chat.db")
	db, err := sql.Open("turso", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open chat database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping chat database: %w", err)
	}

	d := &DB{DB: db}
	if err := d.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	log.Printf("[Chat] Chat database opened: %s", dbPath)

	return d, nil
}

func (d *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			title TEXT,
			system_prompt TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			body TEXT,
			created_at DATETIME,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id)
		)`,
		`CREATE TABLE IF NOT EXISTS ui_blocks (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			data_json TEXT NOT NULL,
			created_at DATETIME,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ui_blocks_conv_created ON ui_blocks(conversation_id, created_at)`,
	}

	for _, query := range queries {
		if _, err := d.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
