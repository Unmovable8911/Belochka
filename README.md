# Belochka

Belochka (белочка, "squirrel") is a lightweight server monitoring tool that ships as a single binary. It connects to 5-20 remote Linux servers over persistent SSH sessions and streams CPU, memory, disk, network, and process metrics to a real-time browser dashboard via WebSocket.

## Features

- **Real-time dashboard** -- server cards with live CPU, memory, disk, and network metrics, color-coded by usage thresholds
- **Server detail pages** -- per-core CPU gauges, memory/swap ring charts, disk partition breakdown, network interface throughput, sortable process table
- **Persistent SSH connections** -- monitors servers continuously with automatic reconnection and exponential backoff
- **Single binary deployment** -- Go backend with embedded React frontend; one file to deploy, nothing else to install
- **Zero-config startup** -- runs with sensible defaults; optional YAML config file for customization
- **Encrypted credential storage** -- server passwords encrypted at rest with AES-256-GCM
- **Graceful shutdown** -- ordered teardown of HTTP, WebSocket, collectors, and SSH connections with WAL checkpoint
- **Connection testing** -- test SSH connectivity and verify host key fingerprints before saving a server

## Requirements

- **Go** 1.21+
- **Node.js** 18+ and npm (for building the frontend)
- Monitored servers must be reachable via SSH

## Quick Start

```bash
# Clone and build
git clone <repo-url> && cd belochka
make build

# Run (zero-config -- uses defaults)
./bin/belochka
```

The dashboard is available at `http://localhost:53136`. Add servers through the UI.

## Configuration

Belochka works out of the box with no configuration. Optionally, create a `belochka.yaml` in the working directory or pass `--config path/to/config.yaml`:

```yaml
port: 53136           # HTTP listen port (default: 53136)
data_dir: ./data      # SQLite database and encryption key location (default: ./data)
encryption_key: ""    # Hex-encoded AES-256 key; leave empty to auto-generate
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BELOCHKA_ENCRYPTION_KEY` | Overrides `encryption_key` from the config file |

### Encryption Key

On first run, if no encryption key is configured, Belochka auto-generates one at `{data_dir}/encryption.key` and logs a warning. For production, set the key via the config file or environment variable and store it securely.

## Development

```bash
# Start backend (port 53136)
make dev-backend

# Start frontend dev server with hot reload (port 53137, proxies /api to backend)
make dev-frontend
```

### Running Tests

```bash
# Go tests
go test ./...

# Frontend tests
cd web && npx vitest run
```