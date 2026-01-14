#!/bin/bash

# tmux.lib.sh - Shared functions for tmux session management
# Source this file in your tmux scripts to use these functions

# Get project root directory
tmux_get_project_dir() {
  echo "$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
}

# Initialize tmux session if it doesn't exist (without creating any windows)
# Usage: tmux_session_init "$SESSION_NAME"
tmux_session_init() {
  local session=$1

  if ! tmux has-session -t "=$session" 2>/dev/null; then
    tmux new-session -s "$session" -d
    return 0
  else
    return 1
  fi
}

# Configure tmux session options
# Usage: tmux_configure_session "$SESSION_NAME"
tmux_configure_session() {
  local session=$1

  # Enable mouse support for clickable tabs and panes
  tmux set-option -g mouse on
  tmux set-option -t "$session" mouse on

  # Bind 'x' to kill current window
  tmux bind-key x kill-window

  # Bind 'K' to kill all tmux sessions with confirmation
  tmux bind-key K confirm-before -p "kill all tmux sessions? (y/n)" "run-shell 'tmux kill-server'"
}

# Create a new window and run a command
# Usage: tmux_create_window "$SESSION" "$WINDOW_NAME" "$WORKING_DIR" "$COMMAND"
tmux_create_window() {
  local session=$1
  local window_name=$2
  local working_dir=$3
  local command=$4

  tmux new-window -t "$session" -n "$window_name"
  tmux send-keys -t "$session:$window_name" "cd \"$working_dir\" && $command" C-m
}

# Attach to session
# Usage: tmux_session_attach "$SESSION_NAME" "$DEFAULT_WINDOW"
tmux_session_attach() {
  local session=$1
  local default_window=${2:-relay}

  tmux select-window -t "$session:$default_window"
  tmux attach-session -t "$session"
}
