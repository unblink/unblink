package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/unblink/unblink/chat"
	"github.com/unblink/unblink/relay"
)

func main() {
	// Load .env file (required)
	if err := godotenv.Load(); err != nil {
		log.Fatalf("[Main] Failed to load .env file: %v", err)
	}
	log.Println("[Main] Loaded .env file")

	// Parse subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "database":
			handleDatabaseCommand()
			return
		case "app-data":
			handleAppDataCommand()
			return
		case "storage":
			handleStorageCommand()
			return
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}

	// Load config from environment
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create database
	db, err := relay.NewDatabase(config.DatabasePath)
	if err != nil {
		log.Fatalf("[ERROR] Failed to open database: %v", err)
	}
	log.Printf("[INFO] Database opened: %s", config.DatabasePath)

	// Initialize tables
	nodeTable := relay.NewNodeTable(db.DB)
	serviceTable := relay.NewServiceTable(db)
	userTable := relay.NewUserTable(db.DB)
	agentTable := relay.NewAgentTable(db)
	agentEventTable := relay.NewAgentEventTable(db)
	storageTable := relay.NewStorageTable(db)
	jwtManager := relay.NewJWTManager(config.JWTSecret)

	// Create relay instance
	r := relay.NewRelay(config, nodeTable, serviceTable, userTable, agentTable, agentEventTable, storageTable, jwtManager)

	// Initialize storage manager for frame persistence (always needed for serving frames)
	storageBackend := relay.NewLocalStorage(config.StorageDir)
	storageManager := relay.NewStorageManager(storageBackend)
	r.SetStorageManager(storageManager)

	// Initialize Chat Service
	// openaiClient := shared.NewClient(config.ChatOpenAIBaseURL, 60 * 1000 * 1000 * 1000, config.ChatOpenAIModel, config.ChatOpenAIAPIKey) // 60s timeout
	log.Printf("[Chat] Initializing chat service:")
	log.Printf("[Chat]   Base URL: %s", config.ChatOpenAIBaseURL)
	log.Printf("[Chat]   API Key: %s", config.ChatOpenAIAPIKey)
	log.Printf("[Chat]   Model: %s", config.ChatOpenAIModel)
	openaiClient := openai.NewClient(
		option.WithAPIKey(config.ChatOpenAIAPIKey),
		option.WithBaseURL(config.ChatOpenAIBaseURL),
	)

	chatService, err := chat.NewService(config.AppDir, openaiClient, config.ChatOpenAIModel)

	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize chat service: %v", err)
	}
	r.SetChatService(chatService)

	// Initialize AutoStreamManager if auto-streaming is enabled
	if config.AutostreamEnable {
		log.Printf("[INFO] Initializing AutoStreamManager (frame_interval=%.1fs, batch_size=%d)",
			config.FrameIntervalSeconds, config.FrameBatchSize)

		autoStreamMgr := relay.NewAutoStreamManager(r, config, storageManager, storageTable)
		r.AutoStreamMgr = autoStreamMgr
		log.Printf("[INFO] AutoStreamManager initialized")
	} else {
		log.Printf("[INFO] Auto-streaming is disabled")
	}

	// Start servers
	relayAddr := ":" + config.RelayPort
	apiAddr := ":" + config.APIPort

	// Start WebSocket server for nodes
	log.Printf("[INFO] Starting WebSocket server on %s", relayAddr)
	go func() {
		if err := http.ListenAndServe(relayAddr, r.GetWebSocketHandler()); err != nil {
			log.Fatalf("[ERROR] Failed to start WebSocket server: %v", err)
		}
	}()

	// Start HTTP API server
	_, err = relay.StartHTTPAPIServer(r, apiAddr, config)
	if err != nil {
		log.Fatalf("[ERROR] Failed to start HTTP API server: %v", err)
	}
	log.Printf("[INFO] HTTP API server started on %s", apiAddr)

	// Print startup info
	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Unblink Relay v2.0")
	fmt.Println("===========================================")
	fmt.Printf("  WebSocket:  %s\n", relayAddr)
	fmt.Printf("  HTTP API:   %s\n", apiAddr)
	fmt.Printf("  Database:   %s\n", config.DatabasePath)
	fmt.Printf("  Dashboard:  %s\n", config.DashboardURL)
	fmt.Printf("  DevImpersonate: %s\n", config.DevImpersonate)

	fmt.Println("===========================================")
	fmt.Println("Press Ctrl+C to stop")

	// Wait forever (servers are already running in goroutines)
	select {}
}

func handleDatabaseCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: relay database <command>")
		fmt.Println("Commands:")
		fmt.Println("  delete  Delete the database directory")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "delete":
		deleteDatabase()
	default:
		fmt.Printf("Unknown database command: %s\n", os.Args[2])
		fmt.Println("Available commands: delete")
		os.Exit(1)
	}
}

func deleteDatabase() {
	// Load config to get database path
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Delete the database file
	dbPath := config.DatabasePath

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Printf("Database file does not exist: %s", dbPath)
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the database at %s? (yes/no): ", dbPath)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete the file
	if err := os.Remove(dbPath); err != nil {
		log.Fatalf("Failed to delete database: %v", err)
	}

	log.Printf("Database deleted: %s", dbPath)
}

func handleAppDataCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: relay app-data <command>")
		fmt.Println("Commands:")
		fmt.Println("  delete  Delete the entire app data directory")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "delete":
		deleteAppData()
	default:
		fmt.Printf("Unknown app-data command: %s\n", os.Args[2])
		fmt.Println("Available commands: delete")
		os.Exit(1)
	}
}

func deleteAppData() {
	// Load config to get app dir path
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Delete the app data directory
	appDir := config.AppDir

	// Check if directory exists
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		log.Printf("App data directory does not exist: %s", appDir)
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the entire app data directory at %s? (yes/no): ", appDir)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete the directory and all its contents
	if err := os.RemoveAll(appDir); err != nil {
		log.Fatalf("Failed to delete app data directory: %v", err)
	}

	log.Printf("App data directory deleted: %s", appDir)
}

func handleStorageCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: relay storage <command>")
		fmt.Println("Commands:")
		fmt.Println("  delete  Delete the storage directory")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "delete":
		deleteStorage()
	default:
		fmt.Printf("Unknown storage command: %s\n", os.Args[2])
		fmt.Println("Available commands: delete")
		os.Exit(1)
	}
}

func deleteStorage() {
	// Load config to get storage path
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Delete the storage directory
	storageDir := config.StorageDir

	// Check if directory exists
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		log.Printf("Storage directory does not exist: %s", storageDir)
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the storage directory at %s? (yes/no): ", storageDir)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete the directory and all its contents
	if err := os.RemoveAll(storageDir); err != nil {
		log.Fatalf("Failed to delete storage directory: %v", err)
	}

	log.Printf("Storage directory deleted: %s", storageDir)
}

func printUsage() {
	fmt.Println("Usage: relay [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  database delete  Delete the database file")
	fmt.Println("  app-data delete  Delete the entire app data directory")
	fmt.Println("  storage delete  Delete the storage directory")
	fmt.Println("  help, -h         Show this help message")
}
