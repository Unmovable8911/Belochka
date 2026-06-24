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

## Architecture

```
Browser ──WebSocket──► Hub ──broadcasts──► React state (useReducer)
                        ▲
               Collector(s) ──SSH──► Remote Linux servers
                        │
                      Parser (CPU, memory, disk, network, processes)
```

### Backend (Go)

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Entrypoint, signal handling, graceful shutdown sequence |
| `internal/api` | Chi router, REST endpoints (`/api/servers/*`, `/api/health`), static file serving |
| `internal/hub` | WebSocket hub -- client registration, broadcast, connection limits (max 10) |
| `internal/monitor` | Metrics collector (SSH command execution) and parsers (pure functions, string-in struct-out) |
| `internal/ssh` | SSH connection testing, host key verification, auto-reconnect with exponential backoff |
| `internal/store` | SQLite with WAL mode, server CRUD, AES-256-GCM password encryption |
| `internal/config` | YAML config loading with env var overrides |
| `internal/shutdown` | Ordered shutdown sequence with hard timeout |
| `internal/static` | SPA-aware file server for embedded frontend assets |

### Frontend (React + TypeScript)

| Module | Responsibility |
|--------|---------------|
| `WebSocketProvider` | Single WebSocket connection, message dispatch, exponential backoff reconnect |
| `useMonitorState` | `useReducer`-based state management for servers, metrics, and connection status |
| `Dashboard` | Server card grid with live metrics, empty state guidance |
| `ServerDetail` | System info, CPU/memory ring gauges, disk partitions, network interfaces, process table |
| `ServerCard` | Individual server card with CPU, memory, disk, network summaries |
| `AddServerDialog` / `EditServerDialog` / `DeleteServerDialog` | Server management dialogs |
| `ConnectionBanner` / `StaleDataOverlay` | Disconnection UX (banner + dimmed stale data) |
| `lib/format` | `formatBytes`, `formatNetworkSpeed`, `formatPercent`, `formatUptime`, `getUsageColor` |

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

### Project Layout

```
belochka/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── api/             # HTTP routes and handlers
│   ├── config/          # Configuration loading
│   ├── hub/             # WebSocket hub
│   ├── model/           # Domain types (Server, Metrics, etc.)
│   ├── monitor/         # Metrics collection and parsing
│   ├── shutdown/        # Graceful shutdown orchestration
│   ├── ssh/             # SSH client, reconnection, keepalive
│   ├── static/          # Embedded SPA file server
│   └── store/           # SQLite persistence and encryption
├── web/
│   ├── src/
│   │   ├── components/  # React components
│   │   ├── hooks/       # State management hooks
│   │   ├── lib/         # Utility functions
│   │   └── pages/       # Dashboard and ServerDetail pages
│   └── embed.go         # go:embed directive for production build
├── Makefile
└── belochka.yaml        # Optional config file
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check (`{"status":"ok"}`) |
| `GET` | `/api/servers` | List all servers |
| `POST` | `/api/servers` | Create a server |
| `GET` | `/api/servers/{id}` | Get a server by ID |
| `PUT` | `/api/servers/{id}` | Update a server |
| `DELETE` | `/api/servers/{id}` | Delete a server |
| `POST` | `/api/servers/{id}/test` | Test SSH connection |
| `GET` | `/api/ws` | WebSocket endpoint for live metrics |

## License

MIT
