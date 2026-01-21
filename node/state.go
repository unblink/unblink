package node

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/unblink/unblink/shared"
)

// ConnState interface defines all possible operations
// Each state implements this, returning errors for invalid operations
type ConnState interface {
	// Connection lifecycle
	Connect(*Conn) error

	// Message handlers
	OnRegisterResponse(*Conn, *shared.RegisterResponse) error
	OnAuthResponse(*Conn, *shared.RequestAuthResponse) error
	OnTokenReceived(*Conn, *shared.ReceiveTokenMessage) error

	// Bridge operations
	HandleOpenBridge(*Conn, *shared.OpenBridgeRequest) error
	HandleBridgeData(*Conn, *shared.BridgeDataMessage) error
	HandleCloseBridge(*Conn, *shared.CloseBridgeRequest) error

	// State name for logging
	Name() string
}

// DisconnectedState - initial state
type DisconnectedState struct{}

func (s *DisconnectedState) Name() string { return "DISCONNECTED" }

func (s *DisconnectedState) Connect(c *Conn) error {
	// Dial WebSocket
	wsConn, _, err := websocket.DefaultDialer.Dial(c.config.RelayAddress, nil)
	if err != nil {
		return err
	}

	c.wsConn = wsConn
	c.transport = &WebSocketConn{conn: wsConn}

	// Start message loop BEFORE transitioning state
	go c.messageLoop()

	// Perform handshake logic - transition based on token
	if c.config.Token == "" {
		// No token - request authorization
		c.setState(&RequestingAuthState{})
		return c.sendRequestAuth()
	}

	// Has token - register with relay
	c.setState(&RegisteringState{})
	return c.sendRegisterRequest()
}

func (s *DisconnectedState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	return fmt.Errorf("cannot handle register response in %s state", s.Name())
}
func (s *DisconnectedState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("cannot handle auth response in %s state", s.Name())
}
func (s *DisconnectedState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("cannot handle token in %s state", s.Name())
}
func (s *DisconnectedState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges in %s state", s.Name())
}
func (s *DisconnectedState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data in %s state", s.Name())
}
func (s *DisconnectedState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close in %s state", s.Name())
}

// RegisteringState - registering with saved token
// This state was entered when we sent a RegisterRequest - now we're waiting for the response
type RegisteringState struct{}

func (s *RegisteringState) Name() string { return "REGISTERING" }

func (s *RegisteringState) Connect(c *Conn) error {
	return fmt.Errorf("already connected")
}

func (s *RegisteringState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	if r.Success {
		c.setState(&RegisteredState{})
		// Send node_ready to signal we're ready for bridges
		msg := &shared.NodeReadyMessage{
			Message: shared.Message{
				Type: shared.MessageTypeNodeReady,
				ID:   c.getMsgID(),
			},
		}
		log.Printf("[State] Sending node_ready message")
		return c.transport.WriteMessage(msg)
	}

	// Failed - clear token and request auth
	log.Printf("Token invalid, requesting new auth")
	c.config.Token = ""
	if err := c.config.Save(); err != nil {
		return err
	}

	// Transition to RequestingAuth and send auth request
	c.setState(&RequestingAuthState{})
	return c.sendRequestAuth()
}

func (s *RegisteringState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("not expecting auth response while registering")
}
func (s *RegisteringState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("not expecting token while registering")
}
func (s *RegisteringState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges while registering")
}
func (s *RegisteringState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data while registering")
}
func (s *RegisteringState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close while registering")
}

// RequestingAuthState - requesting authorization URL
type RequestingAuthState struct{}

func (s *RequestingAuthState) Name() string { return "REQUESTING_AUTH" }

func (s *RequestingAuthState) Connect(c *Conn) error {
	return fmt.Errorf("already connected")
}

func (s *RequestingAuthState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	return fmt.Errorf("not expecting register response while requesting auth")
}

func (s *RequestingAuthState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	if !r.Success {
		return fmt.Errorf("authorization request failed: %s", r.Error)
	}

	if r.AuthURL != "" {
		// New authorization needed - display auth URL
		fmt.Printf("\nAuth URL: %s\n\n", r.AuthURL)
	} else {
		// Node already authorized - token will be sent via ReceiveTokenMessage
		log.Printf("[Conn] Node already authorized, waiting for token...")
	}

	c.setState(&AwaitingTokenState{})
	return nil
}

