# Client WebSocket Protocol

## Overview

Browser clients connect to the relay via WebSocket to receive real-time agent events. The protocol follows the same pattern as node and worker connections.

## Connection

Clients connect via WebSocket:

```
ws://relay:9020/client/connect
```

## Registration

After connecting, the client must register by sending a JWT token.

**Client → Relay:**

```json
{
  "type": "register",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**Relay → Client (success):**

```json
{
  "type": "registered",
  "data": {
    "user_id": 123
  }
}
```

**Relay → Client (error):**

```json
{
  "type": "error",
  "data": {
    "message": "Invalid token"
  }
}
```

The relay validates the JWT token and extracts the `user_id`. Connection is closed if authentication fails.

## Outgoing Messages (Client → Relay)

### Request Agent Events

After registration, clients can request historical events for specific agents or services.

**Client → Relay:**

```json
{
  "type": "req_agent_events",
  "data": {
    "agent_id": "agent-123",
    "limit": 100
  }
}
```

Or request by service:

```json
{
  "type": "req_agent_events",
  "data": {
    "service_id": "service-456",
    "limit": 100
  }
}
```

- `agent_id` (optional): Filter events by specific agent
- `service_id` (optional): Filter events by service (returns events from all agents monitoring this service)
- `limit` (optional): Maximum events to return (default: 100, max: 1000)

**Relay → Client:**

```json
{
  "type": "res_agent_events",
  "data": {
    "events": [
      {
        "id": "event-uuid",
        "agent_id": "agent-123",
        "agent_name": "Security Monitor",
        "service_ids": ["service-456"],
        "data": {
          "answer": "No suspicious activity detected"
        },
        "metadata": {
          "inference_time_seconds": 8.5
        },
        "created_at": "2026-01-15T10:30:00Z"
      }
    ]
  }
}
```

## Incoming Events (Relay → Client)

### Agent Event

When a worker emits an agent event, the relay forwards it to browser clients who own that agent.

```json
{
  "type": "agent_event",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-01-15T10:30:00Z",
  "data": {
    "id": "event-uuid",
    "agent_id": "agent-123",
    "agent_name": "Security Monitor",
    "service_ids": ["service-456"],
    "data": {
      "answer": "No suspicious activity detected",
      "summary": "All clear"
    },
    "metadata": {
      "inference_time_seconds": 8.5
    },
    "created_at": "2026-01-15T10:30:00Z"
  }
}
```

**Filtering:**

- Only events for agents owned by the authenticated user are sent
- Events are broadcast to all connected clients for that user

## Heartbeat

The relay sends periodic ping messages (every 30 seconds) to keep the connection alive. Clients should respond with pong (handled automatically by browser WebSocket API).

## Client Lifecycle

1. **Connect** via WebSocket to `/client/connect`
2. **Register** with JWT token
3. **Receive** `registered` confirmation or `error`
4. **Listen** for `agent_event` messages
5. **Auto-reconnect** on disconnect (recommended: 3s delay)

## Security

- **Authentication:** JWT token validated on registration
- **Authorization:** Only events for user's agents are sent
- **Token refresh:** Client should reconnect with new token before expiry
- **Origin validation:** TODO - Add CORS origin checking for production

## Message Format

All messages follow the same structure:

```typescript
{
  type: string;           // Message type identifier
  id?: string;            // Optional message UUID
  created_at?: string;    // Optional ISO 8601 timestamp
  data?: object;          // Message payload
}
```
