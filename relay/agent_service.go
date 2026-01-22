package relay

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	agentv1 "github.com/unblink/unblink/relay/gen/unblink/agent/v1"
)

// AgentService implements the AgentServiceHandler interface
type AgentService struct {
	relay *Relay
}

func NewAgentService(relay *Relay) *AgentService {
	return &AgentService{relay: relay}
}

// ListAgents returns all agents owned by the authenticated user
func (s *AgentService) ListAgents(
	ctx context.Context,
	req *connect.Request[agentv1.ListAgentsRequest],
) (*connect.Response[agentv1.ListAgentsResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	agents, err := s.relay.AgentTable.GetAgentsByUser(userID)
	if err != nil {
		log.Printf("[AgentService] Failed to list agents: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoAgents := make([]*agentv1.Agent, len(agents))
	for i, agent := range agents {
		protoAgents[i] = toProtoAgent(agent)
	}

	return connect.NewResponse(&agentv1.ListAgentsResponse{
		Agents: protoAgents,
	}), nil
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(
	ctx context.Context,
	req *connect.Request[agentv1.CreateAgentRequest],
) (*connect.Response[agentv1.CreateAgentResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	// Validation
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("agent name is required"))
	}
	if req.Msg.Config == nil || req.Msg.Config.Instruction == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("agent instruction is required"))
	}
	if len(req.Msg.Name) > 255 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name too long (max 255 characters)"))
	}
	if len(req.Msg.Config.Instruction) > 5000 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("instruction too long (max 5000 characters)"))
	}

	// Default type if not provided
	agentType := req.Msg.Type
	if agentType == "" {
		agentType = "openai_compat"
	}

	// Convert proto config to internal config
	config := AgentConfig{
		Instruction: req.Msg.Config.Instruction,
		BaseUrl:     req.Msg.Config.BaseUrl,
		Model:       req.Msg.Config.Model,
		APIKey:      req.Msg.Config.ApiKey,
	}

	// Create agent
	agent, err := s.relay.AgentTable.CreateAgent(CreateAgentRequest{
		Name:       req.Msg.Name,
		Type:       agentType,
		Config:     config,
		ServiceIDs: req.Msg.ServiceIds,
	}, userID)
	if err != nil {
		log.Printf("[AgentService] Failed to create agent: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[AgentService] Agent %s created by user %s (type=%s)", agent.ID, userID, agent.Type)

	// Notify AutoStreamManager
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnAgentCreated(agent)
	}

	return connect.NewResponse(&agentv1.CreateAgentResponse{
		Success: true,
		Id:      agent.ID,
		Agent:   toProtoAgent(agent),
	}), nil
}

// GetAgent returns a specific agent
func (s *AgentService) GetAgent(
	ctx context.Context,
	req *connect.Request[agentv1.GetAgentRequest],
) (*connect.Response[agentv1.GetAgentResponse], error) {
	agentID := req.Msg.AgentId

	// Validate request
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("agent_id is required"))
	}

	agent, err := s.relay.AgentTable.GetAgentByID(agentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("agent not found"))
	}

	return connect.NewResponse(&agentv1.GetAgentResponse{
		Agent: toProtoAgent(agent),
	}), nil
}

// UpdateAgent updates an agent
func (s *AgentService) UpdateAgent(
	ctx context.Context,
	req *connect.Request[agentv1.UpdateAgentRequest],
) (*connect.Response[agentv1.UpdateAgentResponse], error) {
	agentID := req.Msg.AgentId

	// Validate request
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("agent_id is required"))
	}

	// Build update parameters - only set non-empty values
	var name *string
	if req.Msg.Name != "" {
		name = &req.Msg.Name
	}

	var agentType *string
	if req.Msg.Type != "" {
		agentType = &req.Msg.Type
	}

	var config *AgentConfig
	if req.Msg.Config != nil {
		config = &AgentConfig{
			Instruction: req.Msg.Config.Instruction,
			BaseUrl:     req.Msg.Config.BaseUrl,
			Model:       req.Msg.Config.Model,
			APIKey:      req.Msg.Config.ApiKey,
		}
	}

	var serviceIDs *[]string
	if req.Msg.ServiceIds != nil {
		serviceIDs = &req.Msg.ServiceIds
	}

	// Validate fields if provided
	if name != nil && len(*name) > 255 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name too long (max 255 characters)"))
	}
	if config != nil && len(config.Instruction) > 5000 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("instruction too long (max 5000 characters)"))
	}

	// Update agent
	agent, err := s.relay.AgentTable.UpdateAgent(agentID, name, agentType, config, serviceIDs)
	if err != nil {
		log.Printf("[AgentService] Failed to update agent: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[AgentService] Updated agent %s", agentID)

	// Notify AutoStreamManager
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnAgentUpdated(agent)
	}

	return connect.NewResponse(&agentv1.UpdateAgentResponse{
		Agent: toProtoAgent(agent),
	}), nil
}

