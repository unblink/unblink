# vLLM with Qwen3-VL-4B-Instruct Setup Guide

This guide explains how to set up and run vLLM with the Qwen3-VL-4B-Instruct vision-language model using Docker.

## Overview

- **Model**: Qwen/Qwen3-VL-4B-Instruct (4 billion parameters)
- **Architecture**: Vision-language multimodal model (supports images and videos)
- **Hardware**: NVIDIA H100 80GB GPU
- **API**: OpenAI-compatible HTTP API

## Prerequisites

- Docker with NVIDIA Container Toolkit
- NVIDIA GPU (H100 80GB recommended, minimum 24GB VRAM)
- 16GB+ system RAM
- 20GB+ disk space

## Quick Start

### 1. Start the Server

```bash
./start-vllm.sh
```

Or manually:

```bash
sudo docker run --gpus all --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  -p 8000:8000 --rm vllm/vllm-openai:v0.11.0 \
  --model Qwen/Qwen3-VL-4B-Instruct \
  --gpu-memory-utilization 0.8 \
  --max-model-len 32000
```

### 2. Test the Server

```bash
python test_vllm.py
```

### 3. Use the API

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3-VL-4B-Instruct",
    "messages": [{
      "role": "user",
      "content": [{"type": "text", "text": "Hello!"}]
    }]
  }'
```

## Docker Configuration

The Docker run command includes several important flags:

| Flag                      | Purpose                                   |
| ------------------------- | ----------------------------------------- |
| `--gpus all`              | Pass all GPUs to the container            |
| `--ipc=host`              | Enable shared memory for multi-processing |
| `--ulimit memlock=-1`     | Remove memory locking limit               |
| `--ulimit stack=67108864` | Set stack size                            |
| `-p 8000:8000`            | Map container port 8000 to host           |
| `--rm`                    | Remove container on exit                  |

## Server Configuration Options

| Option                     | Default       | Description                             |
| -------------------------- | ------------- | --------------------------------------- |
| `--model`                  | Required      | Model name (HuggingFace format)         |
| `--gpu-memory-utilization` | 0.9           | Fraction of GPU memory to use (0.0-1.0) |
| `--max-model-len`          | Model default | Maximum sequence length                 |
| `--port`                   | 8000          | API server port                         |
| `--host`                   | 0.0.0.0       | Server host address                     |

### Memory Optimization Flags

For single-GPU setups:

```bash
# For image-only processing (saves memory)
--limit-mm-per-prompt.video 0

# Reduce context length for more memory
--max-model-len 16000

# Lower GPU memory utilization
--gpu-memory-utilization 0.7
```

## API Endpoints

The server provides OpenAI-compatible endpoints:

- `POST /v1/chat/completions` - Chat completions with multimodal support
- `POST /v1/completions` - Text completions
- `GET /v1/models` - List available models
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

Interactive API documentation is available at:

- Swagger UI: `http://localhost:8000/docs`
- ReDoc: `http://localhost:8000/redoc`

## Usage Examples

### Text-Only Request

```python
from openai import OpenAI

client = OpenAI(
    api_key="EMPTY",
    base_url="http://localhost:8000/v1"
)

response = client.chat.completions.create(
    model="Qwen/Qwen3-VL-4B-Instruct",
    messages=[{
        "role": "user",
        "content": [{"type": "text", "text": "Explain quantum computing"}]
    }],
    max_tokens=512
)

print(response.choices[0].message.content)
```

### Image + Text Request

```python
from openai import OpenAI

client = OpenAI(
    api_key="EMPTY",
    base_url="http://localhost:8000/v1"
)

response = client.chat.completions.create(
    model="Qwen/Qwen3-VL-4B-Instruct",
    messages=[{
        "role": "user",
        "content": [
            {
                "type": "image_url",
                "image_url": {"url": "https://example.com/image.jpg"}
            },
            {
                "type": "text",
                "text": "Describe this image in detail."
            }
        ]
    }],
    max_tokens=512
)

print(response.choices[0].message.content)
```

