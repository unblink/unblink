package relay

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// StorageHandler handles HTTP requests for storage items (frames and videos)
// GET /storage/{uuid}
type StorageHandler struct {
	relay *Relay
}

// ServeHTTP handles HTTP requests for storage items
func (h *StorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only GET method is allowed
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path (format: /storage/{uuid})
	path := r.URL.Path
	if !strings.HasPrefix(path, "/storage/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	id := strings.TrimPrefix(path, "/storage/")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("[StorageHandler] Request for storage %s from %s", id, r.RemoteAddr)

	// Get storage metadata from database
	storage, err := h.relay.StorageTable.GetStorage(id)
	if err != nil {
		log.Printf("[StorageHandler] Storage not found: %s, error: %v", id, err)
		http.Error(w, "Storage not found", http.StatusNotFound)
		return
	}

	// Retrieve data from storage backend
	data, err := h.relay.StorageManager.Retrieve(storage.StoragePath)
	if err != nil {
		log.Printf("[StorageHandler] Failed to retrieve storage %s: %v", id, err)
		http.Error(w, "Failed to retrieve storage", http.StatusInternalServerError)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", storage.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Handle range requests for videos
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" && storage.Type == "video" {
		h.serveRangeRequest(w, r, data, storage.ContentType)
		return
	}

	// Serve full file
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	if _, err := w.Write(data); err != nil {
		log.Printf("[StorageHandler] Failed to write storage %s: %v", id, err)
	}
}

// serveRangeRequest handles HTTP range requests for video seeking
func (h *StorageHandler) serveRangeRequest(w http.ResponseWriter, r *http.Request, data []byte, contentType string) {
	// Parse Range header (format: "bytes=start-end")
	rangeStr := r.Header.Get("Range")
	if !strings.HasPrefix(rangeStr, "bytes=") {
		http.Error(w, "Invalid range header", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeValue := strings.TrimPrefix(rangeStr, "bytes=")

	var start, end int64
	if strings.Contains(rangeValue, "-") {
		parts := strings.Split(rangeValue, "-")
		if parts[0] != "" {
			start, _ = strconv.ParseInt(parts[0], 10, 64)
		}
		if parts[1] != "" {
			end, _ = strconv.ParseInt(parts[1], 10, 64)
		} else {
			end = int64(len(data)) - 1
		}
	}

	// Validate range
	if start < 0 || end >= int64(len(data)) || start > end {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */ %d", len(data)))
		http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Set range headers
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusPartialContent)

	// Send partial content
	if _, err := w.Write(data[start : end+1]); err != nil {
		log.Printf("[StorageHandler] Failed to write range: %v", err)
	}
}

// NewStorageHandler creates a new StorageHandler
func NewStorageHandler(relay *Relay) *StorageHandler {
	return &StorageHandler{relay: relay}
}
