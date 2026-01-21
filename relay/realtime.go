package relay

import (
	"log"
	"sync"

	"connectrpc.com/connect"
	agentv1 "github.com/unblink/unblink/relay/gen/unblink/agent/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

// ClientRealtimeStreamManager manages active realtime event streams for clients
// A general-purpose manager that can handle multiple event types
type ClientRealtimeStreamManager struct {
	relay *Relay
	// Map of user_id to set of active server streams
	streams map[string]map[*connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse]]struct{}
	mu      sync.RWMutex
}

// NewClientRealtimeStreamManager creates a new client realtime stream manager
func NewClientRealtimeStreamManager(relay *Relay) *ClientRealtimeStreamManager {
	return &ClientRealtimeStreamManager{
		relay:   relay,
		streams: make(map[string]map[*connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse]]struct{}),
	}
}

// RegisterStream adds a new stream for a user
func (m *ClientRealtimeStreamManager) RegisterStream(userID string, stream *connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streams[userID] == nil {
		m.streams[userID] = make(map[*connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse]]struct{})
	}
	m.streams[userID][stream] = struct{}{}
	log.Printf("[ClientRealtime] Registered stream for user %s (total: %d)", userID, len(m.streams[userID]))
}

// UnregisterStream removes a stream
func (m *ClientRealtimeStreamManager) UnregisterStream(userID string, stream *connect.ServerStream[agentv1.StreamClientRealtimeEventsResponse]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streams[userID] != nil {
		delete(m.streams[userID], stream)
		log.Printf("[ClientRealtime] Unregistered stream for user %s (remaining: %d)", userID, len(m.streams[userID]))
	}
}

// BroadcastAgentEvent sends agent event to all streams for the agent owner
func (m *ClientRealtimeStreamManager) BroadcastAgentEvent(agentID string, serviceID string, data map[string]interface{}, metadata map[string]interface{}, createdAt int64) {
	// Get agent to find owner
	agent, err := m.relay.AgentTable.GetAgentByID(agentID)
	if err != nil {
		log.Printf("[ClientRealtime] Failed to get agent %s: %v", agentID, err)
		return
	}

	// Convert data and metadata to structpb.Struct
	dataStruct, err := mapToStruct(data)
	if err != nil {
		log.Printf("[ClientRealtime] Failed to convert data: %v", err)
		return
	}
	metadataStruct, err := mapToStruct(metadata)
	if err != nil {
		log.Printf("[ClientRealtime] Failed to convert metadata: %v", err)
		return
	}

	// Build event message with oneof wrapper
	event := &agentv1.ClientRealtimeEvent{
		Event: &agentv1.ClientRealtimeEvent_Agent{
			Agent: &agentv1.AgentEvent{
				Id:        uuid.New().String(),
				AgentId:   agentID,
				AgentName: agent.Name,
				ServiceId: serviceID,
				Data:      dataStruct,
				Metadata:  metadataStruct,
				CreatedAt: createdAt,
			},
		},
	}

	m.broadcastToUser(agent.UserID, event)
}

// broadcastToUser sends event to all streams for a user
func (m *ClientRealtimeStreamManager) broadcastToUser(userID string, event *agentv1.ClientRealtimeEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := m.streams[userID]
	count := 0
	for stream := range streams {
		if err := stream.Send(&agentv1.StreamClientRealtimeEventsResponse{Event: event}); err != nil {
			log.Printf("[ClientRealtime] Failed to send to stream: %v", err)
		} else {
			count++
		}
	}
	log.Printf("[ClientRealtime] Broadcast event to %d streams for user %s", count, userID)
}

// mapToStruct converts map[string]interface{} to *structpb.Struct
func mapToStruct(data map[string]interface{}) (*structpb.Struct, error) {
	if data == nil {
		return nil, nil
	}
	fields := make(map[string]interface{})
	for k, v := range data {
		fields[k] = v
	}
	return structpb.NewStruct(fields)
}
