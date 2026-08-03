#!/bin/sh
set -e

# 启动后端服务（端口固定 8080，仅容器内部访问）
/app/video-captions &

# 等待后端就绪
echo "等待后端服务启动..."
for i in $(seq 1 30); do
    if wget -q -O- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then
        echo "后端服务已就绪"
        break
    fi
    sleep 1
done

# 启动 nginx（前台运行，监听 80 端口，对外暴露）
echo "启动 nginx..."
nginx -g 'daemon off;'
