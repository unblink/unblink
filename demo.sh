#!/usr/bin/env bash

set -e

FPS=30
HOST=127.0.0.1
RTSP_PORT=8554

# Start mediamtx in background
echo "Starting mediamtx RTSP server..."
mediamtx &
MEDIAMTX_PID=$!
sleep 1

start_stream () {
  NAME="$1"
  URL="$2"
  STREAM_PATH="$3"

  echo "Starting $NAME at rtsp://$HOST:$RTSP_PORT/$STREAM_PATH"

  ffmpeg -loglevel error \
    -stream_loop -1 -re \
    -i "$URL" \
    -vf fps=$FPS \
    -c:v libx264 -preset ultrafast -tune zerolatency \
    -f rtsp \
    "rtsp://$HOST:$RTSP_PORT/$STREAM_PATH" &
}

start_stream "Car Factory" \
  "https://bucket.zapdoslabs.com/car_factory.mp4" \
  "car_factory"

start_stream "Mask Machine" \
  "https://bucket.zapdoslabs.com/mask_machine.mp4" \
  "mask_machine"

start_stream "Mask Production Line" \
  "https://bucket.zapdoslabs.com/mask_production_line.mp4" \
  "mask_production_line"

start_stream "Steel Work Production" \
  "https://bucket.zapdoslabs.com/steel_work_producion.mp4" \
  "steel_work_production"

echo "All streams started."
echo ""
echo "Test with:"
echo "  ffplay rtsp://$HOST:$RTSP_PORT/car_factory"
echo "  ffplay rtsp://$HOST:$RTSP_PORT/mask_machine"
echo "  ffplay rtsp://$HOST:$RTSP_PORT/mask_production_line"
echo "  ffplay rtsp://$HOST:$RTSP_PORT/steel_work_production"

cleanup() {
  echo "Stopping..."
  kill $MEDIAMTX_PID 2>/dev/null || true
  pkill -P $$ 2>/dev/null || true
}
trap cleanup EXIT

wait
