<div align="center">

# VideoFlow

### 给 Whisper + FFmpeg 装上 Web 可视化管理 —— 扫描视频目录，字幕自动生成，损坏视频一键修复。

基于 Whisper ASR + FFmpeg + Docker 修复引擎，用 Vue 3 + Element Plus 打造的 Web 可视化管理界面。告别脚本拼装，扫描入库、创建任务、实时进度、在线配置，全部在浏览器里完成。后端 Go（Gin + GORM + SQLite），支持 Docker 一键部署。

[![Stars](https://img.shields.io/github/stars/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=yellow)](https://github.com/StudyNoWeekend/videoFlow/stargazers)
[![Forks](https://img.shields.io/github/forks/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=blue)](https://github.com/StudyNoWeekend/videoFlow/network/members)
[![Issues](https://img.shields.io/github/issues/StudyNoWeekend/videoFlow?style=flat-square&logo=github)](https://github.com/StudyNoWeekend/videoFlow/issues)
[![License](https://img.shields.io/github/license/StudyNoWeekend/videoFlow?style=flat-square&color=green)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?style=flat-square&logo=vue.js&logoColor=white)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white)](./Dockerfile)

[💡 项目亮点](#-项目亮点) · [🎯 谁会想用](#-谁会想用) · [✨ 功能特性](#-功能特性) · [📥 快速开始](#-快速开始) · [🎬 使用流程](#-使用流程) · [🌐 技术栈](#-技术栈) · [🔧 配置](#-配置) · [📊 API 接口](#-api-接口) · [❓ FAQ](#-faq) · [🗺️ Roadmap](#️-roadmap)

🌐 **简体中文** · [繁體中文](./README.zh-TW.md) · [English](./README.en.md) · [日本語](./README.ja.md)

</div>

---

## 💡 项目亮点

本项目最大的亮点是：**把 Whisper 语音识别、FFmpeg 音频处理、Docker 视频修复这三件本来要拼脚本才能串起来的事，整合成了一套开箱即用的 Web 可视化管理平台**。

单独用 Whisper / FFmpeg 并不难，但要把「扫描目录 -> 提取音频 -> 调 ASR -> 生成字幕 -> 任务并发 -> 进度跟踪 -> 失败重试 -> 在线改配置」串成完整流水线，命令行和脚本很难做得顺手。VideoFlow 在不改动底层引擎的前提下，给它套上了一层 Web UI：

| 对比 | 手动拼装（Whisper + FFmpeg 脚本） | VideoFlow（本项目） |
| --- | --- | --- |
| 交互方式 | 命令行参数 + shell 脚本 | 浏览器图形界面，鼠标点点点 |
| 字幕生成 | 手动提取音频、调 ASR、拼字幕文件 | 一键创建任务，自动 提取音频 -> ASR -> 生成字幕 |
| 视频修复 | 手动跑 Docker 命令、盯终端 | 一键修复任务，支持 CPU / CUDA / MPS / XPU |
| 任务管理 | 自己记，关掉终端就没了 | 任务列表 + 实时进度 + 失败重试 + 历史可查 |
| 并发控制 | 自己写队列 / 信号量 | 调度器按配置并发数自动调度 |
| 配置修改 | 改配置文件、重启服务 | 在线修改，保存即热生效，持久化到数据库 |
| 部署形态 | 一堆依赖要装 | Docker 单镜像，挂载配置即跑 |

底层复用 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) 的识别能力、[FFmpeg](https://ffmpeg.org/) 的音视频处理、[`ladaapp/lada`](https://hub.docker.com/r/ladaapp/lada) 的视频修复，在其之上封装出 HTTP API 与 Web 前端 —— **命令行的能力，图形界面的体验**。

## 🎯 谁会想用

| 你是 | 你能用它做什么 |
| --- | --- |
| **视频创作者 / 自媒体** | 批量给视频生成字幕，省去手动听写，导出 SRT / VTT 直接用 |
| **字幕组 / 翻译爱好者** | 用 Whisper 批量转录生成时间轴，再人工校对，效率翻倍 |
| **有损坏视频的人** | 视频打不开？用 lada Docker 一键修复，支持多种计算设备 |
| **NAS / 家庭服务器玩家** | Docker 长期挂着，定时扫描目录自动入库，新视频自动生成字幕 |
| **想本地跑 Whisper 的人** | 不想写脚本，Web 界面配置 ASR 参数（语言 / VAD / 提示词）即可 |
| **嫌命令行麻烦的人** | 全程图形界面，配置、扫描、任务进度一目了然 |
| **远程 ffmpeg 用户** | ffmpeg 支持 SSH 远程模式，本地不装 ffmpeg 也能用远程机器处理 |

## ✨ 功能特性

- **视频目录扫描** - 手动扫描或后台定时自动扫描，视频自动入库，卡片式分页展示
- **字幕生成** - 基于 Whisper ASR，支持语言、VAD 过滤、任务类型、音频预编码、初始提示词、词级时间戳、多种输出格式（json / srt / vtt / txt / tsv）
- **视频修复** - 基于 Docker 镜像 `ladaapp/lada`，支持 CPU / NVIDIA CUDA / Apple MPS / Intel XPU 四种计算设备
- **任务管理** - 创建 / 查询 / 删除 / 失败重试，按类型过滤，实时进度条，前端 2 秒轮询刷新
- **任务调度器** - 后台调度器按配置的并发数限制执行字幕 / 修复任务
- **运行时配置** - 统一配置页，所有配置在线修改并持久化（SQLite），保存即热生效
- **FFmpeg 双模式** - 本地直接调用，或通过 SSH 调用远程 ffmpeg，运行时可热切换
- **工程化** - `trace_id` 全链路追踪、统一响应结构、zap 结构化日志、优雅关闭（重启时运行中任务自动标记失败）

> 翻译功能（基于 [Ollama](https://github.com/ollama/ollama)）后端已实现，前端入口暂未启用，后续开放。

## 📥 快速开始

### 方式零：一键脚本部署（最推荐）

项目提供 [`install.sh`](./install.sh) 一键安装脚本，自动完成以下全部步骤：

- 检测 Docker 环境
- 部署 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)（支持 CPU/GPU、引擎与模型选择）
- 拉取 [`ladaapp/lada`](https://hub.docker.com/r/ladaapp/lada) 视频修复镜像
- 安装 [FFmpeg](https://ffmpeg.org/)（支持 macOS / Linux / Windows）
- 引导配置并启动本项目容器

```bash
bash install.sh
```

脚本采用交互式引导，按提示选择即可。完成后会输出各服务的访问地址。

<details>
<summary><b>脚本支持的可选项</b></summary>

- **运行模式**：CPU 模式（全平台通用）/ GPU 模式（仅 Linux + NVIDIA GPU）
- **ASR 引擎**：`openai_whisper`（默认）/ `faster_whisper` / `whisperx`
- **字幕模型**：`tiny` / `base` / `small` / `medium` / `large-v3` / `large-v3-turbo` 及英文专用模型
- **模型缓存持久化**：加速后续启动
- **FFmpeg 安装**：自动识别包管理器（Homebrew / apt / dnf / yum / pacman / apk / winget / choco）

</details>

### 方式一：拉取镜像部署

镜像已发布到 GitHub Container Registry：

```bash
docker pull ghcr.io/studynoweekend/videoflow:latest
```

1. 准备配置文件（基于模板修改）：

```bash
cp backend/config/config.yaml.local config.yaml
# 按需修改 config.yaml：asr.url、video.dir 等
```

2. 启动容器（端口、配置、数据、日志均可在 `docker run` 时指定）：

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

3. 健康检查：

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"},"code":0,"msg":"success","trace_id":"..."}
```

**更换端口**：只需改两处，端口真源单一：

```bash
docker run -d --name videoflow \
  -e APP_HTTP_PORT=9090 -p 9090:9090 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  ghcr.io/studynoweekend/videoflow:latest
```

<details>
<summary><b>方式二：本地构建镜像</b></summary>

```bash
docker build -t video-captions:latest -f Dockerfile .
docker run -d --name videoflow \
  -e APP_HTTP_PORT=8080 -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  video-captions:latest
```

Dockerfile 为多阶段构建：华为云 `golang:1.25-alpine` 编译（CGO）+ `alpine:3.20` 运行（内置 ffmpeg）。镜像为 `linux/amd64`，Apple Silicon 上通过 QEMU 运行。

</details>

### 方式三：本地开发部署

**前置要求：** Go 1.25+、Node.js 22.18+、FFmpeg，并已运行 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)。

```bash
# 后端
cd backend
cp config/config.yaml.local config/config.yaml   # 按需修改
go run ./cmd/api

# 前端（另开终端）
cd frontend
npm install
npm run dev
```

开发态前端通过 Vite 代理将 `/api` 转发到后端，端口由环境变量 `APP_HTTP_PORT` 统一控制（默认 8080，与后端共用）：

```bash
APP_HTTP_PORT=9090 npm run dev        # 前端
APP_HTTP_PORT=9090 go run ./cmd/api   # 后端
```

浏览器打开 Vite 提示的本地地址即可。

## 🎬 使用流程

1. 启动服务，浏览器访问前端地址
2. 在「设置」页配置本地视频目录与 ASR 服务地址，保存
3. 在「视频列表」页点击扫描，视频自动入库
4. 点视频卡片上的「生成字幕」，创建字幕任务
5. 在「任务管理」页查看实时进度（2 秒自动刷新）
6. （可选）点「视频修复」修复损坏视频
7. 字幕任务完成后，查看生成的字幕文件

## 🌐 技术栈

| 层 | 技术 | 说明 |
| --- | --- | --- |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) | HTTP 路由与中间件 |
| ORM | [GORM](https://github.com/go-gorm/gorm) | 数据持久化 |
| 数据库 | SQLite（[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)） | CGO 驱动 |
| 配置 | [Viper](https://github.com/spf13/viper) | 配置文件 + 环境变量覆盖 |
| 日志 | [Zap](https://github.com/uber-go/zap) | 结构化日志 |
| ASR | [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) | 语音识别引擎 |
| 音视频 | [FFmpeg](https://ffmpeg.org/) / ffprobe | 音频提取、时长探测，支持本地 / SSH |
| 视频修复 | [`ladaapp/lada`](https://hub.docker.com/r/ladaapp/lada) | Docker 修复引擎 |
| 翻译 | [Ollama](https://github.com/ollama/ollama) | 本地大模型推理（翻译功能） |
| 前端 | [Vue 3](https://vuejs.org/) + [Element Plus](https://element-plus.org/) + [Vite](https://vite.dev/) | 图形界面 |
| 状态管理 | [Pinia](https://pinia.vuejs.org/) | 前端状态 |
| HTTP 客户端 | [Axios](https://axios-http.com/) | 接口请求 |

## 🔧 配置

配置优先级：**环境变量 > config.yaml > 代码默认值**。运行时还可在前端「设置」页在线修改并持久化到数据库。

<details>
<summary><b>配置文件（config/config.yaml）</b></summary>

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
translation:
  ollama_url: http://localhost:11434/api/generate
  model: qwen3.5:0.8b
concurrency:
  subtitle: 2
  repair: 1
  translate: 1
```

</details>

<details>
<summary><b>环境变量（Docker 部署用）</b></summary>

viper 前缀 `APP_`，配置键 `.` -> `_`，故 `http.port` 对应环境变量 `APP_HTTP_PORT`，依此类推：

| 环境变量 | 配置项 | 默认值 |
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
<summary><b>Docker 卷挂载点</b></summary>

| 容器路径 | 用途 |
| --- | --- |
| `/app/config/config.yaml` | 配置文件（只读挂载） |
| `/app/data` | SQLite 数据库持久化（`data/app.db`） |
| `/app/logs` | 日志输出 |
| `/var/run/docker.sock` | （可选）视频修复需要挂载宿主机 Docker socket |

</details>

## 📊 API 接口

所有接口统一前缀 `/api/v1`，响应结构：

```json
{
  "code": 0,
  "msg": "success",
  "data": {},
  "trace_id": "xxx"
}
```

`code=0` 表示成功，非 0 表示业务错误。

<details>
<summary><b>健康检查</b></summary>

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 健康检查 |
| `GET` | `/ready` | 就绪检查 |

</details>

<details>
<summary><b>视频接口</b></summary>

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/videos/scan` | 扫描视频目录入库 |
| `GET` | `/api/v1/videos` | 分页查询视频列表 |
| `PUT` | `/api/v1/videos/:id` | 更新视频信息 |
| `DELETE` | `/api/v1/videos/:id` | 删除视频记录 |

</details>

<details>
<summary><b>任务接口</b></summary>

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | 创建任务（字幕 / 修复） |
| `GET` | `/api/v1/tasks` | 分页查询任务列表（可按类型过滤） |
| `POST` | `/api/v1/tasks/:id/retry` | 重试失败任务 |
| `DELETE` | `/api/v1/tasks/:id` | 删除任务 |

任务状态：`pending`（待处理）-> `running`（进行中）-> `completed`（已完成）/ `failed`（失败）

</details>

<details>
<summary><b>运行时配置接口</b></summary>

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 获取运行时配置 |
| `PUT` | `/api/v1/settings` | 更新运行时配置（保存即热生效） |

</details>

## 📁 项目结构

```
videoFlow/
├── backend/
│   ├── cmd/api/              # 程序入口 main.go
│   ├── bootstrap/            # 配置、DB、ASR、FFmpeg、翻译、修复 等初始化
│   ├── config/               # 配置文件（config.yaml.local 为模板）
│   ├── internal/
│   │   ├── controller/       # HTTP 控制器
│   │   ├── logic/            # 业务逻辑
│   │   ├── model/            # 数据模型与持久化（GORM）
│   │   ├── dto/              # 请求/响应 DTO
│   │   ├── router/           # 路由注册
│   │   ├── asr/              # ASR 客户端
│   │   ├── ffmpeg/           # FFmpeg 本地/SSH 执行器
│   │   ├── repair/           # 视频修复执行器
│   │   ├── translation/      # Ollama 翻译
│   │   ├── subtitle/         # 字幕解析
│   │   ├── scanner/          # 视频目录扫描器
│   │   └── scheduler/        # 任务调度器
│   ├── enum/                 # 业务错误码
│   └── utils/                # 日志、响应封装
├── frontend/
│   ├── src/
│   │   ├── api/              # 接口请求封装
│   │   ├── views/            # 页面（视频/任务/设置）
│   │   ├── stores/           # Pinia 状态
│   │   ├── router/           # 路由
│   │   └── utils/            # 工具函数
│   └── vite.config.ts        # 含 /api 代理配置
├── Dockerfile                # 多阶段构建
└── LICENSE
```

## 🛡️ 注意事项

- **FFmpeg 必需**：`ffmpeg.provider` 默认 `local`，镜像已内置 ffmpeg；本地源码运行需自行安装，或切换为 `ssh` 远程模式
- **视频修复需 Docker**：容器部署时需挂载宿主机 Docker socket；不用该功能可忽略，不影响服务启动
- **ASR 服务需自备**：请自行部署 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)，并配置 `asr.url`
- **数据库持久化**：默认 `data/app.db`，Docker 部署时务必挂载 `/app/data` 目录，否则重启丢数据

## ❓ FAQ

<details>
<summary><b>启动报 <code>ffmpeg not found</code> 怎么办？</b></summary>

`ffmpeg.provider` 默认为 `local`，需要本地（或镜像内）存在 ffmpeg。Docker 镜像已内置；源码运行请安装 ffmpeg，或在配置中切换为 `ssh` 远程模式（填写 SSH 主机信息，后端通过 SSH 调用远程 ffmpeg）。

</details>

<details>
<summary><b>视频修复功能不可用？</b></summary>

修复依赖 Docker。容器部署时需挂载宿主机 Docker socket：`-v /var/run/docker.sock:/var/run/docker.sock`；不用该功能可忽略，不影响服务启动。

</details>

<details>
<summary><b>如何更换后端端口？</b></summary>

环境变量 `APP_HTTP_PORT` 覆盖 `http.port`（后端 viper 与前端 Vite 代理共用此变量）。`docker run -e APP_HTTP_PORT=9090 -p 9090:9090` 即可。

</details>

<details>
<summary><b>数据库在哪？</b></summary>

默认 `data/app.db`（SQLite）。Docker 部署时挂载 `/app/data` 目录以持久化。

</details>

<details>
<summary><b>翻译功能什么时候开放？</b></summary>

翻译（基于 Ollama）后端逻辑已实现，前端入口暂未启用，后续版本开放。可自行在 `frontend/src/views/VideosView.vue` 中解除注释启用。

</details>

## 🗺️ Roadmap

- ✅ 视频扫描 + 字幕生成 + 视频修复
- ✅ 任务管理 + 实时进度 + 失败重试
- ✅ 运行时配置在线修改 + 持久化
- ✅ Docker 部署 + 端口运行时指定
- ✅ FFmpeg 本地 / SSH 双模式
- 🔲 翻译功能前端入口开放
- 🔲 字幕在线预览 / 编辑
- 🔲 字幕文件导出下载
- 🔲 多架构镜像（arm64 原生支持）

## 💌 致谢

本项目站在巨人的肩膀上，特别感谢以下开源项目及其作者：

- **[Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)** - by [@ahmetoner](https://github.com/ahmetoner)，基于 OpenAI Whisper 的 ASR HTTP 服务，VideoFlow 的语音识别能力全部基于此。
- **[FFmpeg](https://ffmpeg.org/)** - 强大的音视频处理工具，负责音频提取与时长探测。
- **[ladaapp/lada](https://hub.docker.com/r/ladaapp/lada)** - 视频修复 Docker 镜像。
- **[Ollama](https://github.com/ollama/ollama)** - 本地大模型推理引擎，为翻译功能提供支持。
- **[Gin](https://github.com/gin-gonic/gin)** / **[GORM](https://github.com/go-gorm/gorm)** / **[Viper](https://github.com/spf13/viper)** / **[Zap](https://github.com/uber-go/zap)** - 优秀的 Go 基础库。
- **[Vue 3](https://vuejs.org/)** / **[Element Plus](https://element-plus.org/)** / **[Vite](https://vite.dev/)** - 前端基石。

## ⭐ Star history

[![Stargazers over time](https://api.star-history.com/svg?repos=StudyNoWeekend/videoFlow&type=Date)](https://star-history.com/#StudyNoWeekend/videoFlow&Date)

## 📜 协议

[MIT License](./LICENSE) - 自由使用、修改、分发，只需保留版权声明。

---

<div align="center">

**用过觉得有用？给个 ⭐ 是对作者最大的鼓励。**

[⬆ 回到顶部](#videoflow) · [📥 快速开始](#-快速开始) · [💬 提 Issue](https://github.com/StudyNoWeekend/videoFlow/issues)

</div>
