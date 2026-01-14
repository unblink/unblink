# Federated Workers

## Overview

Unblink uses a federated worker model for AI vision processing. Camera events are broadcast to workers with matching agent assignments, which process frames according to agent instructions and emit back events (summaries, detections, alerts, etc.) that are stored and searchable.

You can self-host your AI workers, use public ones, or use dedicated workers provided by Unblink.

## Agent-Based Processing

Workers receive frame batches with **agent instructions** attached. Agents are user-configured entities that define:
- **name**: Descriptive name (e.g., "Security Monitor")
- **instruction**: Natural language prompt (e.g., "Are there any suspicious activities?")
- **worker_id**: Which worker type processes this agent (e.g., "unblink/base-vl")
- **service_ids**: Which cameras this agent monitors

When a camera emits frames, only workers with matching agents receive the batch. This enables:
- **Filtered broadcasting**: Workers only receive relevant events
- **Custom instructions**: Each agent can have different analysis goals
- **Parallel processing**: Multiple agents process the same frames concurrently

## Event Flow

```
                         ┌───────────────────────────────────┐
                         │        Relay + AgentRegistry       │
                         │  (service → agents in-memory map) │
                         └───────────────────────────────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    │ Lookup agents      │                    │
                    │ for service        │                    │
                    │ (O(1) lookup)      │                    │
                    └────────────────────┼────────────────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    │  Filter by worker_id                    │
                    │  Attach agent instructions              │
                    └────────────────────┼────────────────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    │                    │                    │
               ┌────▼────┐          ┌────▼────┐         ┌────▼────┐
               │ Worker 1│          │ Worker 2 │         │ Worker 3│
               │ base-vl │          │ custom   │         │ base-vl │
               │ (Agent A│          │ (Agent C)│         │ (Agent A│
               │  Agent B)│          │          │         │  Agent B)│
               └────┬────┘          └────┬────┘         └────┬────┘
                    │                    │                    │
                    │   Process frames with agent instructions│
                    │   (parallel processing, streaming)      │
                    │                    │                    │
                    └────────────────────┼────────────────────┘
                                         │
                    ┌────────────────────▼────────────────────┐
                    │     Agent Results (stored/searchable)   │
                    │  {"agent_id": "...", "data": {...}}     │
                    └─────────────────────────────────────────┘
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

Frame events are only sent to workers with agents configured for the service (consistent with frame_batch).

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

**Note**: Only workers with agents configured for this service will receive the event.

### Frame Batch Event (with Agent Instructions)

Frame batch events now include the `agents` array containing instructions for each agent monitoring this service.

```json
{
  "type": "frame_batch",
  "id": "...",
  "created_at": "2026-01-10T12:00:00Z",
  "data": {
    "service_id": "cam-uuid",
    "frames": ["uuid-1", "uuid-2", ..., "uuid-10"],
    "metadata": {
      "duration_seconds": 50.5,
      "start_time": 0.0,
      "fps": 2.0
    },
    "agents": [
      {
        "id": "agent-123",
        "name": "Security Monitor",
        "instruction": "Are there any suspicious activities or unauthorized persons?"
      },
      {
        "id": "agent-456",
        "name": "Safety Equipment Check",
        "instruction": "Are all personnel wearing required safety equipment?"
      }
    ]
  }
}
```

**Key changes:**
- **agents array**: Contains all agents assigned to this service that match the worker's `worker_id`
- **Filtered broadcasting**: Only workers with matching `worker_id` receive the event
- **Zero DB queries**: Agent lookup uses in-memory `AgentRegistry` (O(1) lookup)

**Worker behavior:**
- Process all agents in the array **in parallel** (using `asyncio.gather()` in Python)
- Use each agent's `instruction` as the prompt for inference
- Emit results immediately as each agent completes (streaming, not batched)

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

Workers emit structured agent results back to the relay. Each result corresponds to one agent's processing.

### Agent Result (Success)

```json
{
  "agent_id": "agent-123",
  "data": {
    "answer": "No suspicious activities detected. All personnel are authorized."
  },
  "inference_time_seconds": 8.5,
  "created_at": "2026-01-14T12:00:00Z"
}
```

### Agent Result (Error)

```json
{
  "agent_id": "agent-456",
  "error": {
    "message": "Model inference failed: CUDA out of memory"
  },
  "created_at": "2026-01-14T12:00:05Z"
}
```

### Result Structure

**Required fields:**
- `agent_id` (string): ID of the agent that processed this batch
- `created_at` (string): ISO 8601 timestamp

**Mutually exclusive fields (one of):**
- `data` (object): Success result with semantic keys like `answer`, `summary`, `alert`
- `error` (object): Error result with `message` key

**Optional fields:**
- `inference_time_seconds` (float): Processing duration (only for success)

**Examples of different result types:**

```json
// Answer result (vision analysis)
{"agent_id": "...", "data": {"answer": "..."}, "created_at": "..."}

