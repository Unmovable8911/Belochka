<p align="center">
  <img src="./logo.png" width="500">
</p>
<p align="center">
<a href="./docs/README_DE.md">Deutsch</a> / 
English / 
<a href="./docs/README_ES.md">Español</a> / 
<a href="./docs/README_FR.md">Français</a> / 
<a href="./docs/README_IT.md">Italiano</a> / 
<a href="./docs/README_PT.md">Português</a> / 
<a href="./docs/README_RU.md">Русский</a> / 
<a href="./docs/README_CN.md">中文</a> / 
<a href="./docs/README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, "squirrel") is a single-binary server monitoring tool for small fleets of Linux servers. It maintains persistent SSH connections to 5–20 remote machines and streams real-time CPU, memory, disk, network, and process metrics to a browser dashboard over WebSocket. It also provides a web-based interactive terminal for direct SSH access — no separate SSH client needed.

## Features

- **Server groups** — organize servers into flat root-level groups in the sidebar; create, rename, and delete groups, and move servers in or out of a group via drag-and-drop or the "Move to..." menu; click a group to filter the dashboard to its members, with breadcrumb navigation
- **Real-time dashboard** — server cards with live CPU, memory, disk, and network metrics, color-coded by usage; right-click a card for quick Edit / Delete / Console actions
- **Detailed server view** — per-core CPU gauges, memory/swap ring charts, disk partition breakdown, network interface throughput
- **Process management** — dedicated Processes tab with a flat sortable table (user-resizable columns), multi-keyword search, auto-refresh toggle, and kill with SIGTERM/SIGKILL signal selection (sshd/init/systemd protected)
- **Web terminal** — full interactive SSH console in the browser via xterm.js
- **System tray icon** — on desktop machines (Windows, macOS, Linux with GNOME/KDE/XFCE), shows a tray icon with **Open Dashboard** and **Quit** menu items; automatically falls back to CLI mode on headless servers
- **Authentication** — password + session cookie protection; first visit walks through a two-step setup wizard (choose language → set password with strength meter and confirmation); rate limiting after 10 failed login attempts (30‑minute lockout)
- **Single binary** — Go backend with embedded React frontend; one file to deploy, nothing else to install
- **Persistent SSH connections** — automatic reconnection with exponential backoff and keepalive; test the connection before saving a server, and verify the host-key fingerprint when adding new machines (trust on first use)
- **Browser-based key upload** — upload SSH private key files directly through the UI; keys are validated, stored with UUID names, and orphan files are automatically cleaned up
- **Encrypted credential storage** — server passwords encrypted at rest with AES-256-GCM
- **Cron job management** — view, add, edit, enable/disable, delete, and run cron jobs directly from the server detail page
- **Batch command** — write a multi-line script and dispatch it to any set of servers from the sidebar; each server's terminal output streams live into the dialog, you can answer interactive prompts, and cancel the whole run at any time
- **Persistent log file** — all output written to `belochka.log` next to the binary (or in the current working directory when run via `go run`) with automatic retention-based cleanup (default: 3 days)
- **Multi-language UI** — English, Simplified Chinese, French, Russian, German, Spanish, Portuguese, Traditional Chinese, and Italian; selectable during first-run setup and switchable from the Settings dialog; browser-detected language pre-selected
- **In-app settings** — configure port, data directory, language, and log retention directly from the dashboard via a gear icon; no config file editing required

## Quick Start

Download the latest binary from [Releases](https://github.com/Unmovable8911/Belochka/releases), then run it:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64-bit)
belochka-windows-x86-64.exe
```

Open `http://localhost:53136` in your browser. On first visit you will be prompted to choose your language and set a password — this protects the dashboard and all API endpoints. After setup you are automatically logged in. Add servers through the UI.

## Build from Source

Requires Go 1.25+ and Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Cross-compile release binaries for all platforms:

```bash
make release
# Outputs:
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Configuration

Belochka works out of the box with no configuration. All settings are available through the **Settings dialog** (gear icon in the dashboard header). A `config.json` with default values is automatically created next to the binary on first run. You can also pass `--config path/to/config.json` for a custom location:

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Field | Default | Description |
|---|---|---|
| `port` | `53136` | HTTP listen port |
| `data_dir` | `./data` | Database, encryption key, and uploaded SSH key files (`data/keys/`) |
| `language` | `""` | UI language (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`); auto-detected on first visit if empty |
| `log_path` | `""` | Log file path; defaults to `belochka.log` next to the binary if empty |
| `log_retention_days` | `3` | Number of days to keep log entries |

Changes to `port` and `data_dir` require a restart; `language` and `log_retention_days` apply immediately via the Settings dialog.

### Flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to the JSON configuration file |
| `--no-tray` | Disable the system tray icon; run as a plain CLI process |
| `--version` | Print version and exit |

### Environment Variables

| Variable | Description |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | AES-256 encryption key for stored passwords; auto-generated on first run if not set |

### Encryption Key

On first run without a key set, Belochka auto-generates one at `{data_dir}/encryption.key` and logs a warning. For production, set the key explicitly via the environment variable.