### Local Image File

```python
import base64
from openai import OpenAI

client = OpenAI(api_key="EMPTY", base_url="http://localhost:8000/v1")

# Read and encode image
with open("path/to/image.jpg", "rb") as f:
    image_data = base64.b64encode(f.read()).decode()

response = client.chat.completions.create(
    model="Qwen/Qwen3-VL-4B-Instruct",
    messages=[{
        "role": "user",
        "content": [
            {
                "type": "image_url",
                "image_url": {"url": f"data:image/jpeg;base64,{image_data}"}
            },
            {"type": "text", "text": "What do you see?"}
        ]
    }],
    max_tokens=512
)

print(response.choices[0].message.content)
```

## Model Variants

| Model                              | Parameters | Description                     |
| ---------------------------------- | ---------- | ------------------------------- |
| `Qwen/Qwen3-VL-4B-Instruct`        | 4B         | Standard model (recommended)    |
| `Qwen/Qwen3-VL-4B-Instruct-FP8`    | 4B         | FP8 quantized (less memory)     |
| `Qwen/Qwen3-VL-8B-Instruct`        | 8B         | Larger model (more capacity)    |
| `Qwen/Qwen3-VL-235B-A22B-Instruct` | 235B       | Flagship MoE (requires 8x GPUs) |

## Troubleshooting

### CUDA Compatibility Issues

If you encounter "unsupported PTX" errors with native installation, use Docker:

```bash
# Native installations may have CUDA version mismatches
# Docker provides a consistent CUDA environment
sudo docker run --gpus all vllm/vllm-openai:v0.11.0 ...
```

### Out of Memory

Reduce memory usage:

```bash
--gpu-memory-utilization 0.6 --max-model-len 16000 --limit-mm-per-prompt.video 0
```

### Slow First Request

The first request takes longer (~1-2 minutes) due to:

- Model loading and compilation
- CUDA graph capture
- Kernel autotuning

Subsequent requests are much faster.

### Port Already in Use

Change the port:

```bash
sudo docker run ... -p 8001:8000 ...
```

## Performance Tips

1. **Use FP8 quantized model** for lower memory usage with minimal quality loss
2. **Disable video processing** if only using images (`--limit-mm-per-prompt.video 0`)
3. **Adjust context length** based on your needs (`--max-model-len`)
4. **Batch requests** for better throughput
5. **Use streaming responses** for faster time-to-first-token

### Streaming Example