// Summary result (batch processing)
{"agent_id": "...", "data": {"summary": "..."}, "created_at": "..."}

// Alert result (detection)
{"agent_id": "...", "data": {"alert": "Motion detected"}, "created_at": "..."}
```

## Worker Lifecycle

1. **Connect** via WebSocket to `/worker/connect`
2. **Register** with `worker_id` (e.g., "unblink/base-vl") and receive authentication key
3. **Listen** for `frame_batch` events via WebSocket
4. **Receive** events **only if** agents with matching `worker_id` exist for the service
5. **Download** frames once using HTTP GET with the key (shared across all agents)
6. **Process** all agents in the batch **in parallel** using each agent's instruction
7. **Stream** results back via WebSocket as each agent completes (don't wait for all)
8. **Disconnect** - key is invalidated

## Agent Registry (Relay-Side Architecture)

The relay maintains an in-memory `AgentRegistry` to avoid database queries during frame batch emission.

**Data structure:**
- `serviceID → []AgentInfo`: O(1) lookup for agents by service
- `agentID → AgentInfo`: Quick lookups for agent details

**Operations:**
- `LoadFromDatabase()`: Called once at relay startup
- `RegisterAgent(agent)`: Called when agent created/updated via API
- `RemoveAgent(agentID)`: Called when agent deleted
- `GetAgentsForService(serviceID)`: O(1) lookup during frame_batch emission

**Performance:**
- **Memory**: ~500 bytes per agent (negligible for thousands of agents)
- **Lookup**: O(1) for service → agents mapping
- **DB queries**: Zero during frame batch emission
- **Updates**: Only on agent CRUD operations (low frequency)

## Streaming Parallel Processing (Worker-Side)

Workers process multiple agents concurrently and stream results back immediately.

**Pattern (Python):**

```python
async def process_single_agent(agent, frames, model, processor, emit_callback):
    """Process one agent and emit result immediately"""
    instruction = agent['instruction']

    # Run inference with agent's instruction
    result = await run_inference(frames, instruction, model, processor)

    # Emit immediately (don't wait for other agents)
    await emit_callback({
        "agent_id": agent['id'],
        "data": {"answer": result},
        "inference_time_seconds": elapsed,
        "created_at": datetime.now(timezone.utc).isoformat()
    })

async def process_frame_batch(event, emit_callback):
    agents = event['agents']
    frames = download_frames(event['frames'])  # Download once

    # Process all agents in parallel
    tasks = [
        process_single_agent(agent, frames, model, processor, emit_callback)
        for agent in agents
    ]
    await asyncio.gather(*tasks)  # Parallel execution
```

**Benefits:**
- **Parallel execution**: 3 agents × 10s = ~10s total (vs 30s sequential)
- **Progressive results**: Frontend receives first result in 8-10s
- **Failure isolation**: One agent error doesn't block others
- **Resource efficiency**: GPU utilized continuously

## Example Worker Implementation

See [examples/worker-base-vl/](../examples/worker-base-vl/) for a complete reference implementation using Qwen3-VL-4B-Instruct.

**Key files:**
- `main.py`: WebSocket connection, registration, and event handling
- `events/process_frame_batch_event.py`: Parallel agent processing with streaming

**Running the example:**

```bash
cd examples/worker-base-vl

# Install dependencies
uv sync

# Run worker (connects to localhost:9020 by default)
uv run main.py
```

**How it works:**
1. Connects to relay at `ws://localhost:9020/worker/connect`
2. Registers with `worker_id="unblink/base-vl"`
3. Loads Qwen3-VL model on GPU
4. Receives `frame_batch` events with agents array
5. Downloads frames once via HTTP GET
6. Processes all agents in parallel using `asyncio.gather()`
7. Streams each result back immediately via WebSocket

**Customization:**
- Change `worker_id` in `main.py` to create custom worker types
- Modify `process_single_agent()` to use different models or logic
- Add custom result types by changing the `data` object structure
