package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"connectrpc.com/connect"
	nodev1 "github.com/unblink/unblink/relay/gen/unblink/node/v1"
)

// NodeService implements the NodeServiceHandler interface
type NodeService struct {
	relay *Relay
}

func NewNodeService(relay *Relay) *NodeService {
	return &NodeService{relay: relay}
}

// ListNodes returns all nodes owned by the authenticated user
func (s *NodeService) ListNodes(
	ctx context.Context,
	req *connect.Request[nodev1.ListNodesRequest],
) (*connect.Response[nodev1.ListNodesResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	nodes, err := s.relay.NodeTable.GetNodesByUser(userID)
	if err != nil {
		log.Printf("[NodeService] Error getting nodes: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoNodes := make([]*nodev1.Node, len(nodes))
	for i, node := range nodes {
		status := "offline"
		if _, ok := s.relay.GetNodeConnection(node.ID); ok {
			status = "online"
		}

		var lastConnected int64
		if node.LastConnectedAt != nil {
			lastConnected = node.LastConnectedAt.Unix()
		}

		var macAddresses []string
		if node.MACAddresses != nil {
			json.Unmarshal([]byte(*node.MACAddresses), &macAddresses)
		}

		protoNodes[i] = &nodev1.Node{
			Id:              node.ID,
			Status:          status,
			Hostname:        toStringPtr(node.Hostname),
			MacAddresses:    macAddresses,
			LastConnectedAt: lastConnected,
		}
	}

	return connect.NewResponse(&nodev1.ListNodesResponse{
		Nodes: protoNodes,
	}), nil
}

// DeleteNode deletes a node
func (s *NodeService) DeleteNode(
	ctx context.Context,
	req *connect.Request[nodev1.DeleteNodeRequest],
) (*connect.Response[nodev1.DeleteNodeResponse], error) {
	nodeID := req.Msg.NodeId

	// Validate request
	if nodeID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("node_id is required"))
	}

	// Delete from database
	if err := s.relay.NodeTable.DeleteNode(nodeID); err != nil {
		log.Printf("[NodeService] Error deleting node: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Unregister connection if exists
	if conn, ok := s.relay.GetNodeConnection(nodeID); ok {
		conn.Close()
		s.relay.UnregisterNodeConnection(nodeID)
	}

	log.Printf("[NodeService] Deleted node %s", nodeID)

	return connect.NewResponse(&nodev1.DeleteNodeResponse{}), nil
}

func toStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
