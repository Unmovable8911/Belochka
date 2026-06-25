# Codebase Overview

## Project Summary
- **Description**: Single-binary Go+React web app that monitors 5-20 remote Linux servers via persistent SSH connections, streaming CPU/memory/disk/network/process metrics to a browser dashboard over WebSocket.
- **Tech Stack**: Go 1.25, React 19, TypeScript 6, Vite 8, Tailwind CSS 4, chi router, gorilla/websocket, modernc.org/sqlite, Radix UI, Lucide icons
- **Entry Points**: `cmd/server/main.go` (Go backend), `web/src/main.tsx` (React frontend)

## Directory Structure
```
cmd/server/          — CLI entry point: loads config, creates Application, runs until signal
internal/app/        — Application lifecycle: wires all components, Start/Shutdown orchestration
internal/api/        — HTTP router, REST endpoints (server CRUD), health check
internal/hub/        — WebSocket hub: client registry, broadcast, connection upgrade
internal/broadcast/  — Assembles server info + metrics into JSON wire format for WS clients
internal/monitor/    — Metrics collection: SSH command builder, /proc parsers, delta computation
internal/ssh/        — SSH connection pool, reconnection with exponential backoff, keepalive
internal/store/      — SQLite persistence with AES-encrypted password storage
internal/model/      — Domain types: Server, Metrics, Snapshot, CPU/Memory/Disk/Network/Process
internal/config/     — YAML config loading with env var override
internal/clock/      — Clock interface for deterministic testing
internal/shutdown/   — Ordered graceful shutdown sequence
internal/static/     — SPA file server with index.html fallback
web/                 — React frontend (Vite build), embedded into Go binary via embed.go
web/src/pages/       — Dashboard (server grid) and ServerDetail (single server view)
web/src/components/  — UI components: ServerCard, AddServerDialog, WebSocketProvider, Layout, RingGauge, etc.
web/src/hooks/       — useMonitorState: WebSocket message state management
web/src/api/         — REST API client (client.ts)
web/src/types/       — TypeScript type definitions for server/metrics wire format
web/src/lib/         — Utilities: formatting, WebSocket reconnection, cn() helper
web/src/i18n/        — Internationalization: i18next config and translation JSON files (en, zh, fr, ru)
```

## Modules

### cmd/server
- **Purpose**: CLI entry point. Parses flags, loads config, creates and starts the Application, waits for SIGTERM/SIGINT, then calls graceful shutdown.
- **Key Files**: `cmd/server/main.go`
- **Dependencies**: config, app
- **Exposes**: `main()` — the compiled binary

### internal/app
- **Purpose**: Top-level application container. Wires hub, store, SSH pool, collector manager, and HTTP server. Manages lifecycle (Start/Shutdown) and periodic metric broadcast loop (2s interval).
- **Key Files**: `internal/app/app.go`
- **Dependencies**: api, broadcast, config, hub, model, monitor, shutdown, ssh, store, web
- **Exposes**: `Application` struct with `New()`, `Start()`, `Shutdown()`, `Addr()`

### internal/api
- **Purpose**: HTTP routing and REST API handlers. Mounts server CRUD endpoints, health check, WebSocket upgrade, and static file serving.
- **Key Files**: `internal/api/router.go`, `internal/api/server_handler.go`
- **Dependencies**: hub, model, ssh, static
- **Exposes**: `NewRouter()` with functional options (`WithServerStore`, `WithSSHTester`, `WithStaticFS`, `WithOnServerChange`); `ServerStore` and `SSHTester` interfaces

### internal/hub
- **Purpose**: WebSocket client management. Handles upgrade, registration, broadcast fan-out, connection limits (max 10), and graceful close with 1001 Going Away frame.
- **Key Files**: `internal/hub/hub.go`
- **Dependencies**: gorilla/websocket
- **Exposes**: `Hub` struct with `New()`, `Run()`, `ServeWS()`, `BroadcastMsg()`, `SetSnapshot()`, `ClientCount()`; `Envelope` wire format

