# Federated Workers

## Overview

Unblink uses a federated worker model for AI vision processing. Camera events are broadcast to authorized workers, which process frames and emit back events (summaries, detections, alerts, etc.) that are stored and searchable.

You can self-host your AI workers, use public ones, or use dedicated workers provided by Unblink.

## Event Flow

```
                    ┌─────────────────────────────────────┐
                    │           Relay Event Bus           │
                    └─────────────────────────────────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    │                  │                  │
               ┌────▼────┐        ┌────▼────┐       ┌────▼────┐
               │ Worker 1│        │ Worker 2 │      │ Worker 3│
               │(self-   │        │ (public) │      │ (unblink│
               │  hosted)│        │          │      │  hosted)│
               └────┬────┘        └────┬────┘       └────┬────┘
                    │                  │                  │
                    │              Process frames        │
                    │                  │                  │
                    └──────────────────┼──────────────────┘
                                       │
                    ┌──────────────────▼──────────────────┐
                    │    Worker Events (stored/searchable)│
                    │  - summaries  - metrics  - alerts   │
                    └─────────────────────────────────────┘
```

## Worker Protocol

### Connection

Workers connect via WebSocket to the main relay server:

```
ws://relay:9020/worker/connect
```

**Note:** Workers now use the same WebSocket server as nodes (port 9020), with endpoint namespacing (`/worker/connect` vs `/node/connect`).

### Registration

Worker generates and sends its own identifier as `worker_id`:

**Worker → Relay:**

```json
{
  "type": "register",
  "data": {
    "worker_id": "unblink/base-vl"
  }
}
```

**Relay → Worker:**

```json
{
  "type": "registered",
  "data": {
    "key": "a3f9e2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1"
  }
}
```

The `key` is a 256-bit cryptographic token used for HTTP requests (frame downloads).

### Heartbeat

**Worker → Relay (via WebSocket):**

```json
{
  "type": "heartbeat"
}
```

## Incoming Events (Relay → Worker)

### Frame Event

```json
{
  "type": "frame",
  "id": "...",
  "created_at": "2026-01-10T12:00:00Z",
  "data": {
    "service_id": "cam-uuid",
    "frame_uuid": "frame-uuid"
  }
}
```

### Frame Batch Event

```json
{
  "type": "frame_batch",
  "id": "...",
  "created_at": "2026-01-10T12:00:00Z",
  "data": {
    "service_id": "cam-uuid",
    "frames": ["uuid-1", "uuid-2", ..., "uuid-10"],
    "metadata": {
      "duration_seconds": 50.5
    }
  }
}
```

## Worker APIs

### Download Frame (HTTP GET)

Frame downloads use HTTP GET with the worker key for authentication.

**Request:**

```bash
GET /worker/frames/{frameUUID}
Header: X-Worker-Key: {worker_key}
```

**Example:**

```bash
curl http://localhost:9020/worker/frames/{frame_uuid} \
  -H "X-Worker-Key: {your_key}" \
  -o frame.jpg
```

### Emit Event (WebSocket)

Workers emit events back to the relay via the existing WebSocket connection using an `event` message type.

**Message:**

```json
{
  "type": "event",
  "data": {
    // your event data
  }
}
```

**Example (Python):**

```python
event_msg = {
    "type": "event",
    "data": {
        "summary": "Processed 10 frames successfully"
    }
}
await ws.send(json.dumps(event_msg))
```

## Outgoing Events (Worker → Relay)

Workers can emit any JSON-serializable data. Common patterns:

### Summary Event

```json
{
  "summary": "Processed 10 frames from cam-123. Duration: 50.5s"
}
```

### Metrics Event

```json
{
  "metric_type": "detection_count",
  "value": 42,
  "created_at": "2026-01-10T12:00:00Z"
}
```

### Alert Event

```json
{
  "alert": "Motion detected in zone A",
  "severity": "high",
  "created_at": "2026-01-10T12:00:00Z"
}
```

### Custom Event

```json
{
  "custom_field": "any data",
  "detections": [
    { "class": "person", "confidence": 0.95, "bbox": [10, 20, 30, 40] }
  ],
  "created_at": "2026-01-10T12:00:00Z"
}
```

## Worker Lifecycle

1. **Connect** via WebSocket to `/worker/connect`
2. **Register** and receive authentication key
3. **Listen** for `frame` and `frame_batch` events via WebSocket
4. **Download** frames using HTTP GET with the key
5. **Process** frames with AI models
6. **Emit** events back via WebSocket (not HTTP POST)
7. **Disconnect** - key is invalidated
