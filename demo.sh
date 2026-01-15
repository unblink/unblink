#!/usr/bin/env bash

set -e

FPS=8
QUALITY=5
HOST=0.0.0.0

start_stream () {
  NAME="$1"
  URL="$2"
  PORT="$3"

  echo "Starting $NAME on port $PORT"

  ffmpeg -loglevel warning \
    -stream_loop -1 -re \
    -i "$URL" \
    -vf fps=$FPS \
    -q:v $QUALITY \
    -f mjpeg \
    -listen 1 \
    http://$HOST:$PORT/stream.mjpg &
}

start_stream "Car Factory" \
  "https://bucket.zapdoslabs.com/car_factory.mp4" \
  8101

start_stream "Mask Machine" \
  "https://bucket.zapdoslabs.com/mask_machine.mp4" \
  8102

start_stream "Mask Production Line" \
  "https://bucket.zapdoslabs.com/mask_production_line.mp4" \
  8103

start_stream "Steel Work Production" \
  "https://bucket.zapdoslabs.com/steel_work_producion.mp4" \
  8104

echo "All streams started."
wait