### internal/broadcast
- **Purpose**: Assembles server connection state and metric snapshots into the JSON wire format sent to WebSocket clients.
- **Key Files**: `internal/broadcast/broadcast.go`, `internal/broadcast/wire.go`
- **Dependencies**: model
- **Exposes**: `Assemble()` function, `ServerInfo` struct

### internal/monitor
- **Purpose**: Metrics collection engine. Builds a combined SSH command that reads /proc/stat, /proc/meminfo, df, /proc/net/dev, top, hostname, uname, uptime, os-release, nproc in a single exec. Parses output into domain types. Computes CPU usage deltas and network rates between consecutive readings. Manager coordinates per-server Collector goroutines.
- **Key Files**: `internal/monitor/collector.go` (Collector, Manager, delta computation), `internal/monitor/parser.go` (ParseCPU, ParseMemory, ParseDisk, ParseNetwork, ParseProcesses, ParseSystemInfo)
- **Dependencies**: clock, model
- **Exposes**: `Manager` (Add/Remove/Latest/StopAll), `Collector`, `SSHExecutor` interface, `CollectCommand()`, `ParseCombinedOutput()`, individual parsers, `ComputeCPUUsage()`, `ComputeNetworkRates()`

### internal/ssh
- **Purpose**: Persistent SSH connection pool with automatic reconnection (exponential backoff 1s→30s), keepalive pings (30s interval, 3-failure threshold), and classified error types (auth, host key, network, passphrase).
- **Key Files**: `internal/ssh/pool.go` (Pool, managed connections), `internal/ssh/reconnect.go` (Reconnector, Keepalive), `internal/ssh/ssh.go` (TestConnection, auth builder, error classification)
- **Dependencies**: clock, model, golang.org/x/crypto/ssh
- **Exposes**: `Pool` (Add/Remove/Execute/Status/TriggerReconnect/CloseAll), `TestConnection()`, `TestResult`, `ConnectionError`, `ErrorKind`, `ConnState`, `ConnStatus`, `ServerProvider` interface

### internal/store
- **Purpose**: SQLite-based server persistence. Passwords are AES-GCM encrypted at rest. Supports auto-generated or config-provided encryption key. WAL mode enabled.
- **Key Files**: `internal/store/store.go` (Open, CRUD), `internal/store/crypto.go` (AES-GCM encrypt/decrypt)
- **Dependencies**: model, modernc.org/sqlite, google/uuid
- **Exposes**: `SQLiteStore` with `Open()`, `Create()`, `GetByID()`, `List()`, `Update()`, `Delete()`, `Close()`

### internal/model
- **Purpose**: Shared domain types used across all backend modules. Raw metrics (jiffy counters, byte counters) and computed snapshots (percentages, rates).
- **Key Files**: `internal/model/model.go`
- **Dependencies**: none
- **Exposes**: `Server`, `AuthType`, `Metrics`, `CPUMetrics`, `CPUCore`, `MemoryMetrics`, `DiskMetrics`, `DiskPartition`, `NetworkMetrics`, `NetworkInterface`, `Process`, `ProcessMetrics`, `SystemInfo`, `Snapshot`, `CPUUsage`, `NetworkRate`

### internal/config
- **Purpose**: Loads YAML config file (default `belochka.yaml`), falls back to built-in defaults (port 53136, data dir `./data`). `BELOCHKA_ENCRYPTION_KEY` env var overrides file value.
- **Key Files**: `internal/config/config.go`
- **Dependencies**: gopkg.in/yaml.v3
- **Exposes**: `Config` struct, `Load()` function

### internal/clock
- **Purpose**: Abstraction over `time.Now()`, `time.NewTicker()`, and context-aware `Sleep()` for deterministic testing.
- **Key Files**: `internal/clock/clock.go`, `internal/clock/fake.go`
- **Dependencies**: none
- **Exposes**: `Clock` interface, `Ticker` interface, `Real` implementation

### internal/shutdown
- **Purpose**: Ordered graceful shutdown with a hard timeout. Steps execute sequentially; failures are collected but don't stop subsequent steps.
- **Key Files**: `internal/shutdown/shutdown.go`
- **Dependencies**: none
- **Exposes**: `Sequence` with `NewSequence()`, `Add()`, `Run()`

