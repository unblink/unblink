#!/bin/bash

# Set the name for your tmux session
SESSION_NAME="unblink"

# Check if a tmux session with the EXACT name already exists.
# The '=' forces an exact match, preventing it from matching "unblink_engine".
if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  # Session does NOT exist.
  echo "Creating and attaching to new tmux session '$SESSION_NAME'."

  # Create a new session with first window running the relay
  tmux new-session -s $SESSION_NAME -n relay -d

  # Run the relay in the first pane
  tmux send-keys -t $SESSION_NAME:0 "cd $(pwd)/relay && go run ./cmd/relay" C-m

  # Create a second window for the worker
  tmux new-window -t $SESSION_NAME:1 -n worker

  # Run the base-vl worker using uv
  tmux send-keys -t $SESSION_NAME:1 "cd $(pwd)/examples/worker-base-vl && uv run main.py" C-m

  # Split the worker window horizontally (optional, for logs/debugging)
  tmux split-window -t $SESSION_NAME:1 -h -p 50

  # Attach to the relay window initially
  tmux select-window -t $SESSION_NAME:0
  tmux attach-session -t $SESSION_NAME
else
  # Session DOES exist.
  echo "Attaching to existing tmux session '$SESSION_NAME'."

  # Attach to the existing session.
  tmux attach-session -t $SESSION_NAME
fi
