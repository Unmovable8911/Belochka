# Codebase Overview

## Project Summary
- **Description**: Single-binary Go+React web app for managing 5-20 remote Linux servers via persistent SSH connections. Streams CPU/memory/disk/network/process metrics to a browser dashboard over WebSocket, and provides a web-based interactive terminal (SSH console) for direct server access.
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
internal/terminal/   — Web terminal: WebSocket-SSH bridge with PTY, resize, session lifecycle
web/                 — React frontend (Vite build), embedded into Go binary via embed.go
web/src/pages/       — Dashboard, ServerDetail, and Console (web terminal) pages
web/src/components/  — UI components: ServerCard, AddServerDialog, WebSocketProvider, Layout, LanguageSwitcher, RingGauge, etc.
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
- **Purpose**: Top-level application container. Wires hub, store, SSH pool, collector manager, terminal handler, and HTTP server. Manages lifecycle (Start/Shutdown) and periodic metric broadcast loop (2s interval).
- **Key Files**: `internal/app/app.go`
- **Dependencies**: api, broadcast, config, hub, model, monitor, shutdown, ssh, store, terminal, web
- **Exposes**: `Application` struct with `New()`, `Start()`, `Shutdown()`, `Addr()`

### internal/api
- **Purpose**: HTTP routing and REST API handlers. Mounts server CRUD endpoints, a stateless connection-test endpoint, health check, WebSocket upgrade, terminal WebSocket endpoint, and static file serving.
- **Key Files**: `internal/api/router.go`, `internal/api/server_handler.go`
- **Dependencies**: hub, model, ssh, static, terminal
- **Exposes**: `NewRouter()` with functional options (`WithServerStore`, `WithSSHTester`, `WithStaticFS`, `WithOnServerChange`, `WithTerminalHandler`); `ServerStore` and `SSHTester` interfaces. The terminal `*terminal.Handler` is created by `internal/app` and injected here (single instance, so `Shutdown` closes the same handler that serves traffic).

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
- **Exposes**: `Pool` (Add/Remove/Execute/OpenSession/Status/TriggerReconnect/CloseAll), `TestConnection()`, `TestResult`, `ConnectionError`, `ErrorKind`, `ConnState`, `ConnStatus`, `ServerProvider` interface

### internal/store
- **Purpose**: SQLite-based server persistence. Passwords are AES-GCM encrypted at rest. Supports auto-generated or config-provided encryption key. WAL mode enabled.
- **Key Files**: `internal/store/store.go` (Open, CRUD), `internal/store/crypto.go` (AES-GCM encrypt/decrypt)
- **Dependencies**: model, modernc.org/sqlite, google/uuid
- **Exposes**: `SQLiteStore` with `Open()`, `Create()`, `GetByID()`, `List()`, `Update()`, `Delete()`, `Close()`

### internal/model
- **Purpose**: Shared domain types used across all backend modules. Raw metrics (jiffy counters, byte counters) and computed snapshots (percentages, rates).
- **Key Files**: `internal/model/model.go`
- **Dependencies**: none
- **Exposes**: `Server`, `AuthType`, `Metrics`, `CPUMetrics`, `CPUCore`, `MemoryMetrics`, `DiskMetrics`, `DiskPartition`, `NetworkMetrics`, `NetworkInterface`, `Process`, `ProcessMetrics`, `SystemInfo`, `Snapshot`, `CPUUsage`, `NetworkRate`; `ErrServerNotFound` sentinel error (wrapped by `store`, matched via `errors.Is` in `api`)

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

