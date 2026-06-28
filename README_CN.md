<p align="center">
  <img src="./logo.png" width="500">
</p>
<p align="center">
<a href="./README.md">English</a> / 
中文 / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_RU.md">Русский</a>
</p>
<hr>

Belochka（белочка，"松鼠"）是一款单二进制文件的服务器监控工具，专为小规模 Linux 服务器集群设计。它通过持久化 SSH 连接管理 5 至 20 台远程服务器，并通过 WebSocket 将 CPU、内存、磁盘、网络和进程的实时指标推送到浏览器仪表盘。同时提供基于 Web 的交互式终端，可直接进行 SSH 访问——无需单独的 SSH 客户端。

## 功能特性

- **实时仪表盘** — 服务器卡片展示实时 CPU、内存、磁盘和网络指标，按使用率自动着色
- **服务器详情视图** — 每核 CPU 仪表盘、内存/交换分区环形图、磁盘分区详情、网络接口吞吐量、可排序的进程表
- **Web 终端** — 通过 xterm.js 在浏览器中提供完整的交互式 SSH 控制台
- **系统托盘图标** — 在桌面环境（Windows、macOS、Linux GNOME/KDE/XFCE）下，在通知区域显示托盘图标，提供**打开仪表盘**和**退出**菜单项；在无桌面的服务器上自动降级为 CLI 模式
- **单一二进制文件** — Go 后端内嵌 React 前端；只需部署一个文件，无需安装其他依赖
- **持久化 SSH 连接** — 支持指数退避自动重连和心跳保活
- **加密凭据存储** — 服务器密码使用 AES-256-GCM 加密存储
- **Cron 任务管理** — 直接在服务器详情页查看、添加、编辑、启用/禁用、删除和立即执行 Cron 任务
- **持久化日志文件** — 所有输出写入用户缓存目录下的日志文件（例如 `~/.cache/belochka/belochka.log`），自动按保留期清理（默认：3 天）
- **多语言界面** — 支持英语、中文、法语和俄语；首次访问时自动检测语言，可在设置对话框中切换
- **应用内设置** — 通过仪表盘右上角的齿轮图标直接配置端口、数据目录、语言和日志保留天数，无需手动编辑配置文件

## 快速开始

从 [Releases](https://github.com/Unmovable8911/Belochka/releases) 下载最新的二进制文件，然后运行：

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 位)
belochka-windows-x86-64.exe
```

在浏览器中打开 `http://localhost:53136`，通过界面添加服务器。

## 从源码构建

需要 Go 1.25+ 和 Node.js 18+。

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

交叉编译所有平台的发布二进制文件：

```bash
make release
# 输出：
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## 配置

Belochka 开箱即用，无需任何配置。所有设置均可通过仪表盘中的**设置对话框**（齿轮图标）修改。也可在工作目录中创建 `config.json`，或通过 `--config 配置文件路径` 参数指定：

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| 字段 | 默认值 | 说明 |
|---|---|---|
| `port` | `53136` | HTTP 监听端口 |
| `data_dir` | `./data` | 数据库和加密密钥存储位置 |
| `language` | `""` | 界面语言（`en`、`zh`、`fr`、`ru`）；留空则首次访问时自动检测 |
| `log_path` | `""` | 日志文件路径；留空则使用 `~/.cache/belochka/belochka.log` |
| `log_retention_days` | `3` | 日志保留天数 |

修改 `port` 和 `data_dir` 需要重启；`language` 和 `log_retention_days` 可通过设置对话框立即生效。

### 命令行参数

| 参数 | 说明 |
|---|---|
| `--config <路径>` | 指定 JSON 配置文件路径 |
| `--no-tray` | 禁用系统托盘图标，以 CLI 进程方式运行 |
| `--version` | 打印版本并退出 |

### 环境变量

| 变量 | 说明 |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | 存储密码所用的 AES-256 加密密钥；未设置时首次运行自动生成 |

### 加密密钥

首次运行时如未设置密钥，Belochka 会在 `{data_dir}/encryption.key` 自动生成一个，并在日志中输出警告。生产环境中，建议通过环境变量显式设置密钥。
