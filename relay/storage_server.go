package relay

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// StorageHandler handles HTTP requests for storage items (frames and videos)
// GET /storage/{uuid} or /storage/{uuid}/segment_XXX.ts
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

	// Extract ID from path (format: /storage/{uuid} or /storage/{uuid}/segment_XXX.ts)
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/storage/")
	if trimmed == "" || trimmed == path {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Check if it's a segment request: /storage/{uuid}/segment_XXX.ts
	parts := strings.SplitN(trimmed, "/", 2)
	id := parts[0]
	segmentPath := ""
	if len(parts) > 1 {
		segmentPath = parts[1]
	}

	log.Printf("[StorageHandler] Request for storage %s (segment: %q) from %s", id, segmentPath, r.RemoteAddr)

	// Get storage metadata from database
	storage, err := h.relay.StorageTable.GetStorage(id)
	if err != nil {
		log.Printf("[StorageHandler] Storage not found: %s, error: %v", id, err)
		http.Error(w, "Storage not found", http.StatusNotFound)
		return
	}

	// Check if this is an HLS recording
	if storage.Metadata != nil {
		if format, ok := storage.Metadata["format"].(string); ok && format == "hls" {
			h.serveHLS(w, r, storage, segmentPath)
			return
		}
	}

	// For non-HLS storage, retrieve from storage backend
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

// serveHLS serves HLS playlist and segment files
func (h *StorageHandler) serveHLS(w http.ResponseWriter, r *http.Request, storage *StorageDB, segmentPath string) {
	// Get the base directory from local storage backend
	localStorage, ok := h.relay.StorageManager.backend.(*LocalStorage)
	if !ok {
		http.Error(w, "HLS storage not available", http.StatusInternalServerError)
		return
	}

	// Parse storage path to get HLS directory
	// storagePath format: "local://hls/{playlistID}/stream.m3u8"
	// Normalize legacy paths (add local:// prefix if missing)
	storagePath := normalizePath(storage.StoragePath)
	filePath, err := parsePath(storagePath)
	if err != nil {
		log.Printf("[StorageHandler] Invalid storage path: %s, error: %v", storage.StoragePath, err)
		http.Error(w, "Invalid storage path", http.StatusInternalServerError)
		return
	}

	// Get directory containing stream.m3u8 and segments
	hlsDir := filepath.Join(localStorage.baseDir, filepath.Dir(filePath))

	// Determine what file to serve
	var serveFilePath string
	var contentType string

	if segmentPath != "" {
		// Serve segment file: /storage/{uuid}/segment_XXX.ts
		serveFilePath = filepath.Join(hlsDir, segmentPath)
		contentType = "video/mp2t" // MPEG-TS
	} else {
		// Serve playlist: /storage/{uuid}
		serveFilePath = filepath.Join(hlsDir, "stream.m3u8")
		contentType = "application/vnd.apple.mpegurl"
	}

	// Check if file exists
	if _, err := os.Stat(serveFilePath); err != nil {
		log.Printf("[StorageHandler] HLS file not found: %s, error: %v", serveFilePath, err)
		http.Error(w, "HLS file not found", http.StatusNotFound)
		return
	}

	// Set CORS headers for video playback
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Enable range requests for seeking
	http.ServeFile(w, r, serveFilePath)
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
