# Simple Worker

A lightweight CV worker for testing message flows without loading any heavy models.

## Purpose

This worker is designed for testing the entire message flow pipeline:
- Worker registration and connection
- Receiving frame batch events
- Processing multiple agents
- Emitting results back to the relay

Instead of running actual ML inference, it sends back summaries of what it received.

## Installation

```bash
cd examples/worker-simple
uv sync
```

## Usage

```bash
uv run python main.py
```

## What it does

1. Connects to the relay server at `ws://localhost:9020/worker/connect`
2. Registers as worker `unblink/simple`
3. Listens for `frame_batch` events
4. For each agent in the batch, generates a summary containing:
   - Agent name and instruction
   - Service ID
   - Frame count and UUIDs
   - Metadata
5. Emits results back to the relay immediately

## Example Output

When a frame batch is received, the worker will emit results like:

```json
{
  "agent_id": "agent-123",
  "data": {
    "answer": "Agent 'motion-detector' processed frame batch. | Instruction: Detect motion in this video segment | Service: service-abc | Frames received: 10 | Frame UUIDs: uuid1, uuid2, uuid3..."
  },
  "inference_time_seconds": 0.001,
  "created_at": "2025-01-14T12:00:00Z"
}
```

## Use Cases

- Testing the relay server configuration
- Verifying agent registration and routing
- Debugging message flow issues
- Load testing without GPU requirements
- Integration testing before adding actual ML models
