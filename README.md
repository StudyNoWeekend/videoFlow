# VideoFlow

> 一个开箱即用的视频字幕生成与视频修复平台：扫描本地视频 → 调用 Whisper ASR 生成字幕 → 可选视频修复（Docker），全程任务化、可并发、可在线配置。

后端 Go（Gin + GORM + SQLite），前端 Vue 3 + Vite + Element Plus，外部依赖 FFmpeg、Whisper ASR Webservice。支持 Docker 一键部署。

---

## 目录

- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [依赖说明](#依赖说明)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [API 接口](#api-接口)
- [项目结构](#项目结构)
- [开发指南](#开发指南)
- [常见问题](#常见问题)
- [贡献](#贡献)
- [开源协议](#开源协议)

---

## 功能特性

### 视频管理
- 扫描本地视频目录入库（手动扫描 + 后台定时自动扫描）
- 卡片式分页列表，展示名称、路径、时长、大小
- 每个视频关联其字幕/修复任务及实时进度
- 删除视频记录

### 字幕生成
- 基于 [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) 语音识别
- 通过 FFmpeg 提取音频后送入 ASR
- 可配置项：语言、VAD 过滤、任务类型（transcribe / translate）、音频预编码、初始提示词、词级时间戳、输出格式（json / srt / vtt / txt / tsv）
- 任务化异步执行，支持进度跟踪与失败重试

### 视频修复
- 基于 Docker 镜像 [`ladaapp/lada`](https://hub.docker.com/r/ladaapp/lada) 修复损坏视频
- 支持多种计算设备：CPU / NVIDIA CUDA / Apple Silicon MPS / Intel XPU
- 任务化执行，与字幕任务共享调度器

### 任务管理
- 统一任务列表，按类型（字幕 / 修复）过滤
- 状态机：待处理（pending）→ 进行中（running）→ 已完成（completed）/ 失败（failed）
- 实时进度条，前端 2 秒轮询刷新
- 失败任务一键重试，历史任务删除
- 后台任务调度器按配置的并发数限制执行

### 运行时配置
- 统一配置页面，所有配置可在线修改并持久化（SQLite `settings` 表），保存即热生效
- 覆盖：视频目录、扫描间隔、ASR 全套参数、修复镜像与设备、各任务并发数

### 工程特性
- `trace_id` 全链路追踪（请求头 `X-Trace-ID` 透传）
- 统一响应结构 `{ code, msg, data, trace_id }`
- zap 结构化日志
- 优雅关闭：服务重启时将运行中任务自动标记为失败，避免脏状态
- 端口、配置、数据、日志均可通过环境变量 / 挂载文件在运行时指定

> 翻译功能（基于 Ollama）后端已实现，前端入口暂未启用，后续开放。

---

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.25、Gin、GORM、SQLite（mattn/go-sqlite3）、Viper、Zap |
| 前端 | Vue 3、Vite、TypeScript、Element Plus、Pinia、Vue Router、Axios |
| 外部服务 | FFmpeg / ffprobe、Whisper ASR Webservice、Docker（修复用，可选）、Ollama（翻译用，可选） |

---

## 依赖说明

### 运行时外部依赖

| 依赖 | 用途 | 是否必需 |
| --- | --- | --- |
| **FFmpeg / ffprobe** | 提取音频、获取时长；`ffmpeg.provider=local` 时必需 | 必需（本地模式） |
| **Whisper ASR Webservice** | 语音识别生成字幕 | 必需 |
| **Docker** | 视频修复执行环境；不可用时仅告警，不阻塞启动 | 可选（仅修复功能） |
| **Ollama** | 字幕翻译；前端入口暂未启用 | 可选 |

> FFmpeg 也支持 `ssh` 远程模式：在配置中设置 `ffmpeg.provider=ssh` 并填写 SSH 主机信息，即可让后端通过 SSH 调用远程 ffmpeg，本地无需安装。

### 后端 Go 依赖

| 模块 | 版本 | 说明 |
| --- | --- | --- |
| github.com/gin-gonic/gin | v1.12.0 | HTTP 框架 |
| gorm.io/gorm | v1.31.2 | ORM |
| gorm.io/driver/sqlite | v1.6.0 | SQLite 驱动（基于 mattn/go-sqlite3 v1.14.22，需 CGO） |
| github.com/spf13/viper | v1.21.0 | 配置管理（支持环境变量覆盖） |
| go.uber.org/zap | v1.28.0 | 结构化日志 |
| github.com/google/uuid | v1.6.0 | UUID 生成 |

### 前端依赖

| 包 | 版本 | 说明 |
| --- | --- | --- |
| vue | ^3.5.38 | 视图层 |
| vue-router | ^5.1.0 | 路由 |
| pinia | ^3.0.4 | 状态管理 |
| element-plus | ^2.14.3 | UI 组件库 |
| axios | ^1.18.1 | HTTP 客户端 |
| vite | ^8.0.16 | 构建工具 |
| typescript | ~6.0.0 | 类型系统 |

> Node 版本要求：`^22.18.0 || >=24.12.0`（见 `frontend/package.json` 的 `engines`）。

---

## 快速开始

### 方式一：Docker（推荐）

镜像已发布到 GitHub Container Registry：

```bash
docker pull ghcr.io/studynoweekend/videoflow:latest
```

1. 准备配置文件（基于模板修改）：

```bash
cp backend/config/config.yaml.local config.yaml
# 按需修改 config.yaml：asr.url、video.dir、http.port 等
```

2. 启动容器（端口、配置、数据、日志均可在 `docker run` 时指定）：

```bash
docker run -d --name videoflow \
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

> 镜像为 `linux/amd64` 架构，在 Apple Silicon 上通过 QEMU 运行；部署到 amd64 服务器原生执行。
> 若需启用视频修复，额外挂载宿主机 Docker socket：`-v /var/run/docker.sock:/var/run/docker.sock`。

### 方式二：源码运行（开发）

前置：本地已安装 Go 1.25+、Node.js 22.18+、FFmpeg，并已运行 Whisper ASR Webservice。

**后端：**

```bash
cd backend
cp config/config.yaml.local config/config.yaml   # 按需修改
go run ./cmd/api
```

**前端：**

```bash
cd frontend
npm install
npm run dev
```

开发态前端通过 Vite 代理将 `/api` 转发到后端，端口由环境变量 `APP_HTTP_PORT` 统一控制（默认 8080，与后端共用）：

```bash
APP_HTTP_PORT=9090 npm run dev   # 前端
APP_HTTP_PORT=9090 go run ./cmd/api   # 后端
```

浏览器打开 Vite 提示的本地地址即可。

---

## 配置说明

配置通过 `config.yaml` 加载，支持环境变量覆盖（viper，前缀 `APP_`，`.` → `_`，如 `http.port` → `APP_HTTP_PORT`）。运行时还可在前端「设置」页在线修改并持久化到数据库。

| 配置项 | 说明 | 默认值 |
| --- | --- | --- |
| `http.port` | 后端监听端口（可被 `APP_HTTP_PORT` 覆盖） | `8080` |
| `video.dir` | 本地视频目录 | `""` |
| `scan.interval` | 自动扫描间隔（秒） | `60` |
| `asr.url` | Whisper ASR 服务地址 | `http://127.0.0.1:9999/asr` |
| `asr.language` | 识别语言 | `zh` |
| `asr.output` | 输出格式 txt/vtt/srt/tsv/json | `json` |
| `repair.docker_image` | 修复用的 Docker 镜像 | `ladaapp/lada:latest` |
| `repair.device` | 修复计算设备 cpu/cuda:0/mps/xpu:0 | `cpu` |
| `concurrency.subtitle` | 字幕任务并发数 | `2` |
| `concurrency.repair` | 修复任务并发数 | `1` |
| `ffmpeg.provider` | FFmpeg 执行环境 local/ssh | `local` |
| `database.dsn` | SQLite 数据库路径 | `data/app.db` |

完整字段见 [`backend/config/config.yaml.local`](backend/config/config.yaml.local)。

---

## API 接口

所有接口统一前缀 `/api/v1`，响应结构 `{ code, msg, data, trace_id }`（`code=0` 表示成功）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health`、`/ready` | 健康检查 |
| POST | `/api/v1/videos/scan` | 扫描视频目录入库 |
| GET | `/api/v1/videos` | 分页查询视频列表 |
| PUT | `/api/v1/videos/:id` | 更新视频信息 |
| DELETE | `/api/v1/videos/:id` | 删除视频记录 |
| POST | `/api/v1/tasks` | 创建任务（字幕 / 修复） |
| GET | `/api/v1/tasks` | 分页查询任务列表（可按类型过滤） |
| POST | `/api/v1/tasks/:id/retry` | 重试失败任务 |
| DELETE | `/api/v1/tasks/:id` | 删除任务 |
| GET | `/api/v1/settings` | 获取运行时配置 |
| PUT | `/api/v1/settings` | 更新运行时配置 |

---

## 项目结构

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

---

## 开发指南

```bash
# 前端类型检查
cd frontend && npm run type-check

# 前端构建
npm run build

# 后端编译（注意 CGO，因 sqlite 驱动）
cd backend && CGO_ENABLED=1 go build -o bin/video-captions ./cmd/api

# 前端代码规范
cd frontend && npm run lint
```

### 构建镜像

```bash
docker build -t video-captions:latest -f Dockerfile .
```

Dockerfile 为多阶段构建：华为云 `golang:1.25-alpine` 编译（CGO）+ `alpine:3.20` 运行（内置 ffmpeg）。详见 [Dockerfile](Dockerfile)。

---

## 常见问题

**Q：启动报 `ffmpeg not found`？**
A：`ffmpeg.provider` 默认为 `local`，需要本地（或镜像内）存在 ffmpeg。可安装 ffmpeg，或在配置中切换为 `ssh` 远程模式。

**Q：视频修复功能不可用？**
A：修复依赖 Docker。容器部署时需挂载宿主机 Docker socket；不用该功能可忽略，不影响服务启动。

**Q：如何更换后端端口？**
A：环境变量 `APP_HTTP_PORT` 覆盖 `http.port`（后端 viper 与前端 Vite 代理共用此变量）。

**Q：数据库在哪？**
A：默认 `data/app.db`（SQLite）。Docker 部署时挂载 `/app/data` 目录以持久化。

---

## 贡献

欢迎提 Issue 和 Pull Request。

1. Fork 本仓库
2. 新建分支：`git checkout -b feat/your-feature`
3. 提交更改：`git commit -m "feat: ..."`
4. 推送分支：`git push origin feat/your-feature`
5. 发起 Pull Request

请确保提交前通过 `npm run type-check` / `npm run lint` 与 `go build`。

---

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。

Copyright (c) 2026 StudyNoWeekend
