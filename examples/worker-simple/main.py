import asyncio
import json
import websockets
from datetime import datetime, timezone


class SimpleWorker:
    def __init__(
        self,
        relay_address: str = "ws://localhost:9020/worker/connect",
    ):
        self.relay_address = relay_address
        # self.worker_id = "unblink/simple"
        # Debug on base VL behalf
        self.worker_id = "unblink/base-vl" 
        self.worker_key = None
        self.ws = None

    async def connect(self):
        print(f"[Worker] Connecting to {self.relay_address}")
        self.ws = await websockets.connect(self.relay_address)

    async def register(self):
        registration_msg = {"type": "register", "data": {"worker_id": self.worker_id}}
        await self.ws.send(json.dumps(registration_msg))
        response = await self.ws.recv()
        data = json.loads(response)
        if data.get("type") == "registered":
            self.worker_key = data["data"]["key"]
            print(f"[Worker] Registered: {self.worker_id}")
            print(f"[Worker] Key: {self.worker_key[:16]}...")
        else:
            print(f"[Worker] Registration response: {response}")

    async def process_frame_batch(self, event: dict):
        """Process frame batch event and emit simple summaries"""
        frames = event.get('frames', [])
        service_id = event.get('service_id')
        agents = event.get('agents', [])
        metadata = event.get('metadata', {})

        if not agents:
            print(f"[Worker] No agents for service {service_id}, skipping")
            return

        # Process each agent and send back a simple summary
        for agent in agents:
            agent_id = agent['id']
            agent_name = agent.get('name', 'Unknown')
            instruction = agent.get('instruction', 'No instruction')

            # Generate a simple summary without loading any model
            duration = metadata.get('duration_seconds', 0)
            fps = metadata.get('fps', 2.0)
            start_time = metadata.get('start_time', 0)

            summary = (
                f"Simple test summary for agent '{agent_name}':\n"
                f"- Received {len(frames)} frames\n"
                f"- Duration: {duration:.1f}s (starting at {start_time:.1f}s)\n"
                f"- FPS: {fps}\n"
                f"- Instruction: {instruction}\n"
                f"- Service: {service_id}"
            )

            # Emit result back to relay
            result = {
                "agent_id": agent_id,
                "data": {
                    "answer": summary
                },
                "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
            }

            print(f"[Worker] Emitting result for agent '{agent_name}'", json.dumps(result))
            await self.emit_event(result)

        print(f"[Worker] Completed processing {len(agents)} agents")

    async def emit_event(self, event_data: dict):
        """Emit event back to relay via WebSocket"""
        try:
            event_msg = {"type": "event", "data": event_data}
            await self.ws.send(json.dumps(event_msg))
            print(f"[Worker] Event emitted: {event_data.get('agent_id', 'unknown')}")
        except Exception as e:
            print(f"[Worker] Emit error: {e}")

    async def listen(self):
        print(f"[Worker] Listening for events...")
        try:
            async for message in self.ws:
                data = json.loads(message)
                print(f"\n[Worker] Received message:")
                print(json.dumps(data, indent=2))

                event_type = data.get("type")
                if event_type == "frame_batch" and self.worker_key:
                    event = data.get("data")
                    if event:
                        await self.process_frame_batch(event)

        except websockets.exceptions.ConnectionClosed:
            print(f"[Worker] Connection closed")

    async def run(self):
        try:
            await self.connect()
            await self.register()
            await self.listen()
        except KeyboardInterrupt:
            print("\n[Worker] Shutting down...")
        finally:
            if self.ws:
                await self.ws.close()


def main():
    print("=" * 60)
    print("Simple Worker - Message Flow Testing")
    print("=" * 60 + "\n")
    print("This worker doesn't load any models.")
    print("It just echoes back simple summaries to test the message flow.\n")

    worker = SimpleWorker()
    asyncio.run(worker.run())


if __name__ == "__main__":
    main()
