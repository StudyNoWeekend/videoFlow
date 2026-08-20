<div align="center">

# VideoFlow

### 給 Whisper + FFmpeg 裝上 Web 視覺化管理 -- 掃描影片目錄，字幕自動產生，損壞影片一鍵去馬賽克。

基於 Whisper ASR + FFmpeg + Docker 去馬賽克引擎，用 Vue 3 + Element Plus 打造的 Web 視覺化管理介面。告別腳本拼裝，掃描入庫、建立任務、即時進度、線上設定，全部在瀏覽器裡完成。後端 Go（Gin + GORM + SQLite），支援 Docker 一鍵部署。

[![Stars](https://img.shields.io/github/stars/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=yellow)](https://github.com/StudyNoWeekend/videoFlow/stargazers)
[![Forks](https://img.shields.io/github/forks/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=blue)](https://github.com/StudyNoWeekend/videoFlow/network/members)
[![Issues](https://img.shields.io/github/issues/StudyNoWeekend/videoFlow?style=flat-square&logo=github)](https://github.com/StudyNoWeekend/videoFlow/issues)
[![License](https://img.shields.io/github/license/StudyNoWeekend/videoFlow?style=flat-square&color=green)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?style=flat-square&logo=vue.js&logoColor=white)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white)](./Dockerfile)

[💡 專案亮點](#-專案亮點) · [🎯 誰會想用](#-誰會想用) · [✨ 功能特性](#-功能特性) · [📥 快速開始](#-快速開始) · [🎬 使用流程](#-使用流程) · [🌐 技術棧](#-技術棧) · [🔧 設定](#-設定) · [📊 API 介面](#-api-介面) · [❓ FAQ](#-faq) · [🗺️ Roadmap](#️-roadmap)

🌐 [简体中文](./README.md) · **繁體中文** · [English](./README.en.md) · [日本語](./README.ja.md)

</div>

---

## 💡 專案亮點

本專案最大的亮點是：**把 Whisper 語音辨識、FFmpeg 音訊處理、Docker 去馬賽克、清晰度增強這四件本來要拼腳本才能串起來的事，整合成了一套開箱即用的 Web 視覺化管理平台**。

單獨用 Whisper / FFmpeg 並不難，但要把「掃描目錄 -> 擷取音訊 -> 呼叫 ASR -> 產生字幕 -> 任務並行 -> 進度追蹤 -> 失敗重試 -> 線上改設定」串成完整流水線，命令列和腳本很難做得順手。VideoFlow 在不變動底層引擎的前提下，給它套上了一層 Web UI：

| 對比 | 手動拼裝（Whisper + FFmpeg 腳本） | VideoFlow（本專案） |
| --- | --- | --- |
| 互動方式 | 命令列參數 + shell 腳本 | 瀏覽器圖形介面，滑鼠點點點 |
| 字幕產生 | 手動擷取音訊、呼叫 ASR、拼字幕檔案 | 一鍵建立任務，自動 擷取音訊 -> ASR -> 產生字幕 |
| 去馬賽克 | 手動跑 Docker 指令、盯終端機 | 一鍵去馬賽克任務，支援 CPU / CUDA / MPS / XPU |
| 清晰度增強 | 自己找工具、參數難調 | 一鍵升級解析度，Real-ESRGAN / Real-CUGAN / libplacebo 可選 |
| 任務管理 | 自己記，關掉終端機就沒了 | 任務清單 + 即時進度 + 失敗重試 + 歷史可查 |
| 並行控制 | 自己寫佇列 / 號誌 | 排程器按設定並行數自動排程 |
| 設定修改 | 改設定檔、重啟服務 | 線上修改，儲存即熱生效，持久化到資料庫 |
| 部署形態 | 一堆相依套件要裝 | Docker 單映像，掛載設定即跑 |

底層復用 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) 的辨識能力、[FFmpeg](https://ffmpeg.org/) 的音訊/影片處理、[`ladaapp/lada`](https://github.com/ladaapp/lada) 的去馬賽克，在其之上封裝出 HTTP API 與 Web 前端 -- **命令列的能力，圖形介面的體驗**。

## 🎯 誰會想用

| 你是 | 你能用它做什麼 |
| --- | --- |
| **影片創作者 / 自媒體** | 批量給影片產生字幕，省去手動聽寫，匯出 SRT / VTT 直接用 |
| **字幕組 / 翻譯愛好者** | 用 Whisper 批量轉錄產生時間軸，再人工校對，效率翻倍 |
| **有損壞影片的人** | 影片打不開？用 lada Docker 一鍵去馬賽克，支援多種運算裝置 |
| **NAS / 家庭伺服器玩家** | Docker 長期掛著，定時掃描目錄自動入庫，新影片自動產生字幕 |
| **想本地跑 Whisper 的人** | 不想寫腳本，Web 介面設定 ASR 參數（語言 / VAD / 提示詞）即可 |
| **嫌命令列麻煩的人** | 全程圖形介面，設定、掃描、任務進度一目了然 |
| **本地化執行** | ffmpeg 自動智慧呼叫本地已安裝的 ffmpeg，無需額外配置 |

## ✨ 功能特性

- **影片目錄掃描** - 手動掃描或背景定時自動掃描，影片自動入庫，卡片式分頁展示
- **字幕產生** - 基於 Whisper ASR，支援語言、VAD 過濾、任務類型、音訊預編碼、初始提示詞、詞級時間戳、多種輸出格式（json / srt / vtt / txt / tsv）
- **去馬賽克** - 基於 Docker 映像 `ladaapp/lada`，支援 x86_64 CPU 以及 NVIDIA CUDA 顯示卡（Turing 系列或更高版本，包括 RTX 20xx 到 RTX 50xx 系列），CUDA 設備自動透傳（`--gpus`）、GPU 故障原因自動提示
- **清晰度增強** - 基於 Video2X（`ghcr.io/k4yt3x/video2x:latest`）將影片升級到更高解析度，支援 Real-ESRGAN / Real-CUGAN / libplacebo 處理器，目標解析度與降噪等級按任務指定
- **任務管理** - 建立 / 查詢 / 刪除 / 失敗重試，按類型過濾，即時進度條，前端 2 秒輪詢重新整理
- **任務排程器** - 背景排程器按設定的並行數限制執行字幕 / 去馬賽克 / 清晰度增強任務，輪詢間隔可線上設定
- **使用者認證** - 首次啟動引導初始化管理員帳號，登入 / 修改密碼 / 密碼重設，業務 API 全部走 JWT 鑑權
- **執行時設定** - 統一設定頁，所有設定線上修改並持久化（SQLite），儲存即熱生效
- **FFmpeg 智慧呼叫** - 自動探測本地 ffmpeg，無需手動配置
- **工程化** - `trace_id` 全鏈路追蹤、統一回應結構、zap 結構化日誌、優雅關閉（重啟時執行中任務自動標記失敗）

## 📥 快速開始

### 方式一：Docker 部署（推薦）

映像已發布到 GitHub Container Registry：

```bash
docker pull ghcr.io/studynoweekend/videoflow:latest
```

1. 準備設定檔（基於範本修改）：

```bash
cp backend/config/config.yaml.local config.yaml
# 按需修改 config.yaml：asr.url、video.dir 等
```

2. 啟動容器（連接埠、設定、資料、日誌均可在 `docker run` 時指定）：

```bash
docker run -d --name videoflow \
  --restart unless-stopped \
  -e APP_HTTP_PORT=8080 \
  -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  -v "$PWD/logs:/app/logs" \
  ghcr.io/studynoweekend/videoflow:latest
```

3. 健康檢查：

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"},"code":0,"msg":"success","trace_id":"..."}
```

**變更連接埠**：只需改兩處，連接埠真源單一：

```bash
docker run -d --name videoflow \
  -e APP_HTTP_PORT=9090 -p 9090:9090 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  ghcr.io/studynoweekend/videoflow:latest
```

<details>
<summary><b>方式二：本地建置映像</b></summary>

```bash
docker build -t video-captions:latest -f Dockerfile .
docker run -d --name videoflow \
  -e APP_HTTP_PORT=8080 -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  video-captions:latest
```

Dockerfile 為多階段建置：`golang:1.25-alpine` 跨譯（CGO）+ `alpine:3.20` 執行（內建 ffmpeg）。支援 `linux/amd64` 和 `linux/arm64` 雙架構。

</details>

### 方式三：本地開發部署

**前置要求：** Go 1.25+、Node.js 22.18+、FFmpeg，並已執行 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)。

```bash
# 後端
cd backend
cp config/config.yaml.local config/config.yaml   # 按需修改
go run ./cmd/api

# 前端（另開終端機）
cd frontend
npm install
npm run dev
```

開發態前端透過 Vite 代理將 `/api` 轉發到後端，連接埠由環境變數 `APP_HTTP_PORT` 統一控制（預設 8080，與後端共用）：

```bash
APP_HTTP_PORT=9090 npm run dev        # 前端
APP_HTTP_PORT=9090 go run ./cmd/api   # 後端
```

瀏覽器開啟 Vite 提示的本地位址即可。

## 🎬 使用流程

1. 啟動服務，瀏覽器存取前端位址；首次使用先初始化管理員帳號並登入
2. 在「設定」頁設定本地影片目錄與 ASR 服務位址，儲存
3. 在「影片清單」頁點擊掃描，影片自動入庫
4. 點影片卡片上的「產生字幕」，建立字幕任務
5. 在「任務管理」頁查看即時進度（2 秒自動重新整理）
6. （可選）點「去馬賽克」修復損壞影片，或點「清晰度增強」將影片升級到更高解析度
7. 字幕任務完成後，查看產生的字幕檔案

## 🌐 技術棧

| 層 | 技術 | 說明 |
| --- | --- | --- |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) | HTTP 路由與中介軟體 |
| ORM | [GORM](https://github.com/go-gorm/gorm) | 資料持久化 |
| 資料庫 | SQLite（[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)） | CGO 驅動 |
| 設定 | [Viper](https://github.com/spf13/viper) | 設定檔 + 環境變數覆寫 |
| 日誌 | [Zap](https://github.com/uber-go/zap) | 結構化日誌 |
| ASR | [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) | 語音辨識引擎 |
| 音訊/影片 | [FFmpeg](https://ffmpeg.org/) / ffprobe | 音訊擷取、時長探測，智慧本地呼叫 |
| 去馬賽克 | [`ladaapp/lada`](https://github.com/ladaapp/lada) | Docker 去馬賽克引擎 |
| 清晰度增強 | [Video2X](https://github.com/k4yt3x/video2x) | Docker 清晰度增強引擎 |
| 前端 | [Vue 3](https://vuejs.org/) + [Element Plus](https://element-plus.org/) + [Vite](https://vite.dev/) | 圖形介面 |
| 狀態管理 | [Pinia](https://pinia.vuejs.org/) | 前端狀態 |
| HTTP 客戶端 | [Axios](https://axios-http.com/) | 介面請求 |

## 🔧 設定

設定優先順序：**環境變數 > config.yaml > 程式碼預設值**。執行時還可在前端「設定」頁線上修改並持久化到資料庫。

<details>
<summary><b>設定檔（config/config.yaml）</b></summary>

```yaml
app:
  name: videoFlow
  env: dev
  port: 8080
http:
  port: 8080
  read_timeout: 30
  write_timeout: 30
log:
  level: info
  path: logs
database:
  driver: sqlite
  dsn: data/app.db
video:
  dir: ""
scan:
  interval: 60
asr:
  url: http://127.0.0.1:9999/asr
  language: zh
  vad_filter: false
  task: transcribe
  encode: true
  initial_prompt: ""
  word_timestamps: false
  output: json
repair:
  docker_image: ladaapp/lada:latest
  device: cpu            # cpu / cuda:0 / mps / xpu:0
concurrency:
  subtitle: 2
  repair: 1
```

</details>

<details>
<summary><b>環境變數（Docker 部署用）</b></summary>

viper 前綴 `APP_`，設定鍵 `.` -> `_`，故 `http.port` 對應環境變數 `APP_HTTP_PORT`，依此類推：

| 環境變數 | 設定項 | 預設值 |
| --- | --- | --- |
| `APP_HTTP_PORT` | `http.port` | `8080` |
| `APP_VIDEO_DIR` | `video.dir` | `""` |
| `APP_SCAN_INTERVAL` | `scan.interval` | `60` |
| `APP_ASR_URL` | `asr.url` | `http://127.0.0.1:9999/asr` |
| `APP_ASR_LANGUAGE` | `asr.language` | `zh` |
| `APP_REPAIR_DEVICE` | `repair.device` | `cpu` |
| `APP_DATABASE_DSN` | `database.dsn` | `data/app.db` |

</details>

<details>
<summary><b>Docker 磁碟區掛載點</b></summary>

| 容器路徑 | 用途 |
| --- | --- |
| `/app/config/config.yaml` | 設定檔（唯讀掛載） |
| `/app/data` | SQLite 資料庫持久化（`data/app.db`） |
| `/app/logs` | 日誌輸出 |
| `/var/run/docker.sock` | （可選）去馬賽克 / 清晰度增強需要掛載宿主機 Docker socket |

</details>

## 📊 API 介面

所有介面統一前綴 `/api/v1`，回應結構：

```json
{
  "code": 0,
  "msg": "success",
  "data": {},
  "trace_id": "xxx"
}
```

`code=0` 表示成功，非 0 表示業務錯誤。

> **認證說明**：除健康檢查、認證介面、`/api/v1/version` 與元件安裝進度 SSE 介面外，其餘業務介面均需在請求頭攜帶登入取得的 Token（`Authorization: Bearer <token>`）。

<details>
<summary><b>認證介面</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/auth/status` | 查詢初始化 / 登入狀態 |
| `POST` | `/api/v1/auth/init` | 首次執行初始化管理員帳號 |
| `POST` | `/api/v1/auth/login/password` | 帳號密碼登入，回傳 Token |
| `POST` | `/api/v1/auth/reset-token` | 產生密碼重設令牌 |
| `POST` | `/api/v1/auth/reset-password` | 透過令牌重設密碼 |
| `POST` | `/api/v1/auth/change-password` | 修改密碼（需登入） |

</details>

<details>
<summary><b>健康檢查</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `GET` | `/health` | 健康檢查 |
| `GET` | `/ready` | 就緒檢查 |
| `GET` | `/api/v1/version` | 目前版本號 |

</details>

<details>
<summary><b>影片介面</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `POST` | `/api/v1/videos/scan` | 掃描影片目錄入庫 |
| `GET` | `/api/v1/videos` | 分頁查詢影片清單 |
| `PUT` | `/api/v1/videos/:id` | 更新影片資訊 |
| `DELETE` | `/api/v1/videos/:id` | 刪除影片記錄 |

</details>

<details>
<summary><b>任務介面</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | 建立任務（字幕 / 字幕燒錄 / 去馬賽克 / 清晰度增強） |
| `GET` | `/api/v1/tasks` | 分頁查詢任務清單（可按類型過濾） |
| `POST` | `/api/v1/tasks/:id/retry` | 重試失敗任務 |
| `DELETE` | `/api/v1/tasks/:id` | 刪除任務 |

任務狀態：`pending`（待處理）-> `running`（執行中）-> `completed`（已完成）/ `failed`（失敗）

> **清晰度增強任務參數**：建立時需指定目標解析度（`target_width` / `target_height`），處理器（`upscale_processor`：`realesrgan` / `realcugan` / `libplacebo`）、模型（`upscale_model`）與降噪等級（`upscale_noise_level`，-1 ~ 3）可按需設定。

</details>

<details>
<summary><b>元件介面</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/components` | 檢測各元件狀態（Docker / FFmpeg / Whisper ASR / lada / Video2X） |
| `GET` | `/api/v1/components/install/progress/:session_id` | 元件安裝進度（SSE 即時推送，公開介面） |

</details>

<details>
<summary><b>執行時設定介面</b></summary>

| 方法 | 路徑 | 說明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 取得執行時設定 |
| `PUT` | `/api/v1/settings` | 更新執行時設定（儲存即熱生效） |

</details>

## 📁 專案結構

```
videoFlow/
├── backend/
│   ├── cmd/api/              # 程式入口 main.go
│   ├── bootstrap/            # 設定、DB、ASR、FFmpeg、去馬賽克 等初始化
│   ├── config/               # 設定檔（config.yaml.local 為範本）
│   ├── internal/
│   │   ├── controller/       # HTTP 控制器
│   │   ├── logic/            # 業務邏輯
│   │   ├── model/            # 資料模型與持久化（GORM）
│   │   ├── dto/              # 請求/回應 DTO
│   │   ├── router/           # 路由註冊
│   │   ├── asr/              # ASR 客戶端
│   │   ├── ffmpeg/           # FFmpeg 本地/SSH 執行器
│   │   ├── repair/           # 去馬賽克執行器
│   │   ├── upscale/          # 清晰度增強執行器
│   │   ├── subtitle/         # 字幕解析
│   │   ├── scanner/          # 影片目錄掃描器
│   │   └── scheduler/        # 任務排程器
│   ├── enum/                 # 業務錯誤碼
│   └── utils/                # 日誌、回應封裝
├── frontend/
│   ├── src/
│   │   ├── api/              # 介面請求封裝
│   │   ├── views/            # 頁面（影片/任務/設定）
│   │   ├── stores/           # Pinia 狀態
│   │   ├── router/           # 路由
│   │   └── utils/            # 工具函式
│   └── vite.config.ts        # 含 /api 代理設定
├── Dockerfile                # 多階段建置
└── LICENSE
```

## 🛡️ 注意事項

- **FFmpeg 必需**：`ffmpeg.provider` 固定為 `local`，映像已內建 ffmpeg；本地原始碼執行需自行安裝
- **去馬賽克 / 清晰度增強需 Docker**：容器部署時需掛載宿主機 Docker socket；不用該功能可忽略，不影響服務啟動
- **首次使用需初始化**：瀏覽器首次存取會引導初始化管理員帳號，登入後才能使用業務功能
- **ASR 服務需自備**：請自行部署 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)，並設定 `asr.url`
- **資料庫持久化**：預設 `data/app.db`，Docker 部署時務必掛載 `/app/data` 目錄，否則重啟遺失資料

## ❓ FAQ

<details>
<summary><b>啟動報 <code>ffmpeg not found</code> 怎麼辦？</b></summary>

`ffmpeg.provider` 固定為 `local`，需要本地（或映像內）存在 ffmpeg。Docker 映像已內建；原始碼執行請自行安裝 ffmpeg。

</details>

<details>
<summary><b>去馬賽克功能不可用？</b></summary>

去馬賽克依賴 Docker。容器部署時需掛載宿主機 Docker socket：`-v /var/run/docker.sock:/var/run/docker.sock`；不用該功能可忽略，不影響服務啟動。

</details>

<details>
<summary><b>如何變更後端連接埠？</b></summary>

環境變數 `APP_HTTP_PORT` 覆寫 `http.port`（後端 viper 與前端 Vite 代理共用此變數）。`docker run -e APP_HTTP_PORT=9090 -p 9090:9090` 即可。

</details>

<details>
<summary><b>資料庫在哪？</b></summary>

預設 `data/app.db`（SQLite）。Docker 部署時掛載 `/app/data` 目錄以持久化。

</details>

## 🗺️ Roadmap

- ✅ 影片掃描 + 字幕產生 + 去馬賽克
- ✅ 任務管理 + 即時進度 + 失敗重試
- ✅ 執行時設定線上修改 + 持久化
- ✅ Docker 部署 + 連接埠執行時指定
- ✅ FFmpeg 智慧本地呼叫
- ✅ 多架構映像（amd64 + arm64 原生支援）
- ✅ 使用者認證（登入 / 初始化 / 修改密碼）
- ✅ 影片清晰度增強（Video2X）
- 🔲 字幕線上預覽 / 跨輯
- 🔲 字幕檔案匯出下載
- 🔲 API Key 鑑權，用於外網存取攔截

## 💌 致謝

本專案站在巨人的肩膀上，特別感謝以下開源專案及其作者：

- **[Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)** - by [@ahmetoner](https://github.com/ahmetoner)，基於 OpenAI Whisper 的 ASR HTTP 服務，VideoFlow 的語音辨識能力全部基於此。
- **[FFmpeg](https://ffmpeg.org/)** - 強大的音訊/影片處理工具，負責音訊擷取與時長探測。
- **[ladaapp/lada](https://github.com/ladaapp/lada)** - 去馬賽克 Docker 映像。
- **[Video2X](https://github.com/k4yt3x/video2x)** - 影片清晰度增強 Docker 映像。
- **[Gin](https://github.com/gin-gonic/gin)** / **[GORM](https://github.com/go-gorm/gorm)** / **[Viper](https://github.com/spf13/viper)** / **[Zap](https://github.com/uber-go/zap)** - 優秀的 Go 基礎庫。
- **[Vue 3](https://vuejs.org/)** / **[Element Plus](https://element-plus.org/)** / **[Vite](https://vite.dev/)** - 前端基石。

## ⭐ Star history

[![Stargazers over time](https://api.star-history.com/svg?repos=StudyNoWeekend/videoFlow&type=Date)](https://star-history.com/#StudyNoWeekend/videoFlow&Date)

## 📜 授權條款

[MIT License](./LICENSE) - 自由使用、修改、散布，只需保留版權聲明。

---

<div align="center">

**用過覺得有用？給個 ⭐ 是對作者最大的鼓勵。**

[⬆ 回到頂部](#videoflow) · [📥 快速開始](#-快速開始) · [💬 提 Issue](https://github.com/StudyNoWeekend/videoFlow/issues)

</div>
