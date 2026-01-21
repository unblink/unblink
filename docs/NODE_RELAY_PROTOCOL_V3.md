# Node-Relay Protocol v3 (CBOR)

This document describes the Unblink v3 protocol between nodes and the relay using **CBOR** encoding over WebSocket.

## Transport Layer

| Property       | Value                          |
| -------------- | ------------------------------ |
| Protocol       | WebSocket                      |
| Encoding       | CBOR (RFC 8949)                |
| Message Format | Binary WebSocket frames        |
| Endpoints      | `/node/connect` (Node → Relay) |

## Message Types

All messages share a common header:

```go
type Message struct {
    Type     string `json:"type"`       // Message type identifier
    ID       uint64 `json:"id"`         // Unique message ID for request/response matching
    Error    string `json:"error"`      // Error message if request failed
}
```

### Message Types

```go
// Registration messages
const (
    MessageTypeRegisterRequest     = "register_request"
    MessageTypeRegisterResponse    = "register_response"
    MessageTypeRequestAuthRequest  = "request_auth_request"
    MessageTypeRequestAuthResponse = "request_auth_response"
    MessageTypeReceiveToken        = "receive_token"
    MessageTypeNodeReady           = "node_ready"

    // Bridge messages
    MessageTypeOpenBridgeRequest   = "open_bridge_request"
    MessageTypeOpenBridgeResponse  = "open_bridge_response"
    MessageTypeCloseBridgeRequest  = "close_bridge_request"
    MessageTypeCloseBridgeResponse = "close_bridge_response"

    // Data messages (streaming)
    MessageTypeBridgeData          = "bridge_data"
    MessageTypeBridgeDataAck       = "bridge_data_ack"
)
```

## Message Definitions

### 1. Register Request (Node → Relay)

```go
type RegisterRequest struct {
    Message
    NodeID       string   `json:"node_id"`
    Token        string   `json:"token"`
    Hostname     string   `json:"hostname,omitempty"`
    MACAddresses []string `json:"mac_addresses,omitempty"`
}
```

**Response:**

```go
type RegisterResponse struct {
    Message
    Success bool `json:"success"`
}
```

### 2. Request Authorization (Node → Relay)

```go
type RequestAuthRequest struct {
    Message
    NodeID string `json:"node_id"`
}
```

**Response:**

```go
type RequestAuthResponse struct {
    Message
    Success bool   `json:"success"`
    AuthURL string `json:"auth_url"`
}
```

### 3. Receive Token (Relay → Node) - Push message

```go
type ReceiveTokenMessage struct {
    Message
    Token string `json:"token"`
}
```

### 4. Node Ready (Node → Relay)

Sent by the node after successfully registering to signal it's ready to handle bridges.

```go
type NodeReadyMessage struct {
    Message
}
```

### 5. Open Bridge (Relay → Node)

```go
type OpenBridgeRequest struct {
    Message
    BridgeID string  `json:"bridge_id"`
    ServiceID string `json:"service_id"`
    Service  Service `json:"service"`
}
```

**Service struct:**

```go
type Service struct {
    ServiceURL string `json:"service_url"`
}
```

**Response:**

```go
type OpenBridgeResponse struct {
    Message
    Success bool `json:"success"`
}
```

### 6. Close Bridge (Relay → Node)

```go
type CloseBridgeRequest struct {
    Message
    BridgeID string `json:"bridge_id"`
}
```

**Response:**

```go
type CloseBridgeResponse struct {
    Message
    Success bool `json:"success"`
}
```

### 7. Bridge Data (Bidirectional)

```go
type BridgeDataMessage struct {
    Message
    BridgeID string `json:"bridge_id"`
    Sequence uint64 `json:"sequence"`
    Data     []byte `json:"data"`
}
```

## State Machine Diagrams

### Node Connection State Machine

```
                          ┌─────────────┐
                          │ DISCONNECTED│
                          └──────┬──────┘
                                 │ Connect()
                                 │ - Dial WebSocket
                                 │ - Start message loop
                                 ▼
               ┌─────────────────┴─────────────────┐
               │ no token                    has token
               ▼                                   ▼
    ┌───────────────────┐              ┌─────────────────┐
    │ REQUESTING_AUTH   │              │   REGISTERING   │
    │                   │              │                 │
    │ Send:             │              │ Send:           │
    │ RequestAuthReq    │              │ RegisterRequest │
    └─────────┬─────────┘              └────────┬────────┘
              │                                  │
              │ RequestAuthResponse              │ RegisterResponse
              ▼                          ┌───────┴────────┐
    ┌───────────────────┐                │                │
    │ AWAITING_TOKEN    │           Success=false  Success=true
    └─────────┬─────────┘                │                │
              │ ReceiveToken              │                │
              │ - Save token              │                │
              ▼                           │                │
    ┌────────────────────────┐     Clear token            │
    │REGISTERING_AFTER_AUTH  │            │                │
    │                        │            │                │
    │ Send:                  │            │                │
    │ RegisterRequest        │            │                │
    └─────────┬──────────────┘            │                │
              │                           │                │
              │ RegisterResponse          ▼                │
       ┌──────┴───────┐         ┌─────────────────┐       │
       │              │         │ REQUESTING_AUTH │       │
  Success=false  Success=true   └─────────────────┘       │
       │              │                                    │
   Fatal Error        └────────────────────────────────────┴───→ REGISTERED
       │                                                         └─────┬─────┘
       ▼                                                               │
  ┌─────────┐                                                   Idle, waiting
  │ CLOSED  │◄──────────────────────────────────────────────    for bridges
  └─────────┘              Any connection error
```

