<p align="center">
  <img src="./logo.png" width="500">
</p>
<p align="center">
English / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_RU.md">Русский</a>
</p>
<hr>

Belochka (белочка, "squirrel") is a single-binary server monitoring tool for small fleets of Linux servers. It maintains persistent SSH connections to 5–20 remote machines and streams real-time CPU, memory, disk, network, and process metrics to a browser dashboard over WebSocket. It also provides a web-based interactive terminal for direct SSH access — no separate SSH client needed.

## Features

- **Real-time dashboard** — server cards with live CPU, memory, disk, and network metrics, color-coded by usage
- **Detailed server view** — per-core CPU gauges, memory/swap ring charts, disk partition breakdown, network interface throughput, sortable process table
- **Web terminal** — full interactive SSH console in the browser via xterm.js
- **System tray icon** — on desktop machines (Windows, macOS, Linux with GNOME/KDE/XFCE), shows a tray icon with **Open Dashboard** and **Quit** menu items; automatically falls back to CLI mode on headless servers
- **Single binary** — Go backend with embedded React frontend; one file to deploy, nothing else to install
- **Persistent SSH connections** — automatic reconnection with exponential backoff and keepalive
- **Encrypted credential storage** — server passwords encrypted at rest with AES-256-GCM
- **Cron job management** — view, add, edit, enable/disable, delete, and run cron jobs directly from the server detail page
- **Persistent log file** — all output written to a log file in your user cache directory (e.g. `~/.cache/belochka/belochka.log`) with automatic retention-based cleanup (default: 3 days)
- **Multi-language UI** — English, Chinese, French, and Russian; language auto-detected on first visit and switchable from the Settings dialog
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

Open `http://localhost:53136` in your browser. Add servers through the UI.

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

Belochka works out of the box with no configuration. All settings are available through the **Settings dialog** (gear icon in the dashboard header). You can also create a `config.json` in the working directory or pass `--config path/to/config.json`:

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
| `data_dir` | `./data` | Database and encryption key location |
| `language` | `""` | UI language (`en`, `zh`, `fr`, `ru`); auto-detected on first visit if empty |
| `log_path` | `""` | Log file path; defaults to `~/.cache/belochka/belochka.log` if empty |
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