```python
from openai import OpenAI

client = OpenAI(api_key="EMPTY", base_url="http://localhost:8000/v1")

stream = client.chat.completions.create(
    model="Qwen/Qwen3-VL-4B-Instruct",
    messages=[{"role": "user", "content": [{"type": "text", "text": "Count to 100"}]}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

## Performance Benchmarks

### Hardware

- **GPU**: NVIDIA H100 80GB HBM3
- **Model**: Qwen/Qwen3-VL-4B-Instruct (4B parameters)
- **Configuration**: 80% GPU memory utilization, 32K max sequence length
- **Test**: Each request processes 10 video frames

### Same Frame vs Different Frames Comparison

| Test Type                        | Frames/Sec   | Requests/Sec    | Efficiency | Avg Latency |
| -------------------------------- | ------------ | --------------- | ---------- | ----------- |
| Same Frame (cached)              | ~107 fps     | 10.74 req/s     | 77.3%      | 6.91s       |
| **Different Frames (realistic)** | **~155 fps** | **15.46 req/s** | **92.9%**  | **5.77s**   |

> **Key Finding**: Processing different frames (realistic video) is actually **44% faster** than processing the same frame repeatedly. The vision encoder handles diverse content more efficiently than identical frames, likely due to better batching and reduced cache lookup overhead.

### Concurrent Throughput Results (Same Frame - Baseline)

| Concurrent | Frames/Sec | Requests/Sec | Tokens/Sec | Efficiency | Avg Latency |
| ---------- | ---------- | ------------ | ---------- | ---------- | ----------- |
| 2          | 6.7        | 0.67         | 85.7       | 91.5%      | 2.73s       |
| 4          | 20.1       | 2.01         | 257.6      | 85.3%      | 1.70s       |
| 8          | 40.6       | 4.06         | 518.1      | 84.9%      | 1.67s       |
| 16         | 53.9       | 5.39         | 673.2      | 71.4%      | 2.12s       |
| 24         | 55.2       | 5.52         | 704.8      | 81.5%      | 3.54s       |
| 32         | 82.4       | 8.24         | 1037.2     | 67.6%      | 2.63s       |
| 48         | 110.1      | 11.01        | 1386.8     | 72.1%      | 3.14s       |
| 64         | 112.4      | 11.24        | 1418.3     | 69.8%      | 3.97s       |
| 96         | **144.9**  | 14.49        | 1824.1     | 69.0%      | 4.57s       |
| 128        | 119.8      | 11.98        | 1500.0     | 62.9%      | 6.73s       |

### Performance Summary

```
        145 fps │                    *
                |              *     *
        120 fps │         *                *
                |       *
        100 fps │      *
                |    *
         80 fps │   *
                |  *
         60 fps │ *
                |
         40 fps │*
                |
         20 fps │*
                │___________________________________
                    8   16   32   48   64   96   128
                         Concurrent Requests
```

### Key Findings

- **Maximum Throughput**: **~155 frames/second** with different frames (realistic video) at 96 concurrent requests
- **Surprising Result**: Different frames process 44% faster than same-frame tests (cache misses > cache hits)
- **Saturation Point**: Performance peaks at 96 concurrent requests, then declines
- **Optimal Range**: 48-96 concurrent requests for best throughput
- **Best Efficiency**: 92.9% parallel efficiency with diverse frames
- **Real-time Video**: Can process 30 fps video at ~5x real-time speed

### Realistic Video Throughput Test

The `test_realistic_throughput.py` script tests actual video processing scenarios with 10 different frames per request:

```bash
# Generate 10 synthetic test frames (optional - creates test_frames/)
python generate_frames.py

# Run realistic throughput test
python test_realistic_throughput.py --requests 48

# Test at higher concurrency
python test_realistic_throughput.py --requests 96 -o realistic_results.json
```

**Why different frames are faster:**

1. **Better GPU utilization**: Diverse frames allow more efficient batching
2. **Reduced cache overhead**: No cache lookup overhead for unique content
3. **Parallel encoding**: Vision encoder can process different images in parallel more effectively
4. **No contention**: Same frames create contention on shared cache/encoder resources

### Recommendations

| Use Case                  | Recommended Concurrency | Expected Performance         |
| ------------------------- | ----------------------- | ---------------------------- |
| Low latency (interactive) | 4-8 requests            | 1.7-2s latency, 20-40 fps    |
| Balanced throughput       | 32-48 requests          | 2.6-3.1s latency, 82-110 fps |
| Maximum throughput        | 96 requests             | 4.6s latency, ~145 fps       |
| Batch processing          | 48-96 requests          | Best overall efficiency      |

### Running Benchmarks

```bash
# Single request with 10 frames
python test_throughput.py

# Generate synthetic test frames (for realistic test)
python generate_frames.py

# Concurrent throughput test (same frame repeated)
python test_concurrent_throughput.py --requests 32

# Realistic throughput test (10 different frames per request)
python test_realistic_throughput.py --requests 48

# Save results to JSON
python test_realistic_throughput.py --requests 96 -o realistic_results.json

