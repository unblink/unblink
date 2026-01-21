package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	chatv1 "github.com/unblink/unblink/relay/gen/unblink/chat/v1"
	"github.com/unblink/unblink/relay/gen/unblink/chat/v1/chatv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	db     *DB
	openai *openai.Client
}

func NewService(unblinkDir string, openai *openai.Client) (*Service, error) {
	db, err := NewDB(unblinkDir)
	if err != nil {
		return nil, err
	}
	return &Service{
		db:     db,
		openai: openai,
	}, nil
}

// Ensure Service implements interface
var _ chatv1connect.ChatServiceHandler = (*Service)(nil)

func (s *Service) CreateConversation(ctx context.Context, req *connect.Request[chatv1.CreateConversationRequest]) (*connect.Response[chatv1.CreateConversationResponse], error) {
	id := uuid.New().String()
	now := time.Now()

	title := req.Msg.Title
	if title == "" {
		title = "New Conversation"
	}

	query := `INSERT INTO conversations (id, title, system_prompt, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, id, title, req.Msg.SystemPrompt, now, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create conversation: %w", err))
	}

	return connect.NewResponse(&chatv1.CreateConversationResponse{
		Conversation: &chatv1.Conversation{
			Id:           id,
			Title:        title,
			SystemPrompt: req.Msg.SystemPrompt,
			CreatedAt:    timestamppb.New(now),
			UpdatedAt:    timestamppb.New(now),
		},
	}), nil
}

func (s *Service) ListConversations(ctx context.Context, req *connect.Request[chatv1.ListConversationsRequest]) (*connect.Response[chatv1.ListConversationsResponse], error) {
	query := `SELECT id, title, system_prompt, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT 50`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list conversations: %w", err))
	}
	defer rows.Close()

	var conversations []*chatv1.Conversation
	for rows.Next() {
		var id, title, systemPrompt string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &title, &systemPrompt, &createdAt, &updatedAt); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		conversations = append(conversations, &chatv1.Conversation{
			Id:           id,
			Title:        title,
			SystemPrompt: systemPrompt,
			CreatedAt:    timestamppb.New(createdAt),
			UpdatedAt:    timestamppb.New(updatedAt),
		})
	}

	return connect.NewResponse(&chatv1.ListConversationsResponse{
		Conversations: conversations,
	}), nil
}

func (s *Service) GetConversation(ctx context.Context, req *connect.Request[chatv1.GetConversationRequest]) (*connect.Response[chatv1.GetConversationResponse], error) {
	query := `SELECT id, title, system_prompt, created_at, updated_at FROM conversations WHERE id = ?`
	row := s.db.QueryRow(query, req.Msg.ConversationId)

	var id, title, systemPrompt string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &title, &systemPrompt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&chatv1.GetConversationResponse{
		Conversation: &chatv1.Conversation{
			Id:           id,
			Title:        title,
			SystemPrompt: systemPrompt,
			CreatedAt:    timestamppb.New(createdAt),
			UpdatedAt:    timestamppb.New(updatedAt),
		},
	}), nil
}

func (s *Service) UpdateConversation(ctx context.Context, req *connect.Request[chatv1.UpdateConversationRequest]) (*connect.Response[chatv1.UpdateConversationResponse], error) {
	// Fetch existing
	query := `SELECT id, title, system_prompt, created_at, updated_at FROM conversations WHERE id = ?`
	row := s.db.QueryRow(query, req.Msg.ConversationId)

	var id, title, systemPrompt string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &title, &systemPrompt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("conversation not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if req.Msg.Title != nil {
		title = *req.Msg.Title
	}
	if req.Msg.SystemPrompt != nil {
		systemPrompt = *req.Msg.SystemPrompt
	}
	updatedAt = time.Now()

	updateQuery := `UPDATE conversations SET title = ?, system_prompt = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(updateQuery, title, systemPrompt, updatedAt, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update conversation: %w", err))
	}

	return connect.NewResponse(&chatv1.UpdateConversationResponse{
		Conversation: &chatv1.Conversation{
			Id:           id,
			Title:        title,
			SystemPrompt: systemPrompt,
			CreatedAt:    timestamppb.New(createdAt),
			UpdatedAt:    timestamppb.New(updatedAt),
		},
	}), nil
}

