# Node-Server Protocol (CBOR over WebSocket)

## Transport

- **Protocol:** WebSocket
- **Encoding:** CBOR (RFC 8949)
- **Endpoint:** `/node/connect`

## Message Header

```go
type Message struct {
    Type  string `json:"type"`
    ID    uint64 `json:"id"`
    Error string `json:"error,omitempty"`
}
```

## Message Types

### Token Messages

```go
// Node checks if existing token is valid
type TokenCheckRequest struct {
    Message
    NodeID string `json:"node_id"`
    Token  string `json:"token"`
}

type TokenCheckResponse struct {
    Message
    Valid bool `json:"valid"`
}

// Node requests new token (no token or invalid)
type NewTokenRequest struct {
    Message
    NodeID       string   `json:"node_id"`
    Hostname     string   `json:"hostname,omitempty"`
    MACAddresses []string `json:"mac_addresses,omitempty"`
}

type NewTokenResponse struct {
    Message
    Token string `json:"token"`
}
```

### Registration Messages

```go
type RegisterRequest struct {
    Message
    NodeID       string   `json:"node_id"`
    Token        string   `json:"token"`
    Hostname     string   `json:"hostname,omitempty"`
    MACAddresses []string `json:"mac_addresses,omitempty"`
}

type RegisterResponse struct {
    Message
    Success bool `json:"success"`
}

type NodeReadyMessage struct {
    Message
}
```

### Bridge Messages

```go
type OpenBridgeRequest struct {
    Message
    BridgeID  string `json:"bridge_id"`
    ServiceID string `json:"service_id"`
    Service   struct {
        ServiceURL string `json:"service_url"`
    } `json:"service"`
}

type OpenBridgeResponse struct {
    Message
    Success bool `json:"success"`
}

type CloseBridgeRequest struct {
    Message
    BridgeID string `json:"bridge_id"`
}

type CloseBridgeResponse struct {
    Message
    Success bool `json:"success"`
}

type BridgeDataMessage struct {
    Message
    BridgeID string `json:"bridge_id"`
    Sequence uint64 `json:"sequence"`
    Data     []byte `json:"data"`
}
```

## Node State Machine

```
DISCONNECTED
     │
     │ Connect()
     ▼
┌─────────────────────────────────┐
│ Has token in config?            │
├─────────────────────────────────┤
│ YES                          NO │
▼                                 ▼
CHECKING_TOKEN              REQUESTING_TOKEN
│ send: token_check_req     │ send: new_token_req
│                           │
▼                           ▼
token_check_response        new_token_response
│                           │ save token to config
├─valid──────┐              │
│            │              │
▼            ▼              │
REQUESTING   REGISTERING ◄──┘
_TOKEN       │ send: register_req
│            │
│            ▼
│            register_response
│            │
│            ├─success───► REGISTERED
│            │             │ send: node_ready
│            │             ▼
│            │             READY
│            │
└────────────┴─failure───► CLOSED
```

## Bridge State Machine

```
IDLE
  │ OpenBridgeRequest
  ▼
CONNECTING (TCP dial to service)
  │
  ├─success──► ACTIVE ◄──────┐
  │            │ send: OpenBridgeResponse(success)
  │            │              │
  │            ├─BridgeData───┤ (bidirectional)
  │            │              │
  │            │ CloseBridgeRequest
  │            ▼
  │            CLOSING
  │            │ send: CloseBridgeResponse
  │            ▼
  └─failure──► CLOSED
               send: OpenBridgeResponse(failure)
```
