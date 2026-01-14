#!/bin/bash

# tmux.prod.sh - Production tmux session with relay + base-vl worker

# Source the library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/tmux.lib.sh"

# Configuration
SESSION_NAME="unblink-prod"
PROJECT_DIR="$(tmux_get_project_dir)"

# Main script
if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  echo "Creating and attaching to new tmux session '$SESSION_NAME'."

  # Initialize session (no windows)
  tmux_session_init "$SESSION_NAME"

  # Configure session (mouse, keybindings)
  tmux_configure_session "$SESSION_NAME"

  # Create all windows using the same function
  tmux_create_window "$SESSION_NAME" "relay" "$PROJECT_DIR/relay" "go run ./cmd/relay"
  tmux_create_window "$SESSION_NAME" "worker_base_vl" "$PROJECT_DIR/examples/worker-base-vl" "sleep 3 && uv run main.py"

  # Attach to relay window
  tmux_session_attach "$SESSION_NAME" "relay"
else
  echo "Attaching to existing tmux session '$SESSION_NAME'."
  tmux attach-session -t "$SESSION_NAME"
fi
