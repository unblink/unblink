package relay

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleFlags returns feature flags and configuration for the frontend
func handleFlags(w http.ResponseWriter, r *http.Request, cfg *Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] /flags request from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"DEV_IMPERSONATE": cfg.DevImpersonate,
	})
}
