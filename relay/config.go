package relay

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all relay configuration loaded from environment
type Config struct {
	// Base directory
	AppDir       string
	StorageDir   string // Computed: AppDir + "/storage"
	DatabasePath string // Computed: AppDir + "/database/relay.db"

	// Ports
	RelayPort string // Port for WebSocket connections (nodes) (e.g., "9020")
	APIPort   string // Port for HTTP API for browsers (e.g., "8020")

	// Dashboard
	DashboardURL string

	// Security
	JWTSecret string // Secret for signing JWT tokens

	// Dev Mode
	DevImpersonate string // "", "true", or specific email like "dev@local"

	// Auto-streaming configuration (defaults for vision agents)
	AutostreamEnable     bool    // Enable automatic frame streaming (default: true)
	FrameIntervalSeconds float64 // Frame extraction interval in seconds (default: 5.0)
	FrameBatchSize       int     // Number of frames to batch before sending (default: 10)
	AutoStreamTimeout    int     // OpenAI request timeout in seconds (default: 30)
	AutoStreamModel      string  // Default OpenAI model for AutoStreamManager (default: "Qwen/Qwen3-VL-4B-Instruct")
	AutoStreamAPIKey     string  // Default API key for AutoStreamManager
	AutoStreamBaseURL    string  // Default base URL for AutoStreamManager

	// Video recording configuration
	VideoRecordingEnabled       bool    // Enable continuous video recording (default: false)
	VideoSegmentDurationMinutes float64 // Duration of each video segment in minutes (default: 5.0)

	// Chat-RPC configuration
	ChatOpenAIModel   string // Model to use for Chat RPC
	ChatOpenAIAPIKey  string // OpenAI API key for chat completions
	ChatOpenAIBaseURL string // OpenAI-compatible API base URL (e.g., https://api.openai.com/v1 or https://openrouter.ai/api/v1)
}

