#!/bin/sh
set -e

# 启动后端服务（端口固定 8080，仅容器内部访问）
/app/video-captions &
BACKEND_PID=$!

# 等待后端就绪
echo "等待后端服务启动..."
for i in $(seq 1 30); do
    if wget -q -O- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then
        echo "后端服务已就绪"
        break
    fi
    sleep 1
done

# 启动 nginx（监听 80 端口，对外暴露）
echo "启动 nginx..."
nginx -g 'daemon off;' &
NGINX_PID=$!

# 优雅关闭：shell 作为 PID 1 不会自动向子进程转发信号，必须显式 trap 转发，
# 否则后端收不到 SIGTERM，无法执行优雅关闭逻辑，运行中的任务会永远停留在 running 状态。
cleanup() {
    set +e
    echo "收到终止信号，正在关闭服务..."
    kill -TERM "$BACKEND_PID" 2>/dev/null   # 后端：将 running 任务标记为失败、关闭 HTTP、停止调度器
    kill -QUIT "$NGINX_PID" 2>/dev/null      # nginx：优雅关闭
    wait "$BACKEND_PID" 2>/dev/null
    wait "$NGINX_PID" 2>/dev/null
}
trap cleanup TERM INT

# 阻塞等待子进程；收到信号时由 cleanup 转发并等待子进程优雅退出后再退出容器
wait
