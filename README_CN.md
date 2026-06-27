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
- **单一二进制文件** — Go 后端内嵌 React 前端；只需部署一个文件，无需安装其他依赖
- **持久化 SSH 连接** — 支持指数退避自动重连和心跳保活
- **加密凭据存储** — 服务器密码使用 AES-256-GCM 加密存储
- **Cron 任务管理** — 直接在服务器详情页查看、添加、编辑、启用/禁用、删除和立即执行 Cron 任务
- **多语言界面** — 支持英语、中文、法语和俄语

## 快速开始

从 [Releases](https://github.com/Unmovable8911/Belochka/releases) 下载最新的二进制文件，然后运行：

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (amd64)
belochka-windows-amd64.exe
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
#   bin/belochka-windows-amd64.exe
```

## 配置

Belochka 开箱即用，无需任何配置。可选择在工作目录中创建 `belochka.yaml` 文件，或通过 `--config 配置文件路径` 参数指定：

```yaml
port: 53136        # HTTP 监听端口（默认：53136）
data_dir: ./data   # 数据库和加密密钥存储位置（默认：./data）
encryption_key: "" # AES-256 密钥；留空则自动生成
```

### 环境变量

| 变量 | 说明 |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | 覆盖配置文件中的 `encryption_key` 值 |

### 加密密钥

首次运行时如未配置密钥，Belochka 会在 `{data_dir}/encryption.key` 自动生成一个，并在日志中输出警告。生产环境中，建议通过配置文件或环境变量显式设置密钥。
