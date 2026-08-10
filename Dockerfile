# ====== 阶段 1：构建前端 ======
FROM node:22-alpine AS frontend-builder

# 前端构建参数（仅构建阶段生效，不影响运行时）
ARG VITE_API_BASE_URL=/api
ARG VITE_APP_TITLE=VideoFlow

WORKDIR /frontend

# 先拷贝依赖清单，利用层缓存
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# 拷贝源码并构建
COPY frontend/ ./
RUN npm run build-only

# ====== 阶段 2：构建后端 ======
# 使用官方 golang 镜像（支持 amd64 + arm64 多架构，配合 buildx 使用）
FROM golang:1.25-alpine AS backend-builder

# CGO 依赖：gorm 的 sqlite 驱动基于 mattn/go-sqlite3，需要 gcc + musl-dev
# git：go mod download 拉取部分依赖时需要
# 使用国内镜像加速 apk
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.ustc.edu.cn|g' /etc/apk/repositories && \
    apk add --no-cache gcc musl-dev git

# CGO_ENABLED=1 以链接 sqlite 驱动
# 不固定 GOARCH，由 buildx --platform 自动注入 TARGETARCH
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build

# 先拷贝依赖清单，利用层缓存
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# 拷贝源码并编译
COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/video-captions ./cmd/api

# ====== 阶段 3：运行时 ======
FROM alpine:3.20

# 运行时依赖：
#  - ffmpeg（含 ffprobe）：ffmpeg.provider=local 时必需
#  - openssh-client：ffmpeg.provider=ssh 时调用远程 ffmpeg 需要 ssh
#  - ca-certificates：调用 ASR 等 HTTPS 服务
#  - tzdata：时区
#  - nginx：前端静态资源服务 + 反向代理 /api 到后端
#  - docker-cli：调用宿主机 Docker（需挂载 /var/run/docker.sock）
# 使用国内镜像加速 apk（ffmpeg 依赖较多，镜像加速明显）
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.ustc.edu.cn|g' /etc/apk/repositories && \
    apk add --no-cache ffmpeg openssh-client ca-certificates tzdata nginx docker-cli

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

# 标识当前运行环境为 Docker 容器，业务代码据此判断是否在容器内部运行
ENV CONTAINER_RUNTIME=docker

# 后端端口固定 8080（容器内部，不暴露）。
# 仅暴露 nginx 的 80 端口（前端），docker run 时用 -p <宿主端口>:80 指定。
EXPOSE 80

ENTRYPOINT ["/app/entrypoint.sh"]
