# Codebase Overview

## Project Summary
- **Description**: Single-binary Go+React web app for managing 5-20 remote Linux servers via persistent SSH connections. Streams CPU/memory/disk/network/process metrics to a browser dashboard over WebSocket, and provides a web-based interactive terminal (SSH console) for direct server access.
- **Tech Stack**: Go 1.25, React 19, TypeScript 6, Vite 8, Tailwind CSS 4, chi router, gorilla/websocket, modernc.org/sqlite, Radix UI, Lucide icons
- **Entry Points**: `cmd/server/main.go` (Go backend), `web/src/main.tsx` (React frontend)

## Directory Structure
```
cmd/server/          — CLI entry point: flags (--config, --no-tray, --version), logging init, tray/CLI mode branch
assets/              — Embedded tray icon: icon.png (128×128, Linux/macOS) + icon.ico (Windows), selected by build tag (icon_other.go / icon_windows.go)
internal/app/        — Application lifecycle: wires all components, Start/Shutdown orchestration
internal/api/        — HTTP router, REST endpoints (server CRUD), health check
internal/hub/        — WebSocket hub: client registry, broadcast, connection upgrade
internal/broadcast/  — Assembles server info + metrics into JSON wire format for WS clients
internal/logging/    — Persistent log file writer; tee mode (CLI) mirrors output to stdout; retention-based cleanup
internal/monitor/    — Metrics collection: SSH command builder, /proc parsers, delta computation
internal/cron/       — Crontab parsing and line building utilities
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
web/src/components/  — UI components: ServerCard, AddServerDialog, AddCronDialog, WebSocketProvider, Layout, LanguageSwitcher, RingGauge, etc.
web/src/hooks/       — useMonitorState: WebSocket message state management
web/src/api/         — REST API client (client.ts)
web/src/types/       — TypeScript type definitions for server/metrics wire format
web/src/lib/         — Utilities: formatting, WebSocket reconnection, cn() helper
web/src/i18n/        — Internationalization: i18next config and translation JSON files (en, zh, fr, ru)
```

## Modules

### cmd/server
- **Purpose**: CLI entry point. Parses flags (`--config`, `--no-tray`, `--version`), initialises `internal/logging`, creates and starts the Application, then branches: tray mode (`hasDesktop() && !--no-tray`) gives the main goroutine to `systray.Run`; CLI mode waits for SIGTERM/SIGINT and calls graceful shutdown.
- **Key Files**: `cmd/server/main.go` (also `logFilePath()` → `os.UserCacheDir()/belochka/belochka.log`), `cmd/server/tray.go` (systray entry, openBrowser — reaps child via `cmd.Wait()`), `cmd/server/desktop.go` (hasDesktop — Linux/BSD, checks DISPLAY/WAYLAND_DISPLAY), `cmd/server/desktop_darwin.go` (hasDesktop — macOS, always true), `cmd/server/desktop_windows.go` (hasDesktop — Windows, always true)
- **Dependencies**: config, app, logging, assets, fyne.io/systray
- **Exposes**: `main()` — the compiled binary

### assets
- **Purpose**: Embeds the system tray icon via `//go:embed`, selected per platform by build tag — PNG on Linux/macOS, ICO on Windows (systray requires ICO on Windows).
- **Key Files**: `assets/icon_other.go` (`!windows`, embeds icon.png), `assets/icon_windows.go` (`windows`, embeds icon.ico), `assets/icon.png` (128×128), `assets/icon.ico`
- **Dependencies**: none
- **Exposes**: `Icon []byte`

### internal/logging
- **Purpose**: Persistent log file writer that satisfies `io.Writer` for `slog.NewTextHandler`. In tee mode (CLI), mirrors output to a secondary writer (stdout). Retention cleanup runs once at construction and at most hourly on `Write` (off the hot path): if the first log line predates the retention window, the file is rewritten dropping expired lines (uses `bufio.Reader`, so no line-length cap; an unparseable first line falls through to a full scan instead of disabling cleanup). Default retention: 3 days; overridden by `BELOCHKA_LOG_RETENTION_DAYS` env var.
- **Key Files**: `internal/logging/logger.go`
- **Dependencies**: none
- **Exposes**: `Logger` struct, `New(path string, tee bool) (*Logger, error)`

### internal/app
- **Purpose**: Top-level application container. Wires hub, store, SSH pool, collector manager, terminal handler, and HTTP server. Manages lifecycle (Start/Shutdown) and periodic metric broadcast loop (2s interval).
- **Key Files**: `internal/app/app.go`
- **Dependencies**: api, broadcast, config, hub, model, monitor, shutdown, ssh, store, terminal, web (wires `pool` as `CronExecutor` and `CronRunner`)
- **Exposes**: `Application` struct with `New()`, `Start()`, `Shutdown()`, `Addr()`

