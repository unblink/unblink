<p align="center">
<img width="300" src="assets/logo.svg">
</p>

[![GitHub Stars](https://img.shields.io/github/stars/unblink/unblink?style=flat)](https://github.com/unblink/unblink/stargazers)
[![Discord](https://img.shields.io/badge/Discord-Join%20Server-5865F2?style=flat&logo=discord&logoColor=white)](https://discord.gg/34jpgpW9Hy)

# Unblink

Unblink is an AI camera monitoring application.

# Get started

## Run the Relay

The relay is the public-facing server that coordinates connections between nodes and clients.

```bash
# From the unblink directory
cd relay
go run ../cmd/relay
```

The relay will:

- Start WebSocket server on `:9020` for node connections
- Start HTTP API on `:8020` for client requests
- Create a SQLite database at `data/database/relay.db`

### Configuration

Copy from `.env.example` to create your `.env` file:

## Run the Node

The node runs in your private network and forwards traffic from the relay to your local cameras.

```bash
# From the unblink directory
go run ./cmd/node
```

The node will:

- Connect to the relay at `ws://localhost:9020/node/connect`
- Load config from `~/.unblink/config.json`
- Create bridges to local services on demand

### Configuration

The node creates a config file at `~/.unblink/config.json` on first run. Edit it:

```json
{
  "relay_address": "ws://localhost:9020/node/connect",
  "node_id": "my-node",
  "reconnect": {
    "enabled": true,
    "max_num_attempts": 10
  }
}
```

### Authorization Flow

On first run, the node will request authorization. The relay will provide an authorization URL that you can open in your browser to authorize the node with your account. The node will then receive a token and connect automatically on subsequent runs.

### Relay

Public traffic router and multiplexer. The relay:

- Is publicly reachable
- Manages nodes and clients via Cap'n Proto RPC
- Creates and multiplexes bidirectional data streams
- Handles user authentication with JWT tokens
- Stores configuration in SQLite database

### Node

Private proxy that runs in your private network. The node:

- Maintains one persistent WebSocket connection to the relay
- Implements Cap'n Proto RPC server for relay callbacks
- Opens TCP connections to local services on demand (cameras, RTSP, etc.)
- Forwards data via bidirectional streaming without inspection

### Protocol

See [docs/NODE_RELAY_PROTOCOL.md](docs/NODE_RELAY_PROTOCOL.md) for detailed protocol specifications.

## API Endpoints

### Authentication

```bash
# Register new user
POST /auth/register
{
  "email": "user@example.com",
  "password": "password123",
  "name": "John Doe"
}

# Login
POST /auth/login
{
  "email": "user@example.com",
  "password": "password123"
}

# Get current user
GET /auth/me
Authorization: Bearer <token>

# Logout
POST /auth/logout
```

## Development

### Build from source

```bash
# Build relay
./tmux.dev.sh
```

### Run tests

```bash
go test ./tests/...
```

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.
