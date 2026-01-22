package relay

import (
	"log"
	"net/http"

	"connectrpc.com/connect"
	elizav1connect "github.com/unblink/unblink/relay/gen/connectrpc/eliza/v1/elizav1connect"
	agentv1connect "github.com/unblink/unblink/relay/gen/unblink/agent/v1/agentv1connect"
	authv1connect "github.com/unblink/unblink/relay/gen/unblink/auth/v1/authv1connect"
	chatv1connect "github.com/unblink/unblink/relay/gen/unblink/chat/v1/chatv1connect"
	configv1connect "github.com/unblink/unblink/relay/gen/unblink/config/v1/configv1connect"
	nodev1connect "github.com/unblink/unblink/relay/gen/unblink/node/v1/nodev1connect"
	servicev1connect "github.com/unblink/unblink/relay/gen/unblink/service/v1/servicev1connect"
	storagev1connect "github.com/unblink/unblink/relay/gen/unblink/storage/v1/storagev1connect"
	webrtcv1connect "github.com/unblink/unblink/relay/gen/unblink/webrtc/v1/webrtcv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// StartHTTPAPIServer starts the HTTP API server
func StartHTTPAPIServer(relay *Relay, addr string, cfg *Config) (*http.Server, error) {
	mux := http.NewServeMux()

	// Create global auth interceptor for all RPC calls
	authInterceptor := NewAuthInterceptor(relay.JWTManager)
	interceptors := connect.WithInterceptors(
		authInterceptor,
	)

	// Mount Connect RPC handlers with auth interceptor
	elizaPath, elizaHandler := elizav1connect.NewElizaServiceHandler(
		&ElizaServer{},
		interceptors,
	)
	mux.Handle(elizaPath, elizaHandler)
	log.Printf("[HTTP] Mounted ElizaService at %s", elizaPath)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		&AuthService{relay: relay, jwtManager: relay.JWTManager, userTable: relay.UserTable},
		interceptors,
	)
	mux.Handle(authPath, authHandler)
	log.Printf("[HTTP] Mounted AuthService at %s", authPath)

	// Mount NodeService
	nodePath, nodeHandler := nodev1connect.NewNodeServiceHandler(
		NewNodeService(relay),
		interceptors,
	)
	mux.Handle(nodePath, nodeHandler)
	log.Printf("[HTTP] Mounted NodeService at %s", nodePath)

	// Mount ServiceService
	servicePath, serviceHandler := servicev1connect.NewServiceServiceHandler(
		NewServiceService(relay),
		interceptors,
	)
	mux.Handle(servicePath, serviceHandler)
	log.Printf("[HTTP] Mounted ServiceService at %s", servicePath)

	// Mount AgentService
	agentPath, agentHandler := agentv1connect.NewAgentServiceHandler(
		NewAgentService(relay),
		interceptors,
	)
	mux.Handle(agentPath, agentHandler)
	log.Printf("[HTTP] Mounted AgentService at %s", agentPath)

	// Mount WebRTCService
	webrtcPath, webrtcHandler := webrtcv1connect.NewWebRTCServiceHandler(
		NewWebRTCService(relay),
		interceptors,
	)
	mux.Handle(webrtcPath, webrtcHandler)
	log.Printf("[HTTP] Mounted WebRTCService at %s", webrtcPath)

	// Mount ConfigService (no auth required)
	configPath, configHandler := configv1connect.NewConfigServiceHandler(
		NewConfigService(relay, cfg),
	)
	mux.Handle(configPath, configHandler)
	log.Printf("[HTTP] Mounted ConfigService at %s", configPath)

	// Mount ChatService
	if relay.ChatService != nil {
		chatPath, chatHandler := chatv1connect.NewChatServiceHandler(
			relay.ChatService,
			interceptors,
		)
		mux.Handle(chatPath, chatHandler)
		log.Printf("[HTTP] Mounted ChatService at %s", chatPath)
	}

	// Mount StorageService
	storagePath, storageServiceHandler := storagev1connect.NewStorageServiceHandler(
		NewStorageServiceServer(relay),
		interceptors,
	)
	mux.Handle(storagePath, storageServiceHandler)
	log.Printf("[HTTP] Mounted StorageService at %s", storagePath)

	// Mount StorageHandler (HTTP handler for serving frames and videos)
	storageHandler := NewStorageHandler(relay)
	mux.Handle("/storage/", storageHandler)
	log.Printf("[HTTP] Mounted StorageHandler at /storage/")

	// Wrap with CORS middleware (applies to non-RPC routes)
	handler := corsMiddleware(mux)

	// Initialize WebRTC session manager
	relay.WebRTCSessionMgr = NewWebRTCSessionManager(relay)

	// Node connection endpoint (WebSocket) - stays as WebSocket
	mux.HandleFunc("/node/connect", func(w http.ResponseWriter, r *http.Request) {
		handleNodeConnect(w, r, relay)
	})

	// Use h2c to support HTTP/2 without TLS for Connect RPC
	// This allows both gRPC (HTTP/2 required) and Connect (HTTP/1.1 or HTTP/2) to work
	h2cHandler := h2c.NewHandler(handler, &http2.Server{})

	server := &http.Server{
		Addr:    addr,
		Handler: h2cHandler,
	}

	log.Printf("[HTTP] Starting HTTP API on %s (with h2c for HTTP/2 support)", addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	return server, nil
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version, Connect-Timeout-Ms, Connect-Accept-Encoding, Connect-Content-Encoding, X-User-Agent, X-Grpc-Web")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version, Connect-Timeout-Ms, Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleNodeConnect handles incoming WebSocket connections from nodes
func handleNodeConnect(w http.ResponseWriter, r *http.Request, relay *Relay) {
	wsConn, err := UpgradeHTTPToWebSocket(w, r)
	if err != nil {
		log.Printf("[HTTP] WebSocket upgrade failed: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}

	log.Printf("[HTTP] Node connected from %s", r.RemoteAddr)

	// Create node connection handler
	nodeConn := NewNodeConn(wsConn, relay)

	// Start handling the connection (non-blocking)
	go func() {
		if err := nodeConn.Run(); err != nil {
			log.Printf("[HTTP] Node connection error: %v", err)
		}
	}()
}
