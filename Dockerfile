# ====== 阶段 1：构建前端 ======
FROM --platform=linux/amd64 node:22-alpine AS frontend-builder

WORKDIR /frontend

# 先拷贝依赖清单，利用层缓存
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# 拷贝源码并构建
COPY frontend/ ./
RUN npm run build-only

# ====== 阶段 2：构建后端 ======
# 使用指定的华为云 golang 镜像（该镜像仅 amd64，故显式指定平台，保证构建/运行阶段架构一致）
FROM --platform=linux/amd64 swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/golang:1.25-alpine AS backend-builder

# CGO 依赖：gorm 的 sqlite 驱动基于 mattn/go-sqlite3，需要 gcc + musl-dev
# git：go mod download 拉取部分依赖时需要
RUN apk add --no-cache gcc musl-dev git

# 国内构建加速；CGO_ENABLED=1 以链接 sqlite 驱动
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

# 先拷贝依赖清单，利用层缓存
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# 拷贝源码并编译
COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/video-captions ./cmd/api

# ====== 阶段 3：运行时 ======
# 与构建阶段同架构（amd64），保证二进制可执行
FROM --platform=linux/amd64 alpine:3.20

# 运行时依赖：
#  - ffmpeg（含 ffprobe）：ffmpeg.provider=local 时必需
#  - openssh-client：ffmpeg.provider=ssh 时调用远程 ffmpeg 需要 ssh
#  - ca-certificates：调用 ASR / Ollama 等 HTTPS 服务
#  - tzdata：时区
#  - nginx：前端静态资源服务 + 反向代理 /api 到后端
RUN apk add --no-cache ffmpeg openssh-client ca-certificates tzdata nginx

WORKDIR /app

# 拷贝后端二进制
COPY --from=backend-builder /out/video-captions /app/video-captions

# 拷贝前端构建产物
COPY --from=frontend-builder /frontend/dist /app/frontend/dist

# 拷贝 nginx 配置
COPY nginx.conf /etc/nginx/http.d/default.conf

# 拷贝启动脚本
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 拷贝默认配置（可被 volume 挂载覆盖）
COPY backend/config/config.yaml.local /app/config/config.yaml

# 配置 / 数据 / 日志均通过 volume 挂载：
#  - /app/config/config.yaml：挂载配置文件覆盖默认
#  - /app/data：sqlite 持久化（config.database.dsn = data/app.db）
#  - /app/logs：日志
VOLUME ["/app/config", "/app/data", "/app/logs"]

# 后端端口固定 8080（容器内部，不暴露）。
# 仅暴露 nginx 的 80 端口（前端），docker run 时用 -p <宿主端口>:80 指定。
EXPOSE 80

ENTRYPOINT ["/app/entrypoint.sh"]
