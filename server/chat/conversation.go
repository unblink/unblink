package chat

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"unblink/server/internal/ctxutil"
	chatv1 "unblink/server/gen/chat/v1"
)

// verifyConversationOwnership checks if the user owns the conversation
func (s *Service) verifyConversationOwnership(ctx context.Context, conversationID string) error {
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	ownerID, err := s.db.GetConversationOwner(conversationID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to verify ownership: %w", err))
	}

	if ownerID == "" {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation not found"))
	}

	if ownerID != userID {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you don't own this conversation"))
	}

	return nil
}

func (s *Service) CreateConversation(ctx context.Context, req *connect.Request[chatv1.CreateConversationRequest]) (*connect.Response[chatv1.CreateConversationResponse], error) {
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	id := generateID()
	now := time.Now()

	title := req.Msg.Title
	if title == "" {
		title = "New Chat"
	}

	err := s.db.CreateConversation(id, userID, title, req.Msg.SystemPrompt)
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

func (s *Service) GetConversation(ctx context.Context, req *connect.Request[chatv1.GetConversationRequest]) (*connect.Response[chatv1.GetConversationResponse], error) {
	if err := s.verifyConversationOwnership(ctx, req.Msg.ConversationId); err != nil {
		return nil, err
	}

	conversation, err := s.db.GetConversation(req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get conversation: %w", err))
	}

	if conversation == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation not found"))
	}

	return connect.NewResponse(&chatv1.GetConversationResponse{
		Conversation: conversation,
	}), nil
}

func (s *Service) UpdateConversation(ctx context.Context, req *connect.Request[chatv1.UpdateConversationRequest]) (*connect.Response[chatv1.UpdateConversationResponse], error) {
	if err := s.verifyConversationOwnership(ctx, req.Msg.ConversationId); err != nil {
		return nil, err
	}

	var title, systemPrompt string
	if req.Msg.Title != nil {
		title = *req.Msg.Title
	}
	if req.Msg.SystemPrompt != nil {
		systemPrompt = *req.Msg.SystemPrompt
	}

	if err := s.db.UpdateConversation(req.Msg.ConversationId, title, systemPrompt); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	conversation, err := s.db.GetConversation(req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&chatv1.UpdateConversationResponse{
		Conversation: conversation,
	}), nil
}

func (s *Service) DeleteConversation(ctx context.Context, req *connect.Request[chatv1.DeleteConversationRequest]) (*connect.Response[chatv1.DeleteConversationResponse], error) {
	if err := s.verifyConversationOwnership(ctx, req.Msg.ConversationId); err != nil {
		return nil, err
	}

	if err := s.db.DeleteConversation(req.Msg.ConversationId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&chatv1.DeleteConversationResponse{
		Success: true,
	}), nil
}

func (s *Service) ListConversations(ctx context.Context, req *connect.Request[chatv1.ListConversationsRequest]) (*connect.Response[chatv1.ListConversationsResponse], error) {
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	conversations, err := s.db.ListConversations(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list conversations: %w", err))
	}

	return connect.NewResponse(&chatv1.ListConversationsResponse{
		Conversations: conversations,
	}), nil
}
