package relay

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	authv1 "github.com/unblink/unblink/relay/gen/unblink/auth/v1"
	"github.com/unblink/unblink/shared"
)

// AuthService implements the AuthServiceHandler interface
type AuthService struct {
	relay      *Relay
	jwtManager *JWTManager
	userTable  *table_user
}

func NewAuthService(relay *Relay) *AuthService {
	return &AuthService{
		relay:      relay,
		jwtManager: relay.JWTManager,
		userTable:  relay.UserTable,
	}
}

// Register handles user registration
func (s *AuthService) Register(
	ctx context.Context,
	req *connect.Request[authv1.RegisterRequest],
) (*connect.Response[authv1.RegisterResponse], error) {
	log.Printf("[AuthService] Register request for %s", req.Msg.Email)

	// Validate request
	if req.Msg.Email == "" || req.Msg.Password == "" || req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("missing required fields (email, password, name)"))
	}

	// Validate password length
	if len(req.Msg.Password) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("password must be at least 8 characters"))
	}

	// Create user
	user, err := s.userTable.CreateUser(req.Msg.Email, req.Msg.Password, req.Msg.Name)
	if err != nil {
		if err.Error() == "email already registered" {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("email already registered"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[AuthService] Created new user %s", user.Email)

	return connect.NewResponse(&authv1.RegisterResponse{
		Success: true,
		Token:   token,
		User: &authv1.User{
			Id:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Unix(),
		},
	}), nil
}

// Login handles user login
func (s *AuthService) Login(
	ctx context.Context,
	req *connect.Request[authv1.LoginRequest],
) (*connect.Response[authv1.LoginResponse], error) {
	log.Printf("[AuthService] Login request for %s", req.Msg.Email)

	// Validate request
	if req.Msg.Email == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("missing required fields (email, password)"))
	}

	// Validate password
	user, err := s.userTable.ValidatePassword(req.Msg.Email, req.Msg.Password)
	if err != nil {
		log.Printf("[AuthService] Login failed for %s: %v", req.Msg.Email, err)
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("invalid email or password"))
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[AuthService] User %s logged in", user.Email)

	return connect.NewResponse(&authv1.LoginResponse{
		Success: true,
		Token:   token,
		User: &authv1.User{
			Id:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Unix(),
		},
	}), nil
}

// GetUser returns the authenticated user's info
func (s *AuthService) GetUser(
	ctx context.Context,
	req *connect.Request[authv1.GetUserRequest],
) (*connect.Response[authv1.GetUserResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	user, err := s.userTable.GetUserByID(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&authv1.GetUserResponse{
		Success: true,
		User: &authv1.User{
			Id:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Unix(),
		},
	}), nil
}

// AuthorizeNode authorizes a node for the authenticated user
func (s *AuthService) AuthorizeNode(
	ctx context.Context,
	req *connect.Request[authv1.AuthorizeNodeRequest],
) (*connect.Response[authv1.AuthorizeNodeResponse], error) {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("not authenticated"))
	}

	nodeID := req.Msg.NodeId

	// Validate request
	if nodeID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("node_id is required"))
	}

	// Check if node is connected
	nodeConn, ok := s.relay.GetNodeConnection(nodeID)
	if !ok {
		log.Printf("[AuthService] Node %s not connected, cannot authorize", nodeID)
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("node not connected. Make sure the node is running and connected to the relay"))
	}

	// Generate a secure token for the node
	token, err := generateSecureToken()
	if err != nil {
		log.Printf("[AuthService] Failed to generate token: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Authorize the node in the database
	err = s.relay.NodeTable.AuthorizeNode(nodeID, userID, "", token)
	if err != nil {
		log.Printf("[AuthService] Failed to authorize node: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Push token to the connected node (if transport is available)
	if nodeConn.transport != nil {
		// Send receive_token message to the node
		msg := &shared.ReceiveTokenMessage{
			Message: shared.Message{
				Type: shared.MessageTypeReceiveToken,
				ID:   nodeConn.getMsgID(),
			},
			Token: token,
		}

		err = nodeConn.transport.WriteMessage(msg)
		if err != nil {
			log.Printf("[AuthService] Failed to push token to node: %v", err)
			return nil, connect.NewError(connect.CodeInternal,
				fmt.Errorf("failed to send authorization to node. Please try again"))
		}
		log.Printf("[AuthService] Successfully pushed token to node %s", nodeID)
	} else {
		log.Printf("[AuthService] Node %s authorized, token stored (node will receive on reconnect)", nodeID)
	}

	log.Printf("[AuthService] Node %s authorized by user %s", nodeID, userID)

	return connect.NewResponse(&authv1.AuthorizeNodeResponse{
		Success: true,
		NodeId:  nodeID,
		Message: "Node authorized successfully",
	}), nil
}
