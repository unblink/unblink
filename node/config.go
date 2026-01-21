package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Config represents the v2 node configuration
type Config struct {
	RelayAddress string `json:"relay_address"`
	NodeID       string `json:"node_id"`
	Token        string `json:"token"`
	Reconnect    ReconnectConf `json:"reconnect"`
}

// ReconnectConf defines reconnection behavior
type ReconnectConf struct {
	Enabled        bool `json:"enabled"`
	MaxNumAttempts int  `json:"max_num_attempts"`
}

// LoadConfig loads configuration from ~/.unblink/config.json
// If the file doesn't exist, creates a new config with a generated node ID
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	configDir := filepath.Join(homeDir, ".unblink")
	configPath := filepath.Join(configDir, "config.json")

	// Try to read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		// File doesn't exist - create default config
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}

		// Create config directory
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}

		// Create default config
		cfg := &Config{
			RelayAddress: "ws://localhost:9020",
			NodeID:       uuid.New().String(),
			Token:        "",
			Reconnect: ReconnectConf{
				Enabled:        true,
				MaxNumAttempts: 10,
			},
		}

		// Save default config
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}

		return cfg, nil
	}

	// Parse existing config
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Auto-generate node ID if missing
	needsSave := false
	if cfg.NodeID == "" {
		cfg.NodeID = uuid.New().String()
		needsSave = true
	}

	// Set defaults for relay address if missing
	if cfg.RelayAddress == "" {
		cfg.RelayAddress = "ws://localhost:9020"
		needsSave = true
	}

	if needsSave {
		data, err := json.MarshalIndent(&cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
	}

	return &cfg, nil
}

// Save persists the config to ~/.unblink/config.json
func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	configDir := filepath.Join(homeDir, ".unblink")
	configPath := filepath.Join(configDir, "config.json")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
