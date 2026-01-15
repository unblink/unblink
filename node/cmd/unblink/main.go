package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/unblink/unblink/node"
)

//go:embed default_config.hujson
var defaultConfig []byte

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "Path to config file")
}

func main() {
	// Check for help
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	// Handle subcommands before parsing flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			handleConfigCommand()
			return
		case "login":
			doLogin()
			return
		case "logout":
			logout()
			return
		case "uninstall":
			uninstall()
			return
		case "service":
			handleServiceCommand()
			return
		}
	}

	// Parse flags for main node execution
	flag.Parse()

	// Load config (LoadConfig creates it with generated node ID if missing)
	var config *node.Config
	var err error
	var actualConfigPath string
	if configPath != "" {
		config, err = node.LoadConfigFromFile(configPath, defaultConfig)
		actualConfigPath = configPath
	} else {
		config, err = node.LoadConfigWithDefault(defaultConfig)
		actualConfigPath = mustConfigPath()
	}
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate relay_address is set
	if config.RelayAddr() == "" {
		log.Fatalf("[Node] relay_address is not set in config. Please edit %s and add \"relay_address\": \"<your-relay-address>\"", actualConfigPath)
	}

	// Log node ID
	log.Printf("[Node] Node ID: %s", config.NodeID())

	if len(config.Services()) == 0 {
		log.Fatalf("[Node] No services configured. Edit %s to add services.", actualConfigPath)
	}

	// Run node (will authorize if needed, then continue running)
	runNode(config)
}

func doLogin() {
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate relay_address is set
	if config.RelayAddr() == "" {
		log.Fatalf("[Node] relay_address is not set in config. Please edit %s and add \"relay_address\": \"<your-relay-address>\"", mustConfigPath())
	}

	if len(config.Services()) == 0 {
		log.Fatalf("[Node] No services configured. Edit %s to add services.", mustConfigPath())
	}

	if config.Token() != "" {
		log.Println("[Node] Already logged in. Use 'unblink logout' first if you want to re-authorize.")
		return
	}

	log.Printf("[Node] Starting authorization with relay=%s, services=%d", config.RelayAddr(), len(config.Services()))

	runNode(config)
}

func runNode(config *node.Config) {
	if config.Token() == "" {
		log.Printf("[Node] Not authorized. Starting authorization flow...")
	} else {
		log.Printf("[Node] Starting with relay=%s, services=%d", config.RelayAddr(), len(config.Services()))
		log.Printf("[Node] Using saved credentials for node: %s", config.NodeID())
	}

	reconnectCfg := config.Reconnect()
	if reconnectCfg.Enabled {
		log.Printf("[Node] Auto-reconnect enabled (max_num_attempts=%d)", reconnectCfg.MaxNumAttempts)
	} else {
		log.Printf("[Node] Auto-reconnect is DISABLED. Add 'reconnect: { max_num_attempts: N }' to config to enable.")
	}

	// Client factory function for reconnector
	var currentClient *node.NodeClient
	createClient := func() *node.NodeClient {
		client := node.NewNodeClient(config.RelayAddr(), config.Services(), config.NodeID(), config.Token())

		// Handle connection ready
		client.OnConnectionReady = func(nodeID, dashboardURL string) {
			if dashboardURL != "" {
				log.Println("========================================")
				log.Printf("AUTHORIZATION REQUIRED")
				log.Printf("Open this URL in your browser:")
				log.Printf("%s", dashboardURL)
				log.Println("========================================")
			} else {
				log.Printf("[Node] Connected and authorized: %s", nodeID)
			}
		}

		currentClient = client
		return client
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Use reconnector if auto-reconnect is enabled
	if reconnectCfg.Enabled {
		reconnector := node.NewReconnector(config)
		// Single signal handler for both client and reconnector cleanup
		go func() {
			<-sigChan
			log.Println("[Node] Shutting down...")
			if currentClient != nil {
				currentClient.Close()
			}
			reconnector.Close()
		}()
		reconnector.Run(createClient)
	} else {
		// Signal handler for non-reconnect mode
		go func() {
			<-sigChan
			log.Println("[Node] Shutting down...")
			if currentClient != nil {
				currentClient.Close()
			}
		}()
		client := createClient()
		if err := client.Run(config); err != nil {
			log.Fatalf("%v", err)
		}
	}
}

func handleConfigCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: unblink config <command>")
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

func showConfig() {
	path, err := node.ConfigPath()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// LoadConfig creates the file if it doesn't exist
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Services()) == 0 {
		fmt.Println("Config file created with default services template.")
		fmt.Println("Edit the file to add your services:")
		fmt.Println(path)
	} else {
		fmt.Println(path)
	}
}

func deleteConfig() {
	path, err := node.ConfigPath()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Config file does not exist: %s", path)
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

	log.Printf("Config file deleted: %s", path)
}

func handleServiceCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: unblink service <command>")
		fmt.Println("Commands:")
		fmt.Println("  add     Add a new service (interactive)")
		fmt.Println("  delete  Delete a service")
		fmt.Println("  list    List all services")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "add":
		serviceAdd()
	case "delete":
		serviceDelete()
	case "list":
		serviceList()
	default:
		fmt.Printf("Unknown service command: %s\n", os.Args[2])
		fmt.Println("Available commands: add, delete, list")
		os.Exit(1)
	}
}

func serviceAdd() {
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Adding a new service")
	fmt.Println("---------------------")

	// Type
	fmt.Print("Type (rtsp or mjpeg): ")
	serviceType, _ := reader.ReadString('\n')
	serviceType = strings.TrimSpace(serviceType)

	// Addr
	fmt.Print("Address (required, e.g., 192.168.1.100 or mycam.example.com): ")
	addr, _ := reader.ReadString('\n')
	addr = strings.TrimSpace(addr)
	if addr == "" {
		log.Fatal("Address is required")
	}

	// Port
	fmt.Print("Port (required, e.g., 554 for RTSP, 80 for HTTP): ")
	portStr, _ := reader.ReadString('\n')
	portStr = strings.TrimSpace(portStr)
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		log.Fatal("Invalid port number")
	}

	// Path
	fmt.Print("Path (optional, e.g., /cam/stream): ")
	path, _ := reader.ReadString('\n')
	path = strings.TrimSpace(path)

	// Auth
	fmt.Print("Add authentication? (yes/no): ")
	authResponse, _ := reader.ReadString('\n')
	authResponse = strings.TrimSpace(strings.ToLower(authResponse))

	var auth *node.Auth
	if authResponse == "yes" || authResponse == "y" {
		fmt.Print("Username: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)

		fmt.Print("Password: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		auth = &node.Auth{
			Type:     node.AuthTypeUsernamePassword,
			Username: username,
			Password: password,
		}
	}

	// Name
	fmt.Print("Name (optional): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	// Create service
	service := node.Service{
		ID:   "", // Will be generated by ensureConfigIDs
		Name: name,
		Type: serviceType,
		Addr: addr,
		Port: port,
		Path: path,
		Auth: auth,
	}

	if err := config.AddService(service); err != nil {
		log.Fatalf("Failed to add service: %v", err)
	}

	if err := config.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Println("\nService added successfully!")
	fmt.Printf("Name: %s\n", service.Name)
	fmt.Printf("Address: %s:%d%s\n", service.Addr, service.Port, service.Path)
}

func serviceDelete() {
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Services()) == 0 {
		fmt.Println("No services configured.")
		return
	}

	// List services
	fmt.Println("Services:")
	for i, svc := range config.Services() {
		name := svc.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  %d. %s - %s:%d%s (ID: %s)\n", i+1, name, svc.Addr, svc.Port, svc.Path, svc.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nEnter service number to delete: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(config.Services()) {
		log.Fatal("Invalid service number")
	}

	services := config.Services()
	svc := services[index-1]
	name := svc.Name
	if name == "" {
		name = "(unnamed)"
	}

	fmt.Printf("Delete service '%s'? (yes/no): ", name)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" && confirm != "y" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Remove service
	if err := config.DeleteService(index - 1); err != nil {
		log.Fatalf("Failed to delete service: %v", err)
	}

	if err := config.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Println("Service deleted successfully!")
}

func serviceList() {
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Services()) == 0 {
		fmt.Println("No services configured.")
		return
	}

	fmt.Println("Services:")
	for i, svc := range config.Services() {
		name := svc.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  %d. %s\n", i+1, name)
		fmt.Printf("     Type: %s\n", svc.Type)
		fmt.Printf("     Address: %s:%d%s\n", svc.Addr, svc.Port, svc.Path)
		fmt.Printf("     ID: %s\n", svc.ID)
		if svc.Auth != nil {
			fmt.Printf("     Auth: %s\n", svc.Auth.Type)
		}
	}
}

func logout() {
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.Token() == "" {
		log.Println("[Node] Not logged in.")
		return
	}

	// Clear both token and node ID so re-authorization gets a fresh identity
	config.SetToken("")
	config.SetNodeID("")
	if err := config.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	log.Println("[Node] Logged out successfully. Run 'unblink' to re-authorize.")
}

func mustConfigPath() string {
	path, err := node.ConfigPath()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return path
}

func uninstall() {
	// Get binary path
	binaryPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get binary path: %v", err)
	}

	// Remove binary
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove binary %s: %v", binaryPath, err)
	}

	log.Println("Uninstall complete. Config file preserved.")
}

func printUsage() {
	fmt.Println("Usage: unblink [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  config show    Show the config file path")
	fmt.Println("  config delete  Delete the config file")
	fmt.Println("  service add    Add a new service (interactive)")
	fmt.Println("  service delete Delete a service")
	fmt.Println("  service list   List all services")
	fmt.Println("  login          Authorize with the relay server")
	fmt.Println("  logout         Remove saved credentials")
	fmt.Println("  uninstall      Remove binary")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help  Show this help message")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Relay address is configured in the config file (relay_address)")
}
