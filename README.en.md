<div align="center">

# VideoFlow

### A Web UI for Whisper + FFmpeg -- scan video directories, auto-generate subtitles, and repair broken videos with one click.

A Web management UI built with Vue 3 + Element Plus, powered by Whisper ASR + FFmpeg + a Docker repair engine. Say goodbye to gluing scripts together—scanning, task creation, real-time progress, and online config are all done in the browser. The backend is Go (Gin + GORM + SQLite), with one-click Docker deployment.

[![Stars](https://img.shields.io/github/stars/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=yellow)](https://github.com/StudyNoWeekend/videoFlow/stargazers)
[![Forks](https://img.shields.io/github/forks/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=blue)](https://github.com/StudyNoWeekend/videoFlow/network/members)
[![Issues](https://img.shields.io/github/issues/StudyNoWeekend/videoFlow?style=flat-square&logo=github)](https://github.com/StudyNoWeekend/videoFlow/issues)
[![License](https://img.shields.io/github/license/StudyNoWeekend/videoFlow?style=flat-square&color=green)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?style=flat-square&logo=vue.js&logoColor=white)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white)](./Dockerfile)

[💡 Project Highlights](#-project-highlights) · [🎯 Who Is This For](#-who-is-this-for) · [✨ Features](#-features) · [📥 Quick Start](#-quick-start) · [🎬 Usage Workflow](#-usage-workflow) · [🌐 Tech Stack](#-tech-stack) · [🔧 Configuration](#-configuration) · [📊 API](#-api) · [❓ FAQ](#-faq) · [🗺️ Roadmap](#️-roadmap)

🌐 [简体中文](./README.md) · [繁體中文](./README.zh-TW.md) · **English** · [日本語](./README.ja.md)

</div>

---

## 💡 Project Highlights

The biggest highlight of this project: **it unifies Whisper speech recognition, FFmpeg audio processing, and Docker video repair—three things that normally require gluing scripts together—into an out-of-the-box Web management platform**.

Using Whisper or FFmpeg alone isn't hard, but chaining "scan directory -> extract audio -> call ASR -> generate subtitles -> concurrent tasks -> progress tracking -> failure retry -> online config editing" into a complete pipeline is awkward to do with command-line scripts. Without modifying the underlying engines, VideoFlow wraps them in a Web UI:

| Comparison | Manual scripting (Whisper + FFmpeg scripts) | VideoFlow (this project) |
| --- | --- | --- |
| Interaction | CLI args + shell scripts | Browser GUI, point and click |
| Subtitle generation | Manually extract audio, call ASR, assemble subtitle files | One-click task creation, auto extract audio -> ASR -> generate subtitles |
| Video repair | Manually run Docker commands, watch the terminal | One-click repair task, supports CPU / CUDA / MPS / XPU |
| Task management | Track it yourself; gone when you close the terminal | Task list + real-time progress + failure retry + searchable history |
| Concurrency control | Write your own queue / semaphore | Scheduler dispatches by configured concurrency |
| Configuration changes | Edit config file, restart service | Edit online, hot-reload on save, persisted to database |
| Deployment | A pile of dependencies to install | Single Docker image, mount config and run |

Under the hood it reuses the recognition power of [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice), the audio/video processing of [FFmpeg](https://ffmpeg.org/), and the video repair of [`ladaapp/lada`](https://github.com/ladaapp/lada), wrapping them with an HTTP API and Web frontend -- **the power of the command line, the experience of a graphical UI**.

## 🎯 Who Is This For

| If you are | What you can do with it |
| --- | --- |
| **Video creators / self-media** | Batch-generate subtitles for videos, skip manual transcription, export SRT / VTT ready to use |
| **Subtitle groups / translation enthusiasts** | Use Whisper to batch-transcribe and generate timelines, then proofread manually—double the efficiency |
| **People with broken videos** | Video won't open? Repair it with one click via the lada Docker image, supporting multiple compute devices |
| **NAS / home server enthusiasts** | Keep Docker running long-term, periodically scan directories to auto-import, and auto-generate subtitles for new videos |
| **People who want to run Whisper locally** | No scripts needed—configure ASR parameters (language / VAD / prompt) via the Web UI |
| **People who find the CLI cumbersome** | Fully graphical—config, scanning, and task progress at a glance |
| **Local users** | ffmpeg is automatically and intelligently called from your local installation, no extra configuration needed |

## ✨ Features

- **Video directory scanning** - Manual scan or background scheduled auto-scan, videos auto-imported, paginated card display
- **Subtitle generation** - Based on Whisper ASR, supports language, VAD filter, task type, audio pre-encoding, initial prompt, word-level timestamps, and multiple output formats (json / srt / vtt / txt / tsv)
- **Video repair** - Based on the `ladaapp/lada` Docker image, supports x86_64 CPU and NVIDIA CUDA GPUs (Turing series or higher, RTX 20xx to RTX 50xx)
- **Task management** - Create / query / delete / retry-on-failure, filter by type, real-time progress bar, 2-second polling refresh on the frontend
- **Task scheduler** - A background scheduler runs subtitle / repair tasks within configured concurrency limits
- **Runtime configuration** - Unified settings page, all config editable online and persisted (SQLite), hot-reload on save
- **Smart FFmpeg invocation** - Automatically detects local ffmpeg, no manual configuration needed
- **Engineering** - `trace_id` end-to-end tracing, unified response structure, zap structured logging, graceful shutdown (running tasks auto-marked as failed on restart)

## 📥 Quick Start

### Option 0: One-click script (highly recommended)

The project provides an [`install.en.sh`](./install.en.sh) one-click installer that automatically completes all of the following:

- Detects the Docker environment
- Deploys [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) (supports CPU/GPU, engine and model selection)
- Pulls the [`ladaapp/lada`](https://github.com/ladaapp/lada) video repair image
- Installs [FFmpeg](https://ffmpeg.org/) (supports macOS / Linux / Windows)
- Guides you through config and starts the project container

```bash
bash install.en.sh
```

The script is interactive-just follow the prompts. Once finished, it prints the access URLs for all services.

> The script is also available in other languages: [简体中文](./install.sh) · [日本語](./install.ja.sh) · [繁體中文](./install.zh-TW.sh)

<details>
<summary><b>Script options</b></summary>

- **Run mode**: CPU mode (all platforms) / GPU mode (Linux + NVIDIA GPU only)
- **ASR engine**: `openai_whisper` (default) / `faster_whisper` / `whisperx`
- **Subtitle model**: `tiny` / `base` / `small` / `medium` / `large-v3` / `large-v3-turbo` and English-only models
- **Model cache persistence**: speeds up subsequent starts
- **FFmpeg installation**: auto-detects package manager (Homebrew / apt / dnf / yum / pacman / apk / winget / choco)

</details>

### Option 1: Pull the image

The image is published on the GitHub Container Registry:

```bash
docker pull ghcr.io/studynoweekend/videoflow:latest
```

1. Prepare the config file (modify from the template):

```bash
cp backend/config/config.yaml.local config.yaml
# Modify config.yaml as needed: asr.url, video.dir, etc.
```

2. Start the container (port, config, data, and logs can all be specified at `docker run`):

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

3. Health check:

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"},"code":0,"msg":"success","trace_id":"..."}
```

**Change the port**: just update two places—the port has a single source of truth:

```bash
docker run -d --name videoflow \
  -e APP_HTTP_PORT=9090 -p 9090:9090 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  ghcr.io/studynoweekend/videoflow:latest
```

<details>
<summary><b>Option 2: Build the image locally</b></summary>

```bash
docker build -t video-captions:latest -f Dockerfile .
docker run -d --name videoflow \
  -e APP_HTTP_PORT=8080 -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  video-captions:latest
```

The Dockerfile uses a multi-stage build: Huawei Cloud `golang:1.25-alpine` for compilation (CGO) + `alpine:3.20` for runtime (with ffmpeg built in). The image is `linux/amd64` and runs via QEMU on Apple Silicon.

</details>

### Option 3: Local development setup

**Prerequisites:** Go 1.25+, Node.js 22.18+, FFmpeg, and a running [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice).

```bash
# Backend
cd backend
cp config/config.yaml.local config/config.yaml   # Modify as needed
go run ./cmd/api

# Frontend (in another terminal)
cd frontend
npm install
npm run dev
```

In development, the frontend uses a Vite proxy to forward `/api` to the backend. The port is controlled by the `APP_HTTP_PORT` environment variable (default 8080, shared with the backend):

```bash
APP_HTTP_PORT=9090 npm run dev        # Frontend
APP_HTTP_PORT=9090 go run ./cmd/api   # Backend
```

Open the local address Vite prints in your browser.

## 🎬 Usage Workflow

1. Start the service and open the frontend address in your browser
2. On the "Settings" page, configure your local video directory and ASR service URL, then save
3. On the "Video List" page, click scan to auto-import videos
4. Click "Generate Subtitles" on a video card to create a subtitle task
5. On the "Task Management" page, view real-time progress (auto-refreshes every 2 seconds)
6. (Optional) Click "Video Repair" to repair broken videos
7. Once the subtitle task completes, view the generated subtitle files

## 🌐 Tech Stack

| Layer | Technology | Description |
| --- | --- | --- |
| Web framework | [Gin](https://github.com/gin-gonic/gin) | HTTP routing and middleware |
| ORM | [GORM](https://github.com/go-gorm/gorm) | Data persistence |
| Database | SQLite ([mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)) | CGO driver |
| Config | [Viper](https://github.com/spf13/viper) | Config file + environment variable override |
| Logging | [Zap](https://github.com/uber-go/zap) | Structured logging |
| ASR | [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) | Speech recognition engine |
| Audio/Video | [FFmpeg](https://ffmpeg.org/) / ffprobe | Audio extraction, duration probing, local / SSH |
| Video repair | [`ladaapp/lada`](https://github.com/ladaapp/lada) | Docker repair engine |
| Frontend | [Vue 3](https://vuejs.org/) + [Element Plus](https://element-plus.org/) + [Vite](https://vite.dev/) | Graphical UI |
| State management | [Pinia](https://pinia.vuejs.org/) | Frontend state |
| HTTP client | [Axios](https://axios-http.com/) | API requests |

## 🔧 Configuration

Config priority: **environment variables > config.yaml > code defaults**. At runtime you can also edit config online on the frontend "Settings" page and persist it to the database.

<details>
<summary><b>Config file (config/config.yaml)</b></summary>

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
<summary><b>Environment variables (for Docker deployment)</b></summary>

Viper prefix `APP_`, config key `.` -> `_`, so `http.port` maps to the environment variable `APP_HTTP_PORT`, and so on:

| Environment variable | Config key | Default |
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
<summary><b>Docker volume mounts</b></summary>

| Container path | Purpose |
| --- | --- |
| `/app/config/config.yaml` | Config file (read-only mount) |
| `/app/data` | SQLite database persistence (`data/app.db`) |
| `/app/logs` | Log output |
| `/var/run/docker.sock` | (Optional) Video repair requires mounting the host Docker socket |

</details>

## 📊 API

All endpoints share the `/api/v1` prefix. Response structure:

```json
{
  "code": 0,
  "msg": "success",
  "data": {},
  "trace_id": "xxx"
}
```

`code=0` means success; non-zero indicates a business error.

<details>
<summary><b>Health check</b></summary>

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Health check |
| `GET` | `/ready` | Readiness check |

</details>

<details>
<summary><b>Video endpoints</b></summary>

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/videos/scan` | Scan video directory and import |
| `GET` | `/api/v1/videos` | Paginated video list query |
| `PUT` | `/api/v1/videos/:id` | Update video info |
| `DELETE` | `/api/v1/videos/:id` | Delete video record |

</details>

<details>
<summary><b>Task endpoints</b></summary>

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | Create a task (subtitle / repair) |
| `GET` | `/api/v1/tasks` | Paginated task list query (filterable by type) |
| `POST` | `/api/v1/tasks/:id/retry` | Retry a failed task |
| `DELETE` | `/api/v1/tasks/:id` | Delete a task |

Task status: `pending` -> `running` -> `completed` / `failed`

</details>

<details>
<summary><b>Runtime config endpoints</b></summary>

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | Get runtime config |
| `PUT` | `/api/v1/settings` | Update runtime config (hot-reload on save) |

</details>

## 📁 Project Structure

```
videoFlow/
├── backend/
│   ├── cmd/api/              # Entry point main.go
│   ├── bootstrap/            # Init for config, DB, ASR, FFmpeg, repair, etc.
│   ├── config/               # Config files (config.yaml.local is the template)
│   ├── internal/
│   │   ├── controller/       # HTTP controllers
│   │   ├── logic/            # Business logic
│   │   ├── model/            # Data models and persistence (GORM)
│   │   ├── dto/              # Request/response DTOs
│   │   ├── router/           # Route registration
│   │   ├── asr/              # ASR client
│   │   ├── ffmpeg/           # FFmpeg local executor
│   │   ├── repair/           # Video repair executor
│   │   ├── subtitle/         # Subtitle parsing
│   │   ├── scanner/          # Video directory scanner
│   │   └── scheduler/        # Task scheduler
│   ├── enum/                 # Business error codes
│   └── utils/                # Logging, response wrappers
├── frontend/
│   ├── src/
│   │   ├── api/              # API request wrappers
│   │   ├── views/            # Pages (videos/tasks/settings)
│   │   ├── stores/           # Pinia state
│   │   ├── router/           # Routes
│   │   └── utils/            # Utility functions
│   └── vite.config.ts        # Includes /api proxy config
├── Dockerfile                # Multi-stage build
└── LICENSE
```

## 🛡️ Notes

- **FFmpeg required**: `ffmpeg.provider` is fixed to `local`; the Docker image has ffmpeg built in. When running from source, install it yourself.
- **Video repair needs Docker**: When deploying in a container, mount the host Docker socket. If you don't use this feature, you can ignore it-it won't affect service startup.
- **Bring your own ASR service**: Deploy [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) yourself and configure `asr.url`.
- **Database persistence**: Defaults to `data/app.db`. When deploying with Docker, be sure to mount the `/app/data` directory, or data will be lost on restart.

## ❓ FAQ

<details>
<summary><b>What if startup reports <code>ffmpeg not found</code>?</b></summary>

`ffmpeg.provider` defaults to `local`, which requires ffmpeg to exist locally (or inside the image). The Docker image already includes it; when running from source, install ffmpeg or switch to `ssh` remote mode in the config (fill in the SSH host info, and the backend will invoke the remote ffmpeg over SSH).

</details>

<details>
<summary><b>Video repair isn't working?</b></summary>

Repair depends on Docker. When deploying in a container, mount the host Docker socket: `-v /var/run/docker.sock:/var/run/docker.sock`. If you don't use this feature, you can ignore it-it won't affect service startup.

</details>

<details>
<summary><b>How do I change the backend port?</b></summary>

The environment variable `APP_HTTP_PORT` overrides `http.port` (shared by the backend Viper config and the frontend Vite proxy). Just run `docker run -e APP_HTTP_PORT=9090 -p 9090:9090`.

</details>

<details>
<summary><b>Where is the database?</b></summary>

Defaults to `data/app.db` (SQLite). When deploying with Docker, mount the `/app/data` directory for persistence.

</details>

## 🗺️ Roadmap

- ✅ Video scanning + subtitle generation + video repair
- ✅ Task management + real-time progress + failure retry
- ✅ Online runtime config editing + persistence
- ✅ Docker deployment + runtime port specification
- ✅ FFmpeg smart local invocation
- 🔲 Subtitle online preview / editing
- 🔲 Subtitle file export & download
- 🔲 Multi-architecture images (native arm64 support)

## 💌 Acknowledgements

This project stands on the shoulders of giants. Special thanks to the following open-source projects and their authors:

- **[Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)** - by [@ahmetoner](https://github.com/ahmetoner), an ASR HTTP service based on OpenAI Whisper. VideoFlow's speech recognition is built entirely on this.
- **[FFmpeg](https://ffmpeg.org/)** - A powerful audio/video processing tool, responsible for audio extraction and duration probing.
- **[ladaapp/lada](https://github.com/ladaapp/lada)** - The video repair Docker image.
- **[Gin](https://github.com/gin-gonic/gin)** / **[GORM](https://github.com/go-gorm/gorm)** / **[Viper](https://github.com/spf13/viper)** / **[Zap](https://github.com/uber-go/zap)** - Excellent Go foundation libraries.
- **[Vue 3](https://vuejs.org/)** / **[Element Plus](https://element-plus.org/)** / **[Vite](https://vite.dev/)** - The frontend foundation.

## ⭐ Star History

[![Stargazers over time](https://api.star-history.com/svg?repos=StudyNoWeekend/videoFlow&type=Date)](https://star-history.com/#StudyNoWeekend/videoFlow&Date)

## 📜 License

[MIT License](./LICENSE) - Free to use, modify, and distribute, just keep the copyright notice.

---

<div align="center">

**Found it useful? A ⭐ is the biggest encouragement for the author.**

[⬆ Back to top](#videoflow) · [📥 Quick Start](#-quick-start) · [💬 Open an Issue](https://github.com/StudyNoWeekend/videoFlow/issues)

</div>