// LoadConfig loads and validates all configuration from environment
func LoadConfig() (*Config, error) {
	var errors []string

	// Get relay port (default: 9020)
	relayPort := os.Getenv("RELAY_PORT")
	if relayPort == "" {
		relayPort = "9020"
	}

	// Get API port (default: 8020)
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "8020"
	}

	// Get app directory (default: ./data)
	appDir := os.Getenv("APP_DIR")
	if appDir == "" {
		appDir = "./data"
	}

	// Dashboard URL (default: http://localhost:5173)
	dashboardURL := os.Getenv("DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = "http://localhost:5173"
	}

	// Dev impersonation mode (optional, default: disabled)
	devImpersonate := os.Getenv("DEV_IMPERSONATE_EMAIL")

	// JWT secret for token signing (required)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		errors = append(errors, "JWT_SECRET is required in .env file")
	}

	// Auto-streaming configuration
	autostreamEnable := true
	if val := os.Getenv("AUTOSTREAM_ENABLED"); val != "" {
		autostreamEnable = val == "true" || val == "1"
	}

	frameIntervalSeconds := 5.0
	if val := os.Getenv("FRAME_INTERVAL_SECONDS"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			frameIntervalSeconds = parsed
		}
	}

	frameBatchSize := 2
	if val := os.Getenv("FRAME_BATCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			frameBatchSize = parsed
		}
	}

	// AutoStreamManager defaults
	autoStreamTimeout := 30
	if val := os.Getenv("AUTOSTREAM_OPENAI_TIMEOUT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			autoStreamTimeout = parsed
		}
	}

	autoStreamModel := "Qwen/Qwen3-VL-4B-Instruct"
	if val := os.Getenv("AUTOSTREAM_OPENAI_MODEL"); val != "" {
		autoStreamModel = val
	}

	autoStreamAPIKey := os.Getenv("AUTOSTREAM_OPENAI_API_KEY")
	autoStreamBaseURL := os.Getenv("AUTOSTREAM_OPENAI_BASE_URL")

	// Video recording configuration
	videoRecordingEnabled := false
	if val := os.Getenv("VIDEO_RECORDING_ENABLED"); val != "" {
		videoRecordingEnabled = val == "true" || val == "1"
	}

	videoSegmentDurationMinutes := 5.0
	if val := os.Getenv("VIDEO_SEGMENT_DURATION_MINUTES"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			videoSegmentDurationMinutes = parsed
		}
	}

	// Chat-RPC API keys (optional)
	chatOpenAIAPIKey := os.Getenv("CHAT_OPENAI_API_KEY")
	chatOpenAIBaseURL := os.Getenv("CHAT_OPENAI_BASE_URL") // Default: https://api.openai.com/v1
	chatOpenAIModel := os.Getenv("CHAT_OPENAI_MODEL")

	// Validate port number
	if _, err := strconv.Atoi(relayPort); err != nil {
		errors = append(errors, fmt.Sprintf("RELAY_PORT must be a number, got: %s", relayPort))
	}

	if _, err := strconv.Atoi(apiPort); err != nil {
		errors = append(errors, fmt.Sprintf("API_PORT must be a number, got: %s", apiPort))
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("configuration validation errors:\n%v", errors)
	}

	// Compute paths from APP_DIR
	storageDir := filepath.Join(appDir, "storage")
	databasePath := filepath.Join(appDir, "database", "relay.db")

	config := &Config{
		AppDir:                     appDir,
		StorageDir:                 storageDir,
		DatabasePath:               databasePath,
		RelayPort:                  relayPort,
		APIPort:                    apiPort,
		DashboardURL:               dashboardURL,
		JWTSecret:                  jwtSecret,
		DevImpersonate:             devImpersonate,
		AutostreamEnable:           autostreamEnable,
		FrameIntervalSeconds:       frameIntervalSeconds,
		FrameBatchSize:             frameBatchSize,
		AutoStreamTimeout:          autoStreamTimeout,
		AutoStreamModel:            autoStreamModel,
		AutoStreamAPIKey:           autoStreamAPIKey,
		AutoStreamBaseURL:          autoStreamBaseURL,
		VideoRecordingEnabled:      videoRecordingEnabled,
		VideoSegmentDurationMinutes: videoSegmentDurationMinutes,
		ChatOpenAIModel:            chatOpenAIModel,
		ChatOpenAIAPIKey:           chatOpenAIAPIKey,
		ChatOpenAIBaseURL:          chatOpenAIBaseURL,
	}

	// Log loaded configuration
	log.Printf("[Config] Loaded configuration:")
	log.Printf("[Config]   APP_DIR: %s", config.AppDir)
	log.Printf("[Config]   STORAGE_DIR: %s", config.StorageDir)
	log.Printf("[Config]   DATABASE_PATH: %s", config.DatabasePath)
	log.Printf("[Config]   RELAY_PORT: %s (nodes)", config.RelayPort)
	log.Printf("[Config]   API_PORT: %s (HTTP API)", config.APIPort)
	log.Printf("[Config]   DASHBOARD_URL: %s", config.DashboardURL)
	if config.DevImpersonate != "" {
		log.Printf("[Config]   DEV_IMPERSONATE_EMAIL: %s", config.DevImpersonate)
	}
	log.Printf("[Config]   AUTOSTREAM_ENABLED: %v", config.AutostreamEnable)
	log.Printf("[Config]   FRAME_INTERVAL_SECONDS: %.1f", config.FrameIntervalSeconds)
	log.Printf("[Config]   FRAME_BATCH_SIZE: %d", config.FrameBatchSize)
	log.Printf("[Config]   AUTOSTREAM_OPENAI_TIMEOUT: %ds", config.AutoStreamTimeout)
	log.Printf("[Config]   AUTOSTREAM_OPENAI_MODEL: %s", config.AutoStreamModel)
	log.Printf("[Config]   VIDEO_RECORDING_ENABLED: %v", config.VideoRecordingEnabled)
	log.Printf("[Config]   VIDEO_SEGMENT_DURATION_MINUTES: %.1f", config.VideoSegmentDurationMinutes)

	return config, nil
}
