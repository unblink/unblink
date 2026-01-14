import asyncio
import requests
import time
from datetime import datetime, timezone
from PIL import Image
from io import BytesIO


async def download_frame(http_url: str, worker_key: str, frame_uuid: str) -> bytes | None:
    """Download a frame from the relay, return JPEG bytes"""
    try:
        response = requests.get(
            f"{http_url}/worker/frames/{frame_uuid}",
            headers={"X-Worker-Key": worker_key},
            timeout=10
        )
        if response.status_code == 200:
            return response.content
        else:
            print(f"[frame_batch_event] Failed to download {frame_uuid}: {response.status_code}")
            return None
    except Exception as e:
        print(f"[frame_batch_event] Error downloading {frame_uuid}: {e}")
        return None


async def process_single_agent(
    agent: dict,
    pil_images: list,
    service_id: str,
    metadata: dict,
    model,
    processor,
    emit_callback
):
    """Process one agent and emit result immediately"""
    try:
        instruction = agent['instruction']
        agent_name = agent['name']

        # Get timing info from metadata
        duration_seconds = metadata.get('duration_seconds', 0)
        start_time = metadata.get('start_time', 0)
        fps = metadata.get('fps', 2.0)

        # Build temporal context text
        if start_time > 0:
            end_time = start_time + duration_seconds
            temporal_context = f"This video segment spans from {start_time:.1f}s to {end_time:.1f}s. "
        else:
            temporal_context = ""

        # Build messages with agent's instruction
        messages = [{
            "role": "user",
            "content": [
                {
                    "type": "video",
                    "video": pil_images,
                    "raw_fps": fps,
                },
                {
                    "type": "text",
                    "text": f"{temporal_context}{instruction}"  # Use agent's instruction
                }
            ]
        }]

        # Import qwen_vl_utils here (only when needed)
        from qwen_vl_utils import process_vision_info

        # Run inference
        image_inputs, video_inputs = process_vision_info(messages)
        text = processor.apply_chat_template(
            messages,
            tokenize=False,
            add_generation_prompt=True
        )
        inputs = processor(
            text=[text],
            images=image_inputs,
            videos=video_inputs,
            padding=True,
            return_tensors="pt"
        )
        inputs = inputs.to(model.device)

        print(f"[Agent {agent_name}] Running inference...")
        start = time.time()
        generated_ids = model.generate(**inputs, max_new_tokens=512)
        elapsed = time.time() - start

        # Slice to get only generated tokens (exclude input prompt)
        input_ids_len = inputs['input_ids'].shape[1]
        generated_only = generated_ids[:, input_ids_len:]
        output_text = processor.batch_decode(
            generated_only,
            skip_special_tokens=True,
            clean_up_tokenization_spaces=False
        )[0]

        # Emit this agent's result immediately
        result = {
            "agent_id": agent['id'],
            "data": {
                "answer": output_text
            },
            "inference_time_seconds": elapsed,
            "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        }

        print(f"[Agent {agent_name}] Completed in {elapsed:.2f}s, emitting result")
        await emit_callback(result)

    except Exception as e:
        print(f"[Agent {agent.get('name', 'Unknown')}] Error: {e}")
        # Emit error result
        await emit_callback({
            "agent_id": agent['id'],
            "error": {
                "message": str(e)
            },
            "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
        })


async def process_frame_batch_event(
    event: dict,
    http_url: str,
    worker_key: str,
    model,
    processor,
    emit_callback
):
    """
    Process frame batch with multiple agents, streaming results.

    Each agent's result is emitted immediately upon completion via emit_callback.
    Agents are processed in parallel using asyncio.gather().
    """
    frames = event.get('frames', [])
    service_id = event.get('service_id')
    agents = event.get('agents', [])
    metadata = event.get('metadata', {})

    if not agents:
        print(f"[frame_batch_event] No agents for service {service_id}, skipping")
        return

    print(f"\n[frame_batch_event] Processing batch: {len(frames)} frames, {len(agents)} agents from {service_id}")

    # Download frames once (shared across all agents)
    pil_images = []
    for frame_uuid in frames:
        jpeg_data = await download_frame(http_url, worker_key, frame_uuid)
        if jpeg_data:
            pil_image = Image.open(BytesIO(jpeg_data)).convert("RGB")
            pil_images.append(pil_image)

    if not pil_images:
        print(f"[frame_batch_event] No frames downloaded, skipping")
        return

    # Process all agents concurrently
    # Each agent will emit its own result as soon as it completes
    tasks = [
        process_single_agent(
            agent,
            pil_images,
            service_id,
            metadata,
            model,
            processor,
            emit_callback
        )
        for agent in agents
    ]

    # Wait for all agents to complete (but each streams results independently)
    await asyncio.gather(*tasks)

    print(f"[frame_batch_event] All {len(agents)} agents completed for batch")