### internal/api
- **Purpose**: HTTP routing and REST API handlers. Mounts server CRUD endpoints, a stateless connection-test endpoint, health check, WebSocket upgrade, terminal WebSocket endpoint, cron CRUD endpoints, and static file serving. Cron handlers delegate the crontab read-modify-write workflow to `cron.Service` (constructed from the injected `cron.Executor` in `NewRouter`); they only parse requests and map `cron.ErrCronIndexOutOfRange` → 404 and SSH errors → 502.
- **Key Files**: `internal/api/router.go`, `internal/api/server_handler.go`, `internal/api/cron_handler.go`
- **Dependencies**: hub, model, ssh, static, terminal, cron
- **Exposes**: `NewRouter()` with functional options (`WithServerStore`, `WithSSHTester`, `WithStaticFS`, `WithOnServerChange`, `WithTerminalHandler`, `WithCronExecutor`, `WithCronRunner`); `ServerStore`, `SSHTester`, `CronRunner` interfaces (`WithCronExecutor` accepts a `cron.Executor`). Routes: `GET/POST /api/servers/{id}/crons`, `PUT/DELETE /api/servers/{id}/crons/{index}`, `POST /api/servers/{id}/crons/{index}/run`.

### internal/cron
- **Purpose**: Crontab parsing/building plus the read-modify-write orchestration over SSH. `cron.go` parses crontab output into enabled/disabled entries and passthrough lines (comments, env vars), builds lines with the `#[disabled] ` prefix convention, and provides `ReplaceCronEntry` for in-place replacement/deletion by zero-based index. `service.go` holds a `Service` that reads (`crontab -l 2>/dev/null || true`) and writes (base64 → `crontab -`) the remote crontab, encapsulating the full List/Create/Update/Delete workflow so the API layer carries no shell/base64 details.
- **Key Files**: `internal/cron/cron.go` (parsing/building), `internal/cron/service.go` (Service: SSH read-modify-write orchestration)
- **Dependencies**: none (Service depends on an injected `Executor` interface, satisfied by `ssh.Pool`)
- **Exposes**: `CronEntry` (schedule fields + command + enabled flag), `CronResult` (entries + passthroughs), `ParseCrontab()`, `BuildLine()`, `BuildCronLine()`, `ReplaceCronEntry()`; `Service` with `NewService()`, `List()`, `Create()`, `Update()`, `Delete()`; `Executor` interface; `ErrCronIndexOutOfRange` sentinel (matched via `errors.Is` in `api` → HTTP 404)

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
- **Exposes**: `Pool` (Add/Remove/Execute/OpenSession/RunCommand/Status/TriggerReconnect/CloseAll), `TestConnection()`, `TestResult`, `ConnectionError`, `ErrorKind`, `ConnState`, `ConnStatus`, `ServerProvider` interface. `RunCommand(ctx, serverID, cmd)` runs a single command and returns stdout+stderr combined, exit code, and error (connection failures distinguished from non-zero exit codes via `*gossh.ExitError`).

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
- **Purpose**: React SPA dashboard. Three routes: `/` (Dashboard — server card grid with connection status and summary metrics), `/server/:id` (ServerDetail — detailed CPU, memory, disk, network, process views, and Cron Jobs tab with full CRUD + run-now), and `/server/:id/console` (Console — full-page web terminal via xterm.js + WebSocket). Dashboard and detail routes use the shared Layout/WebSocketProvider; console route is standalone. Internationalized with react-i18next supporting English, Chinese (Simplified), French, and Russian.
- **Key Files**: `web/src/App.tsx` (routes, Layout wrapper), `web/src/pages/Dashboard.tsx`, `web/src/pages/ServerDetail.tsx` (tabbed: Overview / Cron Jobs — overview renders inline, cron tab delegates to `CronJobsTab`), `web/src/pages/Console.tsx` (web terminal page), `web/src/components/CronJobsTab.tsx` (cron tab UI: table, toggle, run/edit/delete, confirm + run-output dialogs; driven by `useCrons`), `web/src/components/AddServerDialog.tsx` + `web/src/components/EditServerDialog.tsx` (each owns its form data + change-detection, both share the connection-test/fingerprint/save state machine via `useServerForm` and render the shared `ServerForm`), `web/src/components/ServerForm.tsx` (controlled presentational form fields + fingerprint trust block, parameterized by `idPrefix`), `web/src/components/WebSocketProvider.tsx`, `web/src/components/Layout.tsx` (global layout shell), `web/src/components/LanguageSwitcher.tsx`, `web/src/components/AddCronDialog.tsx` (add/edit cron dialog, reused via `editEntry`/`editIndex` props), `web/src/i18n/index.ts` (i18n config), `web/src/i18n/en.json` (English translations, reference for all languages), `web/src/hooks/useMonitorState.ts`, `web/src/hooks/useCrons.ts` (cron fetch + mutation logic: lazy fetch, optimistic toggle w/ revert, row-error index shifting), `web/src/hooks/useServerForm.ts` (shared connection-test/fingerprint/save state machine for Add/Edit dialogs), `web/src/api/client.ts`, `web/embed.go` (Go embed)
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
12. **Cron management**: The cron handlers delegate to `cron.Service`, which performs the SSH read-modify-write. `GET /api/servers/{id}/crons` → `Service.List` (`crontab -l` → `ParseCrontab` → JSON). `POST` → `Service.Create` (append line, write back via `echo <base64> | base64 -d | crontab -`). `PUT /{index}` → `Service.Update` (`ReplaceCronEntry` in place); `DELETE /{index}` → `Service.Delete`; out-of-range index returns `ErrCronIndexOutOfRange` → HTTP 404. `POST /{index}/run` resolves the entry via `Service.List` then executes the command via `pool.RunCommand()` and returns `{exitCode, output}`; non-zero exit codes are returned as data, not HTTP errors.

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