# Test with different output lengths
python test_concurrent_throughput.py --requests 16 --max-tokens 512
```

## Monitoring

### View Server Logs

The server logs output to stdout. Key metrics are logged periodically:

```
Engine 000: Avg prompt throughput: 24.6 tokens/s, Avg generation throughput: 7.4 tokens/s,
Running: 0 reqs, Waiting: 0 reqs, GPU KV cache usage: 0.0%, Prefix cache hit rate: 46.6%
```

### Prometheus Metrics

```bash
curl http://localhost:8000/metrics
```

## Resources

- [vLLM Documentation](https://docs.vllm.ai/)
- [Qwen3-VL on Hugging Face](https://huggingface.co/Qwen/Qwen3-VL-4B-Instruct)
- [vLLM GitHub](https://github.com/vllm-project/vllm)
- [Qwen GitHub](https://github.com/QwenLM/Qwen3-VL)

## License

- vLLM: Apache 2.0
- Qwen3-VL: Apache 2.0

```
#!/bin/bash
# Start both Qwen3-VL-4B and Qwen3-VL-8B servers on different ports

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  Starting Multiple vLLM Servers${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Stop any existing vllm containers
echo -e "${YELLOW}Stopping existing vLLM containers...${NC}"
sudo docker ps -a | grep vllm | awk '{print $1}' | xargs -r sudo docker rm -f
sleep 2

# Start 4B VL model on port 8000 FIRST (~35GB GPU memory = 44%)
echo -e "${GREEN}Starting Qwen3-VL-4B-Instruct on port 8000...${NC}"
VLLM_MODEL=Qwen/Qwen3-VL-4B-Instruct VLLM_PORT=8000 VLLM_GPU_MEMORY=0.44 ./start-vllm.sh &
VLLM_4B_PID=$!

# Wait for 4B to fully initialize
sleep 30

# Start 8B model on port 8001 SECOND (~40GB GPU memory = 50%)
echo -e "${GREEN}Starting Qwen3-8B on port 8001...${NC}"
VLLM_MODEL=Qwen/Qwen3-8B VLLM_PORT=8001 VLLM_GPU_MEMORY=0.50 \
VLLM_REASONING_PARSER=deepseek_r1 \
./start-vllm.sh &
VLLM_8B_PID=$!

echo ""
echo -e "${GREEN}Both servers starting...${NC}"
echo -e "${BLUE}4B VL Model: http://localhost:8000${NC}"
echo -e "${BLUE}8B Model: http://localhost:8001${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop both servers${NC}"

# Trap Ctrl+C to cleanup
trap 'echo -e "\n${YELLOW}Shutting down servers...${NC}"; kill $VLLM_4B_PID $VLLM_8B_PID 2>/dev/null; sudo docker ps -a | grep vllm | awk "{print \$1}" | xargs -r sudo docker rm -f; exit 0' INT TERM

# Wait for both background processes
wait $VLLM_4B_PID $VLLM_8B_PID
```

```
#!/bin/bash
# vLLM Server Startup Script for Qwen3-VL-4B-Instruct
# This script starts the vLLM server with Docker for proper CUDA compatibility

set -e

# Configuration
MODEL="${VLLM_MODEL:-Qwen/Qwen3-VL-8B-Instruct}"
VLLM_VERSION="${VLLM_VERSION:-v0.11.0}"
PORT="${VLLM_PORT:-8000}"
HOST="${VLLM_HOST:-0.0.0.0}"
GPU_MEMORY_UTILIZATION="${VLLM_GPU_MEMORY:-0.8}"
MAX_MODEL_LEN="${VLLM_MAX_LEN:-32000}"

# Optional: Limit video processing to save memory
LIMIT_VIDEO="${VLLM_LIMIT_VIDEO:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print banner
echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  vLLM Server Startup Script${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    echo "Please install Docker first: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check if NVIDIA Container Toolkit is installed
if ! docker run --rm --gpus all nvidia/cuda:11.0.3-base-ubuntu20.04 nvidia-smi &> /dev/null; then
    echo -e "${RED}Error: NVIDIA Container Toolkit not properly installed${NC}"
    echo "Please install it: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html"
    exit 1
fi

# Check if sudo is available (needed for Docker GPU access)
if ! sudo -n true 2>/dev/null; then
    echo -e "${YELLOW}Warning: sudo access required for Docker GPU access${NC}"
fi

# Build Docker command
DOCKER_CMD="sudo docker run --gpus all"
DOCKER_CMD="$DOCKER_CMD --ipc=host"
DOCKER_CMD="$DOCKER_CMD --ulimit memlock=-1"
DOCKER_CMD="$DOCKER_CMD --ulimit stack=67108864"
DOCKER_CMD="$DOCKER_CMD -p ${PORT}:8000"
DOCKER_CMD="$DOCKER_CMD --rm"
DOCKER_CMD="$DOCKER_CMD vllm/vllm-openai:${VLLM_VERSION}"

# vLLM arguments
VLLM_ARGS="--model ${MODEL}"
VLLM_ARGS="$VLLM_ARGS --gpu-memory-utilization ${GPU_MEMORY_UTILIZATION}"
VLLM_ARGS="$VLLM_ARGS --max-model-len ${MAX_MODEL_LEN}"
VLLM_ARGS="$VLLM_ARGS --host ${HOST}"
VLLM_ARGS="$VLLM_ARGS --enable-auto-tool-choice"
VLLM_ARGS="$VLLM_ARGS --tool-call-parser=hermes"
VLLM_ARGS="$VLLM_ARGS --enable-chunked-prefill"
VLLM_ARGS="$VLLM_ARGS --enable-prefix-caching"

# Optional: Enable reasoning (for models that support it)
if [ "${VLLM_ENABLE_REASONING:-false}" = "true" ]; then
    VLLM_ARGS="$VLLM_ARGS --enable-reasoning"
fi

# Optional: Reasoning parser (can be used without enable-reasoning)
if [ -n "${VLLM_REASONING_PARSER:-}" ]; then
    VLLM_ARGS="$VLLM_ARGS --reasoning-parser=$VLLM_REASONING_PARSER"
fi

# Optional: KV cache dtype (for memory optimization)
if [ -n "${VLLM_KV_CACHE_DTYPE:-}" ]; then
    VLLM_ARGS="$VLLM_ARGS --kv-cache-dtype $VLLM_KV_CACHE_DTYPE"
fi

# Optional video limiting
if [ "$LIMIT_VIDEO" = "true" ]; then
    VLLM_ARGS="$VLLM_ARGS --limit-mm-per-prompt.video 0"
    echo -e "${YELLOW}Video processing disabled (image-only mode)${NC}"
fi

# Print configuration
echo -e "${GREEN}Configuration:${NC}"
echo "  Model:              ${MODEL}"
echo "  vLLM Version:       ${VLLM_VERSION}"
echo "  Port:               ${PORT}"
echo "  GPU Memory:         ${GPU_MEMORY_UTILIZATION}"
echo "  Max Sequence Length: ${MAX_MODEL_LEN}"
echo "  Video Processing:   $([ "$LIMIT_VIDEO" = "true" ] && echo "Disabled" || echo "Enabled")"
echo ""

# Check if port is already in use
if sudo lsof -i :${PORT} &> /dev/null; then
    echo -e "${RED}Error: Port ${PORT} is already in use${NC}"
    echo "Please stop the existing service or change VLLM_PORT"
    exit 1
fi

# Display startup message
echo -e "${GREEN}Starting vLLM server...${NC}"
echo -e "${BLUE}Server will be available at: http://localhost:${PORT}${NC}"
echo -e "${BLUE}API docs: http://localhost:${PORT}/docs${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop the server${NC}"
echo ""

# Trap Ctrl+C to cleanup
trap 'echo -e "\n${YELLOW}Shutting down server...${NC}"; exit 0' INT TERM

# Start the server
eval $DOCKER_CMD $VLLM_ARGS
```