**State Transitions:**

| From State             | Event                     | To State               | Action                                                      |
| ---------------------- | ------------------------- | ---------------------- | ----------------------------------------------------------- |
| DISCONNECTED           | Connect() (no token)      | REQUESTING_AUTH        | Dial WebSocket, start message loop, send RequestAuthRequest |
| DISCONNECTED           | Connect() (has token)     | REGISTERING            | Dial WebSocket, start message loop, send RegisterRequest    |
| REQUESTING_AUTH        | RequestAuthResponse       | AWAITING_TOKEN         | Display auth URL to user (or log if already authorized)     |
| AWAITING_TOKEN         | ReceiveToken              | REGISTERING_AFTER_AUTH | Save token, send RegisterRequest                            |
| REGISTERING            | RegisterResponse(success) | REGISTERED             | Node is ready                                               |
| REGISTERING            | RegisterResponse(fail)    | REQUESTING_AUTH        | Clear token, retry auth                                     |
| REGISTERING_AFTER_AUTH | RegisterResponse(success) | REGISTERED             | Node is ready                                               |
| REGISTERING_AFTER_AUTH | RegisterResponse(fail)    | CLOSED                 | Fatal error, exit                                           |
| Any state              | Connection error          | CLOSED                 | Cleanup and exit                                            |

### Bridge State Machine (per bridge instance)

```
            ┌────────────┐
            │   IDLE     │ (no bridge exists)
            └─────┬──────┘
                  │ OpenBridgeRequest received
                  │ - Parse service URL
                  ▼
            ┌────────────┐
            │ CONNECTING │
            │            │
            │ TCP dial   │
            │ to service │
            └─────┬──────┘
                  │
         ┌────────┴────────┐
    fail │                 │ success
         │                 │
         ▼                 ▼
  ┌────────────┐     ┌────────────┐
  │  FAILED    │     │   ACTIVE   │◄──────────┐
  │            │     │            │           │
  │ Send:      │     │ Send:      │           │
  │ OpenBridge │     │ OpenBridge │           │
  │ Response   │     │ Response   │           │
  │ (fail)     │     │ (success)  │           │
  └────────────┘     └─────┬──────┘           │
                           │                  │
                ┌──────────┼──────────┐       │
                ▼          ▼          ▼       │
           BridgeData  BridgeData  (idle)     │
           Relay→Node  Node→Relay             │
                │          │                  │
                │ Write to │ Read from        │
                │ TCP conn │ TCP conn         │
                │          │                  │
                └──────────┴──────────────────┘
                           │
                           │ CloseBridgeRequest received
                           ▼
                    ┌────────────┐
                    │  CLOSING   │
                    │            │
                    │ - Close TCP│
                    │ - Cleanup  │
                    └─────┬──────┘
                          │ Send CloseBridgeResponse
                          ▼
                    ┌────────────┐
                    │   CLOSED   │
                    │            │
                    │ Removed    │
                    │ from map   │
                    └────────────┘
```

**Bridge State Transitions:**

| From State | Event                   | To State   | Action                                            |
| ---------- | ----------------------- | ---------- | ------------------------------------------------- |
| IDLE       | OpenBridgeRequest       | CONNECTING | Parse service URL, dial TCP                       |
| CONNECTING | TCP dial success        | ACTIVE     | Send OpenBridgeResponse(success), start forwarder |
| CONNECTING | TCP dial fail           | FAILED     | Send OpenBridgeResponse(fail)                     |
| ACTIVE     | BridgeData (from relay) | ACTIVE     | Write data to TCP connection                      |
| ACTIVE     | TCP read                | ACTIVE     | Send BridgeData to relay                          |
| ACTIVE     | CloseBridgeRequest      | CLOSING    | Close TCP connection                              |
| CLOSING    | Cleanup complete        | CLOSED     | Send CloseBridgeResponse, remove from map         |
| ACTIVE     | TCP error               | CLOSING    | Initiate cleanup                                  |

## Data Streaming

Once a bridge is open, data flows using `BridgeDataMessage`:

```
Relay --> Node (HTTP request to camera):
{
  "type": 0x20,
  "id": 123,
  "bridge_id": "br-1",
  "sequence": 1,
  "data": [0x47, 0x45, 0x54...]  // "GET /axis-cgi/mjpg/video.cgi HTTP/1.1..."
}

Node --> Relay (MJPEG response):
{
  "type": 0x20,
  "id": 124,
  "bridge_id": "br-1",
  "sequence": 1,
  "data": [0xFF, 0xD8, 0xFF...]  // JPEG data
}
```

**Key points:**

- Each side can send data independently
- No need to wait for ACK (fire-and-forget streaming)
- Sequence numbers help detect gaps but are not strictly required for flow control
- Bridge closure implicitly stops all data flow
