package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/unblink/unblink/node"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "Path to config file")
}

func main() {
	// Parse flags first (so -h works)
	flag.Parse()

	// Check for help
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			handleConfigCommand()
			return
		case "login":
			handleLoginCommand()
			return
		case "logout":
			handleLogoutCommand()
			return
		case "uninstall":
			handleUninstallCommand()
			return
		}
	}

	// Default: run node
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("[Node] Failed to load config: %v", err)
	}

	// Log startup info
	log.Printf("[Node] Starting node...")
	log.Printf("[Node] Relay: %s", config.RelayAddress)
	log.Printf("[Node] Node ID: %s", config.NodeID)

	// Create connection
	conn := node.NewConn(config)

	// Connect to relay (this starts RPC server and monitor in background)
	if err := conn.Connect(); err != nil {
		log.Fatalf("[Node] Failed to connect to relay: %v", err)
	}

	// If no token, wait for authorization with timeout
	if config.Token == "" {
		log.Println("[Node] Waiting for authorization...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if !conn.WaitForToken(ctx) {
			log.Println("[Node] Authorization timed out. Exiting.")
			conn.Close()
			os.Exit(1)
		}
		log.Println("[Node] Authorization complete. Node is now running.")
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[Node] Shutting down...")
	conn.Close()
}

func loadConfig(customPath string) (*node.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	configDir := filepath.Join(homeDir, ".unblink")
	configPath := filepath.Join(configDir, "config.json")

	if customPath != "" {
		configPath = customPath
	}

	// Try to read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}

		// File doesn't exist - create default config
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}

		// Create default config
		cfg := &node.Config{
			RelayAddress: "ws://localhost:9020",
			NodeID:       uuid.New().String(),
			Token:        "",
			Reconnect: node.ReconnectConf{
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
	var cfg node.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func mustConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return filepath.Join(homeDir, ".unblink", "config.json")
}

// saveConfig saves the config to its file path
func saveConfig(config *node.Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	configPath := filepath.Join(homeDir, ".unblink", "config.json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func handleConfigCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: node config <command>")
		fmt.Println("Commands:")
		fmt.Println("  show    Show the config file path")
		fmt.Println("  delete  Delete the config file")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "show":
		showConfig()
	case "delete":
		deleteConfig()
	default:
		fmt.Printf("Unknown config command: %s\n", os.Args[2])
		fmt.Println("Available commands: show, delete")
		os.Exit(1)
	}
}

func handleLoginCommand() {
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.Token != "" {
		log.Println("[Node] Already logged in. Use 'node logout' first if you want to re-authorize.")
		os.Exit(1)
	}

	if config.RelayAddress == "" {
		log.Fatalf("[Node] relay_address is not set in config. Edit: %s", mustConfigPath())
	}

	log.Printf("[Node] Starting authorization with relay=%s", config.RelayAddress)
	log.Println("[Node] Connecting to relay...")

	// Create connection (no token means authorization mode)
	conn := node.NewConn(config)

	if err := conn.Connect(); err != nil {
		log.Fatalf("[Node] Failed to connect to relay: %v", err)
	}

	log.Println("[Node] Waiting for authorization...")
	log.Println("[Node] Visit the URL shown above to authorize this node.")

	// Wait for token to be received
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if conn.WaitForToken(ctx) {
		log.Println("[Node] Authorization successful! Token saved.")
		log.Println("[Node] You can now start the node normally.")
	} else {
		log.Println("[Node] Authorization timed out or was interrupted.")
	}

	conn.Close()
	os.Exit(0)
}

func handleLogoutCommand() {
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.Token == "" {
		log.Println("[Node] Not logged in.")
		return
	}

	// Clear token
	config.Token = ""

	// Save config using Config.Save() method
	if err := config.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	log.Println("[Node] Logged out successfully. The node will request authorization on next run.")
}

func handleUninstallCommand() {
	// Get binary path
	binaryPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get binary path: %v", err)
	}

	// Remove binary
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove binary %s: %v", binaryPath, err)
	}

	fmt.Println("Uninstall complete. Config file preserved at:", mustConfigPath())
}

func showConfig() {
	path := mustConfigPath()

	config, err := node.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println()
	fmt.Println("Config File:")
	fmt.Println("  Path:", path)
	fmt.Println()
	fmt.Println("Config:")
	fmt.Printf("  relay_address: %s\n", config.RelayAddress)
	fmt.Printf("  node_id: %s\n", config.NodeID)
	fmt.Printf("  token: %s\n", config.Token)
	fmt.Println()

	if config.Token != "" {
		fmt.Println("  Status: Authorized (ready to connect)")
	} else {
		fmt.Println("  Status: Not authorized (will request authorization on start)")
	}
}

func deleteConfig() {
	path := mustConfigPath()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Config file does not exist: %s\n", path)
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the config file at %s? (yes/no): ", path)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete the config file
	if err := os.Remove(path); err != nil {
		log.Fatalf("Failed to delete config file: %v", err)
	}

	fmt.Printf("Config file deleted: %s\n", path)
}

func printUsage() {
	fmt.Println("Usage: node [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  config show    Show the config file path and contents")
	fmt.Println("  config delete  Delete the config file")
	fmt.Println("  login       Show authorization instructions")
	fmt.Println("  logout      Remove saved credentials (token)")
	fmt.Println("  uninstall   Remove binary")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config <path>  Use custom config file (default: ~/.unblink/config.json)")
	fmt.Println("  -h, --help     Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  node                    # Start node (default config)")
	fmt.Println("  node config show          # Show config")
	fmt.Println("  node config delete         # Delete config (will regenerate with UUID)")
	fmt.Println("  node -config ./my-config  # Use custom config file")
	fmt.Println()
}