### internal/static
- **Purpose**: Serves embedded frontend assets with SPA fallback (unknown paths serve index.html). Returns nil handler when no FS is provided (dev mode).
- **Key Files**: `internal/static/handler.go`
- **Dependencies**: none
- **Exposes**: `NewHandler()` function

### web (frontend)
- **Purpose**: React SPA dashboard. Two routes: `/` (Dashboard — server card grid with connection status and summary metrics) and `/server/:id` (ServerDetail — detailed CPU, memory, disk, network, process views). Connects via WebSocket for real-time updates; falls back with reconnection logic. Internationalized with react-i18next supporting English, Chinese (Simplified), French, and Russian.
- **Key Files**: `web/src/App.tsx` (routes, Layout wrapper), `web/src/pages/Dashboard.tsx`, `web/src/pages/ServerDetail.tsx`, `web/src/components/WebSocketProvider.tsx`, `web/src/components/Layout.tsx` (global layout with language switcher), `web/src/i18n/index.ts` (i18n config), `web/src/i18n/en.json` (English translations, reference for all languages), `web/src/hooks/useMonitorState.ts`, `web/src/api/client.ts`, `web/embed.go` (Go embed)
- **Dependencies**: React 19, react-router-dom, Radix UI, Tailwind CSS, Lucide icons, sonner (toasts), i18next, react-i18next, i18next-browser-languagedetector
- **Exposes**: Embedded filesystem via `web.DistFS()` consumed by Go backend

## Data Flow
1. **Config load**: `cmd/server/main.go` reads `belochka.yaml` (or defaults) via `config.Load()`.
2. **App init**: `app.New()` opens SQLite store, creates Hub, SSH Pool (backed by store as ServerProvider), and Monitor Manager (backed by pool as SSHExecutor).
3. **Server sync**: On start and after any CRUD operation, `syncServers()` reconciles Pool and Manager with the current server list from the store.
4. **SSH connections**: Pool maintains a persistent SSH connection per server with automatic reconnection (exponential backoff) and keepalive pings.
5. **Metric collection**: Each server's Collector runs a 2s loop: executes a combined shell command over SSH → parses /proc output → computes CPU/network deltas from previous reading → stores latest Snapshot.
6. **Broadcast loop**: Every 2s, `broadcastAll()` gathers all server states from Pool and latest Snapshots from Manager, assembles them into JSON via `broadcast.Assemble()`, and pushes to all WebSocket clients via Hub.
7. **WebSocket delivery**: Hub fans out the `{"type":"snapshot","data":...}` envelope to all connected browser clients. New clients receive the cached snapshot immediately on connect.
8. **Frontend rendering**: `WebSocketProvider` receives messages → `useMonitorState` hook updates React state → Dashboard/ServerDetail re-render with fresh metrics. All UI strings resolve through `react-i18next` `t()` calls against the active language's translation file.
9. **Server CRUD**: REST API (`POST/GET/PUT/DELETE /api/servers`) persists to SQLite, then triggers `onServerChange` callback which re-syncs SSH pool and broadcasts updated state.

## External Dependencies
- **i18next / react-i18next**: Frontend internationalization framework with React bindings; language auto-detected from browser, persisted in localStorage, switchable via UI
- **i18next-browser-languagedetector**: Automatic language detection from localStorage and navigator
- **gorilla/websocket**: WebSocket server implementation for real-time metric streaming
- **go-chi/chi**: HTTP router with path parameter support
- **modernc.org/sqlite**: Pure-Go SQLite driver (no CGo required)
- **golang.org/x/crypto/ssh**: SSH client connections, key parsing, keepalive
- **google/uuid**: Server ID generation
- **gopkg.in/yaml.v3**: Configuration file parsing
- **Radix UI**: Accessible headless UI primitives (dialogs, buttons, etc.)
- **react-router-dom**: Client-side routing for SPA
- **sonner**: Toast notification component
- **Tailwind CSS 4 + tw-animate-css**: Utility-first CSS framework with animations