func (s *RequestingAuthState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("not expecting token while requesting auth")
}
func (s *RequestingAuthState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges while requesting auth")
}
func (s *RequestingAuthState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data while requesting auth")
}
func (s *RequestingAuthState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close while requesting auth")
}

// AwaitingTokenState - waiting for user to authorize
type AwaitingTokenState struct{}

func (s *AwaitingTokenState) Name() string { return "AWAITING_TOKEN" }

func (s *AwaitingTokenState) Connect(c *Conn) error {
	return fmt.Errorf("already connected")
}

func (s *AwaitingTokenState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	return fmt.Errorf("not expecting register response while awaiting token")
}
func (s *AwaitingTokenState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("not expecting auth response while awaiting token")
}

func (s *AwaitingTokenState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	// Save token
	c.config.Token = m.Token
	if err := c.config.Save(); err != nil {
		return err
	}

	// Notify token received (for WaitForToken() blocking call if used)
	select {
	case <-c.tokenReady:
		// Already closed
	default:
		close(c.tokenReady)
	}

	// Transition to RegisteringAfterAuth and send register request
	c.setState(&RegisteringAfterAuthState{})
	return c.sendRegisterRequest()
}

func (s *AwaitingTokenState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges while awaiting token")
}
func (s *AwaitingTokenState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data while awaiting token")
}
func (s *AwaitingTokenState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close while awaiting token")
}

// RegisteringAfterAuthState - registering after user authorized (failure is fatal)
type RegisteringAfterAuthState struct{}

func (s *RegisteringAfterAuthState) Name() string { return "REGISTERING_AFTER_AUTH" }

func (s *RegisteringAfterAuthState) Connect(c *Conn) error {
	return fmt.Errorf("already connected")
}

func (s *RegisteringAfterAuthState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	if r.Success {
		c.setState(&RegisteredState{})
		// Send node_ready to signal we're ready for bridges
		msg := &shared.NodeReadyMessage{
			Message: shared.Message{
				Type: shared.MessageTypeNodeReady,
				ID:   c.getMsgID(),
			},
		}
		log.Printf("[State] Sending node_ready message")
		return c.transport.WriteMessage(msg)
	}

	// Fatal error - user just authorized but still failed
	c.setState(&ClosedState{})
	return fmt.Errorf("registration failed after authorization: %s", r.Error)
}

func (s *RegisteringAfterAuthState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("not expecting auth response while registering after auth")
}
func (s *RegisteringAfterAuthState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("not expecting token while registering after auth")
}
func (s *RegisteringAfterAuthState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges while registering")
}
func (s *RegisteringAfterAuthState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data while registering")
}
func (s *RegisteringAfterAuthState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close while registering")
}

// RegisteredState - ready to handle bridges
type RegisteredState struct{}

func (s *RegisteredState) Name() string { return "REGISTERED" }

func (s *RegisteredState) Connect(c *Conn) error {
	return fmt.Errorf("already connected")
}

func (s *RegisteredState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	return fmt.Errorf("not expecting register response in registered state")
}
func (s *RegisteredState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("not expecting auth response in registered state")
}
func (s *RegisteredState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("not expecting token in registered state")
}

// Bridge operations are ONLY valid in registered state
func (s *RegisteredState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return c.handleOpenBridge(r)
}

func (s *RegisteredState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return c.handleBridgeData(m)
}

func (s *RegisteredState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return c.handleCloseBridge(r)
}

// ClosedState - terminal state
type ClosedState struct{}

func (s *ClosedState) Name() string { return "CLOSED" }

func (s *ClosedState) Connect(c *Conn) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) OnAuthResponse(c *Conn, r *shared.RequestAuthResponse) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) OnTokenReceived(c *Conn, m *shared.ReceiveTokenMessage) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return fmt.Errorf("connection closed")
}
func (s *ClosedState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return fmt.Errorf("connection closed")
}