func (s *Service) DeleteConversation(ctx context.Context, req *connect.Request[chatv1.DeleteConversationRequest]) (*connect.Response[chatv1.DeleteConversationResponse], error) {
	_, err := s.db.Exec(`DELETE FROM conversations WHERE id = ?`, req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete conversation: %w", err))
	}
	return connect.NewResponse(&chatv1.DeleteConversationResponse{Success: true}), nil
}

func (s *Service) ListMessages(ctx context.Context, req *connect.Request[chatv1.ListMessagesRequest]) (*connect.Response[chatv1.ListMessagesResponse], error) {
	query := `SELECT id, conversation_id, body, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`
	rows, err := s.db.Query(query, req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list messages: %w", err))
	}
	defer rows.Close()

	var messages []*chatv1.Message
	for rows.Next() {
		var id, conversationID, body string
		var createdAt time.Time
		if err := rows.Scan(&id, &conversationID, &body, &createdAt); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		messages = append(messages, &chatv1.Message{
			Id:             id,
			ConversationId: conversationID,
			Body:           body,
			CreatedAt:      timestamppb.New(createdAt),
		})
	}

	return connect.NewResponse(&chatv1.ListMessagesResponse{
		Messages: messages,
	}), nil
}

func (s *Service) SendMessage(ctx context.Context, req *connect.Request[chatv1.SendMessageRequest], stream *connect.ServerStream[chatv1.SendMessageResponse]) error {
	conversationID := req.Msg.ConversationId
	content := req.Msg.Content

	// 1. Save User Message (as JSON body)
	userBody := map[string]interface{}{
		"role":    "user",
		"content": content,
	}
	userBodyJSON, _ := json.Marshal(userBody)
	userMsg := &chatv1.Message{
		Id:             uuid.New().String(),
		ConversationId: conversationID,
		Body:           string(userBodyJSON),
		CreatedAt:      timestamppb.New(time.Now()),
	}
	if err := s.saveMessage(userMsg); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save user message: %w", err))
	}

	// 2. Fetch History
	history, err := s.getConversationHistory(conversationID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get history: %w", err))
	}

	// 3. Define available tools
	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "search_video",
			Description: openai.String("Search for video frames"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type":        "string",
						"description": "Search query describing what to look for",
					},
				},
				"required": []string{"query"},
			},
		}),
	}

	// 4. Prepare OpenAI Request with tools
	openAIReq := openai.ChatCompletionNewParams{
		Messages: history,
		Tools:    tools,
	}

	// 5. Stream from OpenAI with tool call handling loop (max 5 passes)
	const maxPasses = 5

	// Add panic recovery to catch exact location
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ChatService] PANIC recovered: %v\n%s", r, debug.Stack())
		}
	}()

	for pass := 0; pass < maxPasses; pass++ {
		log.Printf("[ChatService] Pass %d/%d", pass+1, maxPasses)

		streamResp := s.openai.Chat.Completions.NewStreaming(ctx, openAIReq)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ChatService] PANIC in streamResp.Close(): %v\n%s", r, debug.Stack())
			}
			streamResp.Close()
		}()

		var fullContent string
		acc := openai.ChatCompletionAccumulator{}
		var toolCalls []openai.FinishedChatCompletionToolCall

		// Stream loop with detailed logging
		log.Printf("[ChatService] Starting stream loop...")
		iteration := 0
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ChatService] PANIC in stream loop: %v\n%s", r, debug.Stack())
				}
			}()

			for streamResp.Next() {
				iteration++
				chunk := streamResp.Current()

				// Simple logging without JSON marshaling
				toolCallCount := 0
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.ToolCalls != nil {
					toolCallCount = len(chunk.Choices[0].Delta.ToolCalls)
				}
				log.Printf("[ChatService] Chunk %d: choices=%d, tool_calls=%d, content_len=%d",
					iteration, len(chunk.Choices), toolCallCount, len(chunk.Choices[0].Delta.Content))

				acc.AddChunk(chunk)

				// Check for content deltas
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta.Content
					if delta != "" {
						fullContent += delta
						// Send to client
						if err := stream.Send(&chatv1.SendMessageResponse{
							Event: &chatv1.SendMessageResponse_TextDelta{
								TextDelta: delta,
							},
						}); err != nil {
							panic(fmt.Errorf("stream send error: %w", err))
						}
					}
				}

				// Check for finished tool calls
				if tool, ok := acc.JustFinishedToolCall(); ok {
					log.Printf("[ChatService] Tool call finished: %s (id: %s)", tool.Name, tool.ID)
					toolCalls = append(toolCalls, tool)
				}
			}
		}()
		log.Printf("[ChatService] Stream loop completed after %d iterations", iteration)

		// Check stream error with panic recovery (library bug?)
		log.Printf("[ChatService] About to check streamResp.Err()...")
		var streamErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ChatService] PANIC in streamResp.Err(): %v\n%s", r, debug.Stack())
					streamErr = fmt.Errorf("panic in stream error: %v", r)
				}
			}()
			streamErr = streamResp.Err()
		}()
		log.Printf("[ChatService] streamResp.Err() returned: %v", streamErr)
		if streamErr != nil {
			// Qwen model has a bug where it returns "list index out of range" when tool calls are incomplete
			// Check if we have a valid response in the accumulator despite the error
			if len(acc.Choices) > 0 {
				log.Printf("[ChatService] Stream error but accumulator has %d choices, continuing...", len(acc.Choices))
				// Clear the error and continue with what we have
				streamErr = nil
			} else {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("stream error: %w", streamErr))
			}
		}

		// Debug: log accumulator state
		accJSON, _ := json.MarshalIndent(acc, "", "  ")
		log.Printf("[ChatService] Full accumulator state:\n%s", string(accJSON))

		log.Printf("[ChatService] Stream ended, toolCalls count: %d", len(toolCalls))

		// If no tool calls, we're done - save the final message and exit
		if len(toolCalls) == 0 {
			log.Printf("[ChatService] No tool calls, saving final message...")
			if len(acc.Choices) == 0 {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("no choices in accumulator response"))
			}
			modelMsgJSON, _ := json.Marshal(acc.Choices[0].Message)
			modelMsg := &chatv1.Message{
				Id:             uuid.New().String(),
				ConversationId: conversationID,
				Body:           string(modelMsgJSON),
				CreatedAt:      timestamppb.New(time.Now()),
			}
			if err := s.saveMessage(modelMsg); err != nil {
				log.Printf("[ChatService] Failed to save model message: %v", err)
			}
			log.Printf("[ChatService] Completed with no tool calls on pass %d", pass+1)
			return nil
		}

		// Tool calls detected - save assistant message and execute tools
		log.Printf("[ChatService] Tool calls detected: %d", len(toolCalls))
		if len(acc.Choices) == 0 {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("no choices in accumulator response"))
		}

		// Save assistant message with tool calls
		log.Printf("[ChatService] Saving assistant message...")
		assistantMsgJSON, _ := json.Marshal(acc.Choices[0].Message)
		assistantMsg := &chatv1.Message{
			Id:             uuid.New().String(),
			ConversationId: conversationID,
			Body:           string(assistantMsgJSON),
			CreatedAt:      timestamppb.New(time.Now()),
		}
		if err := s.saveMessage(assistantMsg); err != nil {
			log.Printf("[ChatService] Failed to save assistant message: %v", err)
		}

		// Execute each tool call and save tool messages
		for i, toolCall := range toolCalls {
			log.Printf("[ChatService] Executing tool %d/%d: %s", i+1, len(toolCalls), toolCall.Name)
			// Send tool invoked event
			if err := stream.Send(&chatv1.SendMessageResponse{
				Event: &chatv1.SendMessageResponse_ToolCall{
					ToolCall: &chatv1.ToolCallEvent{
						ToolName:  toolCall.Name,
						Arguments: toolCall.Arguments,
						State:     "invoked",
					},
				},
			}); err != nil {
				return err
			}

			result := s.executeTool(ctx, toolCall.Name, toolCall.Arguments)
			log.Printf("[ChatService] Tool result: %s", result)

			// Send tool result event
			if err := stream.Send(&chatv1.SendMessageResponse{
				Event: &chatv1.SendMessageResponse_ToolCall{
					ToolCall: &chatv1.ToolCallEvent{
						ToolName: toolCall.Name,
						Result:   result,
						State:    "completed",
					},
				},
			}); err != nil {
				return err
			}

			// Save tool message to database
			log.Printf("[ChatService] Saving tool message...")
			toolBodyJSON, _ := json.Marshal(map[string]interface{}{
				"role":         "tool",
				"tool_call_id": toolCall.ID,
				"content":      result,
			})
			toolMsg := &chatv1.Message{
				Id:             uuid.New().String(),
				ConversationId: conversationID,
				Body:           string(toolBodyJSON),
				CreatedAt:      timestamppb.New(time.Now()),
			}
			if err := s.saveMessage(toolMsg); err != nil {
				log.Printf("[ChatService] Failed to save tool message: %v", err)
			}
		}

		// Continue conversation with tool results - fetch updated history
		log.Printf("[ChatService] Fetching updated history...")
		history, err = s.getConversationHistory(conversationID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated history: %w", err))
		}

		// Prepare next API call with tool results
		log.Printf("[ChatService] Preparing next API call...")
		openAIReq = openai.ChatCompletionNewParams{
			Messages: history,
			Tools:    tools,
		}

		log.Printf("[ChatService] Completed pass %d with %d tool calls, continuing...", pass+1, len(toolCalls))
	}

	// Max passes reached - should not normally get here if model stops calling tools
	log.Printf("[ChatService] Reached max passes (%d)", maxPasses)
	return nil
}

