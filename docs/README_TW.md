<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
<a href="./README_DE.md">Deutsch</a> / 
<a href="../README.md">English</a> / 
<a href="./README_ES.md">Español</a> / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_IT.md">Italiano</a> / 
<a href="./README_PT.md">Português</a> / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
繁體中文
</p>
<hr>

Belochka（белочка，「松鼠」）是一款單一二進位檔的伺服器監控工具，專為小規模 Linux 伺服器群組設計。它透過持久化 SSH 連線管理 5 至 20 台遠端伺服器，並透過 WebSocket 將 CPU、記憶體、磁碟、網路與程序的即時指標推送到瀏覽器儀表板。同時提供基於 Web 的互動式終端，可直接進行 SSH 存取——無需單獨的 SSH 用戶端。

## 功能特色

- **伺服器分組** — 在側欄中用扁平的頂層群組整理伺服器；可建立、重新命名、刪除群組，透過拖放或「移至…」選單將伺服器移入或移出群組；點擊群組可將儀表板篩選至其成員伺服器，並以麵包屑導覽返回
- **即時儀表板** — 伺服器卡片顯示即時 CPU、記憶體、磁碟與網路指標，依使用率自動著色；在卡片上按右鍵可快速進行編輯／刪除／主控台操作
- **伺服器詳細檢視** — 每核心 CPU 儀表、記憶體／置換分割區環形圖、磁碟分割區明細、網路介面吞吐量
- **程序管理** — 獨立程序頁籤，提供可排序的扁平表格（欄寬可調整）、多重關鍵字搜尋、自動重新整理開關，以及支援 SIGTERM/SIGKILL 訊號選擇的程序終止（sshd/init/systemd 受保護）
- **Web 終端** — 透過 xterm.js 在瀏覽器中提供完整的互動式 SSH 主控台
- **系統匣圖示** — 在桌面環境（Windows、macOS、Linux GNOME/KDE/XFCE）下，在工作列顯示匣圖示，提供**開啟儀表板**與**結束**選單項目；在無桌面的伺服器上自動切換為 CLI 模式
- **身分驗證** — 密碼＋工作階段 Cookie 保護；首次造訪會引導完成兩步驟設定精靈（選擇語言 → 設定密碼，含強度指示器與確認）；10 次登入失敗後觸發速率限制（30 分鐘鎖定）
- **單一二進位檔** — Go 後端內嵌 React 前端；只需部署一個檔案，無需安裝其他項目
- **持久化 SSH 連線** — 支援指數退避自動重新連線與 Keepalive 保活；儲存伺服器前可測試連線，新增機器時顯示主機金鑰指紋供首次信任（TOFU）驗證
- **瀏覽器上傳金鑰** — 透過介面直接上傳 SSH 私鑰檔案；金鑰經驗證後以 UUID 檔名儲存，孤立檔案會自動清除
- **加密的憑證儲存** — 伺服器密碼使用 AES-256-GCM 靜態加密
- **Cron 工作管理** — 直接在伺服器詳細頁檢視、新增、編輯、啟用／停用、刪除與立即執行 Cron 工作
- **批次指令** — 在側欄對話框中編寫多行指令碼並批次傳送至任意伺服器群組；每台伺服器的終端輸出即時流入對話框，可互動回應提示，並可隨時取消整個執行
- **持久化日誌檔案** — 所有輸出寫入二進位檔旁的 `belochka.log`（使用 `go run` 執行時則寫入目前工作目錄），依保留期限自動清理（預設：3 天）
- **多語言介面** — 支援英文、簡體中文、法文、俄文、德文、西班牙文、葡萄牙文、繁體中文與義大利文；首次設定時可選擇，也可在設定對話框中切換；瀏覽器偵測的語言預設預選
- **應用程式內設定** — 透過儀表板中的齒輪圖示直接設定連接埠、資料目錄、語言與日誌保留天數，無需手動編輯設定檔

## 快速開始

從 [Releases](https://github.com/Unmovable8911/Belochka/releases) 下載最新的二進位檔，然後執行：

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 位元)
belochka-windows-x86-64.exe
```

在瀏覽器中開啟 `http://localhost:53136`。首次造訪時會提示選擇語言並設定密碼——該密碼用於保護儀表板與所有 API 端點。設定完成後自動登入。透過介面新增伺服器。

## 從原始碼建置

需要 Go 1.25+ 與 Node.js 18+。

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

交叉編譯所有平台的發行二進位檔：

```bash
make release
# 輸出：
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## 設定

Belochka 開箱即用，無需任何設定。所有設定均可透過儀表板中的**設定對話框**（齒輪圖示）修改。首次執行時，二進位檔旁會自動建立包含預設值的 `config.json`。也可透過 `--config 設定檔路徑` 參數指定自訂位置：

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| 欄位 | 預設值 | 說明 |
|---|---|---|
| `port` | `53136` | HTTP 監聽連接埠 |
| `data_dir` | `./data` | 資料庫、加密金鑰與上傳的 SSH 金鑰檔案（`data/keys/`）儲存位置 |
| `language` | `""` | 介面語言（`en`、`zh`、`fr`、`ru`、`de`、`es`、`pt`、`zh-TW`、`it`）；留空則首次造訪時自動偵測 |
| `log_path` | `""` | 日誌檔案路徑；留空則使用二進位檔旁的 `belochka.log` |
| `log_retention_days` | `3` | 日誌保留天數 |

修改 `port` 與 `data_dir` 需要重新啟動；`language` 與 `log_retention_days` 可透過設定對話框立即生效。

### 命令列參數

| 參數 | 說明 |
|---|---|
| `--config <路徑>` | 指定 JSON 設定檔路徑 |
| `--no-tray` | 停用系統匣圖示，以 CLI 程序方式執行 |
| `--version` | 列印版本並結束 |

### 環境變數

| 變數 | 說明 |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | 儲存密碼所用的 AES-256 加密金鑰；未設定時首次執行自動產生 |

### 加密金鑰

首次執行時如未設定金鑰，Belochka 會在 `{data_dir}/encryption.key` 自動產生一個，並在日誌中輸出警告。生產環境中，建議透過環境變數明確設定金鑰。