// DeleteAgent deletes an agent
func (s *AgentService) DeleteAgent(
	ctx context.Context,
	req *connect.Request[agentv1.DeleteAgentRequest],
) (*connect.Response[agentv1.DeleteAgentResponse], error) {
	agentID := req.Msg.AgentId

	// Validate request
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("agent_id is required"))
	}

	if err := s.relay.AgentTable.DeleteAgent(agentID); err != nil {
		log.Printf("[AgentService] Failed to delete agent: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[AgentService] Deleted agent %s", agentID)

	// Notify AutoStreamManager
	if s.relay.AutoStreamMgr != nil {
		go s.relay.AutoStreamMgr.OnAgentDeleted(agentID)
	}

	return connect.NewResponse(&agentv1.DeleteAgentResponse{}), nil
}

// StreamClientRealtimeEvents is a server-streaming RPC for all client realtime events
func (s *AgentService) StreamClientRealtimeEvents(
	ctx context.Context,
	req *connect.Request[agentv1.StreamClientRealtimeEventsRequest],
	stream *connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse],
) error {
	// Get user from auth context
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	log.Printf("[AgentService] StreamClientRealtimeEvents started for user %s", userID)

	// Register stream with realtime manager
	s.relay.ClientRealtimeMgr.RegisterStream(userID, stream)
	defer s.relay.ClientRealtimeMgr.UnregisterStream(userID, stream)

	// Keep stream open and wait for context cancellation
	<-ctx.Done()
	log.Printf("[AgentService] StreamClientRealtimeEvents ended for user %s: %v", userID, ctx.Err())
	return nil
}

// ListAgentEvents returns historical agent events (unary RPC)
func (s *AgentService) ListAgentEvents(
	ctx context.Context,
	req *connect.Request[agentv1.ListAgentEventsRequest],
) (*connect.Response[agentv1.ListAgentEventsResponse], error) {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Query events from database
	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 100 // default limit
	}

	events, err := s.relay.AgentEventTable.ListEvents(userID, req.Msg.AgentId, req.Msg.ServiceId, limit)
	if err != nil {
		log.Printf("[AgentService] Failed to list events: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build a map of agentID -> agentName for efficient lookup
	agentNames := make(map[string]string)
	for _, e := range events {
		if _, exists := agentNames[e.AgentID]; !exists {
			if agent, err := s.relay.AgentTable.GetAgentByID(e.AgentID); err == nil {
				agentNames[e.AgentID] = agent.Name
			}
		}
	}

	protoEvents := make([]*agentv1.AgentEvent, len(events))
	for i, e := range events {
		agentName := agentNames[e.AgentID]
		protoEvents[i] = toProtoAgentEvent(e, agentName)
	}

	return connect.NewResponse(&agentv1.ListAgentEventsResponse{
		Events: protoEvents,
	}), nil
}

func toProtoAgent(agent *Agent) *agentv1.Agent {
	return &agentv1.Agent{
		Id:   agent.ID,
		Name: agent.Name,
		Type: agent.Type,
		Config: &agentv1.AgentConfig{
			Instruction: agent.Config.Instruction,
			BaseUrl:     agent.Config.BaseUrl,
			Model:       agent.Config.Model,
			ApiKey:      agent.Config.APIKey,
		},
		ServiceIds: agent.ServiceIDs,
		CreatedAt:  agent.CreatedAt.Unix(),
		UpdatedAt:  agent.UpdatedAt.Unix(),
	}
}

func toProtoAgentEvent(event *AgentEventDB, agentName string) *agentv1.AgentEvent {
	// Convert data and metadata to structpb.Struct
	dataStruct, _ := mapToStruct(event.Data)
	metadataStruct, _ := mapToStruct(event.Metadata)

	return &agentv1.AgentEvent{
		Id:        event.ID,
		AgentId:   event.AgentID,
		AgentName: agentName,
		ServiceId: event.ServiceID,
		Data:      dataStruct,
		Metadata:  metadataStruct,
		CreatedAt: event.CreatedAt.Unix(),
	}
}
