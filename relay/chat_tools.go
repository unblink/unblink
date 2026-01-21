package relay

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/unblink/unblink/chat"
)

// SearchVideoTool implements the chat.Tool interface for searching videos
type SearchVideoTool struct {
	agentEventTable *table_agent_event
}

// NewSearchVideoTool creates a new video search tool
func NewSearchVideoTool(agentEventTable *table_agent_event) *SearchVideoTool {
	return &SearchVideoTool{agentEventTable: agentEventTable}
}

// Name returns the tool name
func (t *SearchVideoTool) Name() string {
	return "search_video"
}

// Description returns the tool description
func (t *SearchVideoTool) Description() string {
	return "Search for video recordings based on a query. Returns information about matching video segments including timestamps and duration."
}

// Parameters returns the JSON schema for the tool parameters
func (t *SearchVideoTool) Parameters() map[string]any {
	return map[string]any{
		"query": map[string]string{
			"type":        "string",
			"description": "Search query describing what to look for in videos (e.g., 'person at front door', 'motion detected', 'car in driveway')",
		},
	}
}

// searchVideoArgs represents the arguments for the search_video tool
type searchVideoArgs struct {
	Query string `json:"query"`
}

// Execute executes the tool with the given arguments
func (t *SearchVideoTool) Execute(ctx context.Context, argumentsJSON string) string {
	log.Printf("[SearchVideoTool] Executing with args: %s", argumentsJSON)

	// Parse arguments
	var args searchVideoArgs
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	if args.Query == "" {
		return "Error: query parameter is required"
	}

	const limit = 10
	log.Printf("[SearchVideoTool] query=%q, limit=%d", args.Query, limit)

	// Query the agent_events table for matching events
	// Use LIKE to search within the data JSON field
	query := `
		SELECT ae.id, ae.agent_id, ae.service_id, ae.data, ae.metadata, ae.created_at
		FROM agent_events ae
		JOIN agents a ON ae.agent_id = a.id
		WHERE LOWER(ae.data) LIKE ?
		ORDER BY ae.created_at DESC
		LIMIT ?
	`

	searchPattern := "%" + args.Query + "%"
	rows, err := t.agentEventTable.db.Query(query, searchPattern, limit)
	if err != nil {
		return fmt.Sprintf("Error searching videos: %v", err)
	}
	defer rows.Close()

	type eventResult struct {
		ID        string                 `json:"id"`
		AgentID   string                 `json:"agent_id"`
		ServiceID string                 `json:"service_id"`
		Data      map[string]interface{} `json:"data"`
		CreatedAt time.Time              `json:"created_at"`
	}

	var events []eventResult
	for rows.Next() {
		var e eventResult
		var dbServiceID sql.NullString
		var dataJSON, metadataJSON sql.NullString

		if err := rows.Scan(&e.ID, &e.AgentID, &dbServiceID, &dataJSON, &metadataJSON, &e.CreatedAt); err != nil {
			log.Printf("[SearchVideoTool] Error scanning row: %v", err)
			continue
		}

		if dbServiceID.Valid {
			e.ServiceID = dbServiceID.String
		}

		if dataJSON.Valid {
			json.Unmarshal([]byte(dataJSON.String), &e.Data)
		}

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return fmt.Sprintf("Error iterating results: %v", err)
	}

	// Format results
	if len(events) == 0 {
		return "No matching events found."
	}

	result := fmt.Sprintf("Found %d event(s):\n\n", len(events))
	for i, e := range events {
		result += fmt.Sprintf("%d. Event ID: %s\n", i+1, e.ID)
		result += fmt.Sprintf("   Camera: %s\n", e.ServiceID)
		result += fmt.Sprintf("   Time: %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"))

		// Include relevant data from the event
		if len(e.Data) > 0 {
			result += "   Details: "
			for k, v := range e.Data {
				result += fmt.Sprintf("%s: %v, ", k, v)
			}
			result = result[:len(result)-2] // Remove trailing comma
			result += "\n"
		}
		result += "\n"
	}

	return result
}

// RegisterChatTools registers all relay tools with the chat service
func (r *Relay) RegisterChatTools(chatService interface{ RegisterTool(chat.Tool) }) {
	chatService.RegisterTool(NewSearchVideoTool(r.AgentEventTable))
}
