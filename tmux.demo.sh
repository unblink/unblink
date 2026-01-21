#!/bin/bash

# tmux.dev.sh - Development tmux session with relay + node + app

# Source the library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/tmux.lib.sh"

# Configuration
SESSION_NAME="unblink-dev"
PROJECT_DIR="$(tmux_get_project_dir)"

# Main script
if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  echo "Creating and attaching to new tmux session '$SESSION_NAME'."

  # Initialize session (no windows)
  tmux_session_init "$SESSION_NAME"

  # Configure session (mouse, keybindings)
  tmux_configure_session "$SESSION_NAME"

  # Create all windows using the same function
  tmux_create_window "$SESSION_NAME" "relay" "$PROJECT_DIR/relay" "go run ../cmd/relay/main.go"
  tmux_create_window "$SESSION_NAME" "app" "$PROJECT_DIR/app" "bun dev"
  tmux_create_window "$SESSION_NAME" "node" "$PROJECT_DIR" "sleep 8 && go run ./cmd/node/main.go"
  tmux_create_window "$SESSION_NAME" "demo" "$PROJECT_DIR" "sleep 8 && bash ./start_demo_streams.sh"

  # Attach to relay window
  tmux_session_attach "$SESSION_NAME" "relay"
else
  echo "Attaching to existing tmux session '$SESSION_NAME'."
  tmux attach-session -t "$SESSION_NAME"
fi