func (s *Service) saveMessage(msg *chatv1.Message) error {
	query := `INSERT INTO messages (id, conversation_id, body, created_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, msg.Id, msg.ConversationId, msg.Body, msg.CreatedAt.AsTime())
	return err
}

func (s *Service) getConversationHistory(conversationID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	// First get system prompt
	var systemPrompt string
	err := s.db.QueryRow("SELECT system_prompt FROM conversations WHERE id = ?", conversationID).Scan(&systemPrompt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var messages []openai.ChatCompletionMessageParamUnion
	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}

	// Get messages
	rows, err := s.db.Query("SELECT body FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}

		// For tool messages, we need tool_call_id which is not in ChatCompletionMessage
		// So parse into a map first to check the role
		var rawMsg map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &rawMsg); err != nil {
			log.Printf("[ChatService] Failed to unmarshal message body: %v", err)
			continue
		}

		// Get role from raw message
		var role string
		json.Unmarshal(rawMsg["role"], &role)

		switch role {
		case "user":
			var msg openai.ChatCompletionMessage
			json.Unmarshal([]byte(body), &msg)
			messages = append(messages, openai.UserMessage(msg.Content))
		case "assistant", "model":
			var msg openai.ChatCompletionMessage
			json.Unmarshal([]byte(body), &msg)
			messages = append(messages, openai.AssistantMessage(msg.Content))
		case "tool":
			// Tool message needs special handling for tool_call_id
			var content string
			var toolCallID string
			json.Unmarshal(rawMsg["content"], &content)
			json.Unmarshal(rawMsg["tool_call_id"], &toolCallID)
			messages = append(messages, openai.ToolMessage(content, toolCallID))
		case "system":
			var msg openai.ChatCompletionMessage
			json.Unmarshal([]byte(body), &msg)
			messages = append(messages, openai.SystemMessage(msg.Content))
		}
	}
	return messages, nil
}

// executeTool executes a tool call and returns the result
func (s *Service) executeTool(ctx context.Context, toolName string, argumentsJSON string) string {
	fmt.Printf("[ChatService] Executing tool: %s with args: %s", toolName, argumentsJSON)
	switch toolName {
	case "search_video":
		// Simple example - just return fixed text
		// In a real implementation, this would parse arguments and search video
		return "There was a man"
	default:
		return fmt.Sprintf("Unknown tool: %s", toolName)
	}
}
