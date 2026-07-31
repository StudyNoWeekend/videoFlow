# ====== 构建阶段 ======
# 使用指定的华为云 golang 镜像（该镜像仅 amd64，故显式指定平台，保证构建/运行阶段架构一致）
FROM --platform=linux/amd64 swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/golang:1.25-alpine AS builder

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

# ====== 运行阶段 ======
# 与构建阶段同架构（amd64），保证二进制可执行
FROM --platform=linux/amd64 alpine:3.20

# 运行时依赖：
#  - ffmpeg（含 ffprobe）：ffmpeg.provider=local 时必需（config 未配 ffmpeg 段时默认 local，启动会校验）
#  - openssh-client：ffmpeg.provider=ssh 时调用远程 ffmpeg 需要 ssh
#  - ca-certificates：调用 ASR / Ollama 等 HTTPS 服务
#  - tzdata：时区
RUN apk add --no-cache ffmpeg openssh-client ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/video-captions /app/video-captions

# 配置 / 数据 / 日志均通过 volume 挂载，镜像内不内置：
#  - /app/config/config.yaml：挂载配置文件
#  - /app/data：sqlite 持久化（config.database.dsn = data/app.db）
#  - /app/logs：日志
VOLUME ["/app/config", "/app/data", "/app/logs"]

# 端口通过 APP_HTTP_PORT 指定（后端 viper AutomaticEnv 用其覆盖 http.port）。
# 默认 8080；docker run 时用 -e APP_HTTP_PORT=xxxx -p xxxx:xxxx 指定。
ENV APP_HTTP_PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/video-captions"]