### internal/terminal
- **Purpose**: Web terminal session lifecycle: bridges a WebSocket connection to an SSH session with PTY. Handles bidirectional data (binary frames), resize control messages (JSON text frames), connection/disconnection status messages, and session tracking for graceful shutdown.
- **Key Files**: `internal/terminal/terminal.go` (Handler, Session interface, WebSocket-SSH bridge), `internal/terminal/adapter.go` (SSHSessionOpener adapter wrapping gossh.Session)
- **Dependencies**: gorilla/websocket, golang.org/x/crypto/ssh
- **Exposes**: `Handler` (ServeHTTP/CloseAll), `Session` interface, `SessionOpener` interface, `SSHSessionOpener` adapter, `ServerNotFoundError`

### internal/static
- **Purpose**: Serves embedded frontend assets with SPA fallback (unknown paths serve index.html). Returns nil handler when no FS is provided (dev mode).
- **Key Files**: `internal/static/handler.go`
- **Dependencies**: none
- **Exposes**: `NewHandler()` function

### web (frontend)
- **Purpose**: React SPA dashboard. Three routes: `/` (Dashboard — server card grid with connection status and summary metrics), `/server/:id` (ServerDetail — detailed CPU, memory, disk, network, process views), and `/server/:id/console` (Console — full-page web terminal via xterm.js + WebSocket). Dashboard and detail routes use the shared Layout/WebSocketProvider; console route is standalone. Internationalized with react-i18next supporting English, Chinese (Simplified), French, and Russian.
- **Key Files**: `web/src/App.tsx` (routes, Layout wrapper), `web/src/pages/Dashboard.tsx`, `web/src/pages/ServerDetail.tsx`, `web/src/pages/Console.tsx` (web terminal page), `web/src/components/WebSocketProvider.tsx`, `web/src/components/Layout.tsx` (global layout shell), `web/src/components/LanguageSwitcher.tsx` (language switcher used in Dashboard and ServerDetail), `web/src/i18n/index.ts` (i18n config), `web/src/i18n/en.json` (English translations, reference for all languages), `web/src/hooks/useMonitorState.ts`, `web/src/api/client.ts`, `web/embed.go` (Go embed)
- **Dependencies**: React 19, react-router-dom, Radix UI, Tailwind CSS, Lucide icons, sonner (toasts), i18next, react-i18next, i18next-browser-languagedetector, @xterm/xterm, @xterm/addon-fit
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
10. **Terminal session**: Browser opens WebSocket to `/api/ws/terminal/{serverID}` → terminal Handler calls `SessionOpener.OpenSession()` to get an SSH session from Pool → requests PTY (xterm-256color) → starts shell → bridges stdin/stdout bidirectionally as binary WebSocket frames. Resize control messages (JSON text frames) trigger `WindowChange`. On SSH EOF or WebSocket close, session is cleaned up.
11. **Connection test**: `POST /api/servers/test` accepts a full server config in the request body and runs `ssh.TestConnection` without persisting anything (no DB writes, no pool sync). When the password is omitted but an `id` is supplied, the stored secret is read (read-only) and reused. The Add/Edit dialogs test against in-memory form data and persist only on Save, so cancelling never leaves orphaned server records.

## External Dependencies
- **i18next / react-i18next**: Frontend internationalization framework with React bindings; language auto-detected from browser, persisted in localStorage, switchable via UI
- **i18next-browser-languagedetector**: Automatic language detection from localStorage and navigator
- **gorilla/websocket**: WebSocket server implementation for real-time metric streaming
- **go-chi/chi**: HTTP router with path parameter support
- **modernc.org/sqlite**: Pure-Go SQLite driver (no CGo required)
- **golang.org/x/crypto/ssh**: SSH client connections, key parsing, keepalive
- **google/uuid**: Server ID generation
- **gopkg.in/yaml.v3**: Configuration file parsing
- **@xterm/xterm + @xterm/addon-fit**: Terminal emulator for the web console page; fit addon auto-sizes to container
- **Radix UI**: Accessible headless UI primitives (dialogs, buttons, etc.)
- **react-router-dom**: Client-side routing for SPA
- **sonner**: Toast notification component
- **Tailwind CSS 4 + tw-animate-css**: Utility-first CSS framework with animations
