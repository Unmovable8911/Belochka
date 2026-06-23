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
- **Single binary** — Go backend with embedded React frontend; one file to deploy, nothing else to install
- **Persistent SSH connections** — automatic reconnection with exponential backoff and keepalive
- **Encrypted credential storage** — server passwords encrypted at rest with AES-256-GCM
- **Multi-language UI** — English, Chinese, French, and Russian

## Quick Start

Download the latest binary from [Releases](https://github.com/Unmovable8911/Belochka/releases), then run it:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (amd64)
belochka-windows-amd64.exe
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
#   bin/belochka-windows-amd64.exe
```

## Configuration

Belochka works out of the box with no configuration. Optionally create a `belochka.yaml` in the working directory or pass `--config path/to/config.yaml`:

```yaml
port: 53136        # HTTP listen port (default: 53136)
data_dir: ./data   # Database and encryption key location (default: ./data)
encryption_key: "" # AES-256 key; leave empty to auto-generate
```

### Environment Variables

| Variable | Description |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Overrides `encryption_key` from the config file |

### Encryption Key

On first run without a configured key, Belochka auto-generates one at `{data_dir}/encryption.key` and logs a warning. For production, set the key explicitly via the config file or environment variable.
