# 方案 A：容器内嵌 dockerd（Docker-in-Docker）

## 背景

用户 NAS 限制了 Docker socket 挂载（`/var/run/docker.sock`），导致 videoFlow 的 Lada（去马赛克）和 Video2X（清晰度增强）功能不可用。需要将 dockerd 内嵌到 videoFlow 容器中，使其不依赖宿主机 Docker daemon。

## 设计原则

- **零改 Go 业务代码**：所有 `exec.Command("docker", ...)` 调用走同一个 `/var/run/docker.sock`，内嵌 dockerd 也监听该 socket，代码无需改动
- **自动降级**：用户不挂载 Docker socket 时自动启动内嵌 dockerd，也可用 `DOCKER_MODE` 环境变量显式控制
- **GPU 最佳努力**：保留 `--gpus` 透传能力，但不在 NAS 上保证可用
- **兼容现有用户**：现有 docker-compose 用户无感知，继续走宿主机 socket

## 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `Dockerfile` | 修改 | 安装 dockerd/containerd + 预置 daemon.json 配置 |
| `entrypoint.sh` | 修改 | dockerd 启动/健康检查/优雅关闭 + GPU 自动检测 |
| `backend/internal/utils/docker_dind.go` | **新增** | `IsDindMode()` 检测函数 |
| `backend/internal/repair/repair.go` | 修改 | `resolveHostPath()` DinD 短路返回原路径 |
| `backend/internal/upscale/upscale.go` | 修改 | `resolveHostPath()` DinD 短路返回原路径 |
| `backend/internal/component/detector.go` | 修改 | `detectDocker()` DinD 模式错误信息 |
| `backend/bootstrap/config.go` | 修改 | `AppConfigDocker` 结构体 |
| `backend/config/config.yaml` | 修改 | 添加 `docker.mode` 配置项 |
| `docker-compose.yml` | 修改 | 添加 DinD 注释说明 |
| `docker-compose.dind.yml` | **新增** | DinD 模式专用 compose 文件 |
| `backend/internal/repair/repair_test.go` | 修改 | 新增 DinD 模式测试 |
| `backend/internal/upscale/upscale_test.go` | 修改 | 新增 DinD 模式测试 |
| `README.md` | 修改 | DinD 部署文档 |
| README 其他语言（zh-TW/en/ja） | 修改 | 对应翻译 |

## Step 1：Dockerfile 改造 — 安装 dockerd + containerd

### 1.1 运行时镜像添加 Docker CE 软件包

在 Alpine 3.20 运行时镜像基础上，增加 Docker CE（包含 dockerd、containerd、runc、docker CLI）：

```dockerfile
# 安装 dockerd 及依赖工具
RUN apk add --no-cache \
    docker \
    docker-cli \
    docker-engine \
    # dockerd 运行依赖
    iptables \
    ip6tables \
    bridge \
    # 健康检查工具
    curl
```

**注意**：官方 `docker:dind` 镜像（Alpine 版）约 300MB，我们加上后运行时镜像预计膨胀 150-200MB。现有 `docker-cli` 包替换为完整 `docker` 包即可。

### 1.2 GPU 支持（构建参数 + 条件安装）

由于 Alpine 仓库可能不包含 `nvidia-container-cli`，通过构建参数和条件下载实现：

```dockerfile
ARG INSTALL_NVIDIA=false

# NVIDIA Container Toolkit 静态安装
RUN if [ "$INSTALL_NVIDIA" = "true" ]; then \
        NVIDIA_VERSION=v1.17.4 && \
        wget -qO /tmp/nvidia-toolkit.tar.gz \
            "https://github.com/NVIDIA/nvidia-container-toolkit/releases/download/${NVIDIA_VERSION}/nvidia-container-toolkit-${NVIDIA_VERSION}-solo.tar.gz" && \
        tar -xzf /tmp/nvidia-toolkit.tar.gz -C /usr/local && \
        rm /tmp/nvidia-toolkit.tar.gz; \
    fi
```

### 1.3 dockerd 默认配置

预创建 `/etc/docker/daemon.json`，entrypoint 会根据模式覆盖：

```json
{
  "storage-driver": "overlay2",
  "data-root": "/app/data/docker",
  "bridge": "none",
  "iptables": false,
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "1" }
}
```

关键设计说明：
- `data-root: /app/data/docker` — 镜像/容器数据持久化在 `/app/data` 挂载卷中，重启不丢失
- `bridge: none` / `iptables: false` — 禁用网络功能，NAS 场景不需要复杂的容器网络（Lada/Video2X 只做文件处理）
- GPU 启用时：entrypoint 运行时自动添加 nvidia runtime 配置并重启 dockerd

### 1.4 Dockerfile 完整 diff

```dockerfile
# 阶段 3 运行时 (现状)
# 安装的包: ffmpeg, openssh-client, ca-certificates, tzdata, nginx, docker-cli
# 
# 改为:
RUN apk add --no-cache \
    ffmpeg \
    openssh-client \
    ca-certificates \
    tzdata \
    nginx \
    docker \
    docker-engine \
    iptables \
    ip6tables \
    bridge \
    curl

# 新增: NVIDIA Container Toolkit（条件安装）
ARG INSTALL_NVIDIA=false
RUN if [ "$INSTALL_NVIDIA" = "true" ]; then \
        wget -qO /tmp/nvidia-toolkit.tar.gz \
            "https://github.com/NVIDIA/nvidia-container-toolkit/releases/download/v1.17.4/nvidia-container-toolkit-v1.17.4-solo.tar.gz" && \
        tar -xzf /tmp/nvidia-toolkit.tar.gz -C /usr/local && \
        rm /tmp/nvidia-toolkit.tar.gz; \
    fi

# 新增: dockerd 配置文件
COPY docker/daemon.json /etc/docker/daemon.json
```

## Step 2：entrypoint.sh 改造 — 启动内嵌 dockerd

### 2.1 启动流程

```bash
#!/bin/sh
set -e

# ============================================
# Docker 模式检测与 dockerd 启动
# ============================================
DOCKER_MODE=${DOCKER_MODE:-auto}  # 可选: host / dind / auto

# 判断是否需要启动内嵌 dockerd
START_DOCKERD=false
if [ "$DOCKER_MODE" = "dind" ]; then
    START_DOCKERD=true
elif [ "$DOCKER_MODE" = "auto" ] && [ ! -S "/var/run/docker.sock" ]; then
    START_DOCKERD=true
    export DOCKER_MODE=dind
fi

if [ "$START_DOCKERD" = "true" ]; then
    echo ">>> 启动内嵌 dockerd (Docker-in-Docker 模式)..."

    # 准备 dockerd 数据目录
    mkdir -p /app/data/docker

    # 检查 /sys/fs/cgroup 是否挂载（--privileged 或 cgroup 挂载必需）
    if [ ! -d "/sys/fs/cgroup" ]; then
        echo "警告: /sys/fs/cgroup 不存在，尝试挂载..."
        mkdir -p /sys/fs/cgroup
        mount -t tmpfs -o uid=0,gid=0,mode=0755 cgroup /sys/fs/cgroup 2>/dev/null || true
    fi

    # GPU 检测：如果 nvidia-smi 可用，自动配置 nvidia runtime
    if nvidia-smi >/dev/null 2>&1; then
        echo ">>> 检测到 NVIDIA GPU，配置 GPU 支持..."
        if command -v nvidia-ctk >/dev/null 2>&1; then
            nvidia-ctk runtime configure --runtime=docker
        fi
    fi

    # 启动 dockerd
    dockerd \
        --data-root /app/data/docker \
        --storage-driver overlay2 \
        --bridge none \
        --iptables=false \
        --log-driver json-file \
        --log-opt max-size=10m \
        --log-opt max-file=1 \
        --pidfile /var/run/docker_inner.pid \
        &
    DOCKERD_PID=$!

    # 等待 dockerd 就绪（最多 30 秒）
    echo ">>> 等待 dockerd 就绪..."
    for i in $(seq 1 30); do
        if docker info >/dev/null 2>&1; then
            echo ">>> dockerd 就绪 (PID: $DOCKERD_PID)"
            break
        fi
        sleep 1
    done

    if ! docker info >/dev/null 2>&1; then
        echo "!!! dockerd 启动失败，请检查日志"
        # 不阻止 nginx 和前端启动，但后端 Docker 功能不可用
    fi

    # GPU 配置后重启 dockerd 使其加载 nvidia runtime
    if [ -n "$GPU_CONFIGURED" ]; then
        kill -HUP $DOCKERD_PID 2>/dev/null
        sleep 2
    fi
fi

# ============================================
# 启动后端服务（原有逻辑）
# ============================================
echo ">>> 启动后端服务..."
# ... 现有后端启动代码 ...

# ============================================
# 等待后端就绪 + 启动 nginx（原有逻辑）
# ============================================
```

### 2.2 关闭流程

在 `cleanup()` 函数中增加 dockerd 的优雅关闭：

```bash
cleanup() {
    echo ">>> 收到关闭信号，开始优雅关闭..."

    # 停止后端
    kill -TERM "$BACKEND_PID" 2>/dev/null
    wait "$BACKEND_PID" 2>/dev/null

    # 停止 nginx
    kill -QUIT "$NGINX_PID" 2>/dev/null
    wait "$NGINX_PID" 2>/dev/null

    # DinD 模式: 先停止所有子容器，再关闭 dockerd
    if [ -n "$DOCKERD_PID" ] && kill -0 "$DOCKERD_PID" 2>/dev/null; then
        echo ">>> 停止所有运行中的子容器..."
        docker stop $(docker ps -q) 2>/dev/null || true

        echo ">>> 停止内嵌 dockerd..."
        kill -TERM "$DOCKERD_PID" 2>/dev/null
        # 等待 dockerd 退出（最多 10 秒）
        for i in $(seq 1 10); do
            if ! kill -0 "$DOCKERD_PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        # 强制杀掉
        kill -KILL "$DOCKERD_PID" 2>/dev/null || true
    fi
}
```

## Step 3：应用代码修改

### 3.1 新增：`backend/internal/utils/docker_dind.go`

```go
package utils

import "os"

// IsDindMode 检测是否运行在 Docker-in-Docker 模式。
// 优先检查 DOCKER_MODE 环境变量，回退到自动检测。
func IsDindMode() bool {
    // 显式指定
    switch os.Getenv("DOCKER_MODE") {
    case "dind":
        return true
    case "host":
        return false
    }
    // auto 模式：容器内且 socket 不存在 -> 假定 DinD
    if IsRunningInContainer() {
        if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
            return true
        }
    }
    return false
}
```

### 3.2 修改 `repair/repair.go` 的 `resolveHostPath()`

在 `resolveHostPath()` 函数开头加入 DinD 短路：

```go
func resolveHostPath(containerPath string) string {
    // DinD 模式：内嵌 dockerd 与当前进程共享文件系统命名空间，
    // 容器内路径即为内嵌 dockerd 可直接使用的路径，无需转换。
    if utils.IsDindMode() {
        return containerPath
    }
    // 原有逻辑（用于宿主机 Docker socket 模式）
    if hostPath, ok := resolveHostPathViaDocker(containerPath); ok {
        return hostPath
    }
    return resolveHostPathViaMountinfo(containerPath)
}
```

### 3.3 修改 `upscale/upscale.go` 的 `resolveHostPath()`

完全相同的改动。

### 3.4 修改修复/清晰度执行器的 `Reload()` 方法

在 `repair/repair.go` 的 `Reload()` 中，在 `exec.LookPath("docker")` 检查后增加 DinD 模式下的额外检查：

```go
func (e *Executor) Reload(cfg Config) error {
    if cfg.DockerImage == "" {
        return errors.New("去马赛克 Docker 镜像不能为空")
    }
    if _, err := exec.LookPath("docker"); err != nil {
        return fmt.Errorf("未找到 docker 命令，请确认 Docker 已安装并启动: %w", err)
    }
    // DinD 模式下额外检查 dockerd 是否正在运行
    if utils.IsDindMode() {
        cmd := exec.Command("docker", "info")
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("内嵌 dockerd 未正常运行: %w", err)
        }
    }
    e.mu.Lock()
    e.config = cfg
    e.mu.Unlock()
    return nil
}
```

`upscale/upscale.go` 做同样的改动。

## Step 4：组件检测适配 DinD

### 修改 `component/detector.go` 的 `detectDocker()`

```go
func (d *Detector) detectDocker(ctx context.Context) ComponentInfo {
    info := ComponentInfo{
        Type:        ComponentDocker,
        Name:        "Docker",
        Description: "Container runtime, required by all Docker components",
        NeedsDocker: false,
    }

    // Check docker --version
    version, err := runCommand(ctx, "docker", "--version")
    if err != nil {
        info.Status = StatusMissing
        inContainer := utils.IsRunningInContainer()
        if inContainer {
            if utils.IsDindMode() {
                info.ErrorMsg = "DinD 模式下未找到 docker CLI，请使用包含完整 Docker CE 的镜像"
            } else {
                info.ErrorMsg = "当前运行在 Docker 容器内,但未找到 docker CLI。" +
                    "请使用最新镜像（已内置 docker-cli），" +
                    "或在 docker run 时挂载宿主机 docker socket：-v /var/run/docker.sock:/var/run/docker.sock"
            }
        }
        return info
    }
    info.Version = parseDockerVersion(version)

    // Check docker info
    _, err = runCommand(ctx, "docker", "info")
    if err != nil {
        info.Status = StatusError
        if utils.IsDindMode() {
            info.ErrorMsg = "内嵌 dockerd 未正常运行。请检查日志，或确认 volume /app/data 有足够空间存储 Docker 镜像"
        } else {
            inContainer := utils.IsRunningInContainer()
            if inContainer {
                info.ErrorMsg = "Docker daemon 不可达。请确保 docker run 时挂载了宿主机 Docker 套接字：" +
                    "-v /var/run/docker.sock:/var/run/docker.sock"
            } else {
                info.ErrorMsg = "Docker daemon is not running"
            }
        }
        return info
    }

    info.Status = StatusInstalled
    return info
}
```

## Step 5：docker-compose 配置

### 新增 `docker-compose.dind.yml`

```yaml
services:
  videoflow:
    image: ghcr.io/studynoweekend/videoflow:latest
    container_name: videoflow
    restart: unless-stopped
    privileged: true
    environment:
      - DOCKER_MODE=dind
      - TZ=Asia/Shanghai
    volumes:
      - ./config.yaml:/app/config/config.yaml:ro
      - ./data:/app/data                # SQLite + Docker 镜像数据持久化
      - /path/to/your/videos:/videos:ro
      - /path/to/your/output:/output
      - ./logs:/app/logs
    ports:
      - "8080:80"

# 说明：
# - privileged: true 是 DinD 模式必需的（dockerd 需要创建 cgroup 和挂载文件系统）
# - /app/data 目录会存储 SQLite 数据库 + Docker 镜像/容器数据，建议至少分配 5GB 空间
# - GPU 支持：需要额外挂载 NVIDIA 设备（--gpus all）并确认镜像带 nvidia-container-toolkit
#   Docker Compose 支持 devices 字段或 deploy.resources.reservations.devices
```

### 修改主 `docker-compose.yml`

添加注释，标明 DinD 模式可选的替代方案。

## Step 6：配置项扩展

### 6.1 `bootstrap/config.go` 新增

```go
type AppConfig struct {
    App         AppConfigApp         `mapstructure:"app"`
    HTTP        AppConfigHTTP        `mapstructure:"http"`
    Log         AppConfigLog         `mapstructure:"log"`
    Database    AppConfigDatabase    `mapstructure:"database"`
    Video       AppConfigVideo       `mapstructure:"video"`
    Output      AppConfigOutput      `mapstructure:"output"`
    Scan        AppConfigScan        `mapstructure:"scan"`
    ASR         AppConfigASR         `mapstructure:"asr"`
    Repair      AppConfigRepair      `mapstructure:"repair"`
    Scheduler   AppConfigScheduler   `mapstructure:"scheduler"`
    Concurrency AppConfigConcurrency `mapstructure:"concurrency"`
    FFmpeg      AppConfigFFmpeg      `mapstructure:"ffmpeg"`
    Upscale     AppConfigUpscale     `mapstructure:"upscale"`
    Docker      AppConfigDocker      `mapstructure:"docker"`     // 新增
}

// AppConfigDocker Docker 运行模式配置
type AppConfigDocker struct {
    Mode string `mapstructure:"mode"` // "host" / "dind" / "auto"，默认 "auto"
}
```

### 6.2 `config.yaml` 新增

```yaml
docker:
  mode: auto  # 运行模式: host（宿主机 socket）/ dind（内嵌 dockerd）/ auto（自动检测）
```

### 6.3 环境变量覆盖

`APP_DOCKER_MODE=dind` 可覆盖配置文件。

## Step 7：测试

### 7.1 单元测试

- `utils/IsDindMode()` — 环境变量控制测试（`DOCKER_MODE=dind` → true, `DOCKER_MODE=host` → false, 无 socket → true）
- `resolveHostPath()` — DinD 模式下返回原路径
- `detectDocker()` — DinD 模式下错误信息检查

### 7.2 DinD 集成测试

- 构建带 dockerd 的测试镜像（`--build-arg INSTALL_NVIDIA=false`）
- 启动容器（`docker run --privileged`），验证内嵌 dockerd 能正确启动
- 验证 `docker info`、`docker pull`、`docker run --rm` 正常工作
- 验证 Lada/Video2X 容器能通过内嵌 dockerd 启动并处理视频

### 7.3 回归测试

- 现有宿主机 socket 模式完全不受影响
- `repair_test.go` 的 `TestBuildRunArgsCPU/TestBuildRunArgsCUDA` 仍然通过
- `upscale_test.go` 的对应测试

## Step 8：文档更新

### README 新增 DinD 部署说明

```markdown
### 方式二：Docker-in-Docker 部署（适用于 NAS 等无法挂载 Docker socket 的环境）

如果你的 NAS 或服务器不支持挂载宿主机 Docker socket，可以使用 DinD 模式：

```bash
docker run -d \
  --name videoflow \
  --privileged \
  -e DOCKER_MODE=dind \
  -p 8080:80 \
  -v ./config.yaml:/app/config/config.yaml:ro \
  -v ./data:/app/data \
  -v /path/to/videos:/videos:ro \
  -v /path/to/output:/output \
  ghcr.io/studynoweekend/videoflow:latest
```

或使用 docker-compose：
```bash
docker compose -f docker-compose.dind.yml up -d
```

**注意事项：**
- DinD 模式需要 `--privileged` 权限，因为内嵌 dockerd 需要创建 cgroup 和挂载文件系统
- 镜像数据存储在 `/app/data/docker`，请确保 volume 有至少 5GB 可用空间
- 第一次启动需要拉取 Lada 和 Video2X 镜像，会多花几分钟
- GPU 支持：如需 GPU 加速，额外传 `--gpus all` 参数，且需构建时启用 `INSTALL_NVIDIA=true`
```

## 风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| `--privileged` 安全性 | 容器权限提升 | 将 `--cap-add SYS_ADMIN` 作为最低要求方案写入文档 |
| 镜像体积增大 150-200MB | 拉取/构建更慢 | 多阶段构建优化，仅运行时镜像膨胀 |
| 启动时间增加 3-8 秒 | 容器启动变慢 | entrypoint 中异步等待，服务降级可用 |
| cgroup v2 兼容性 | dockerd 启动失败 | Alpine 3.20 支持 cgroup v2，测试验证 |
| Alpine 上 nvidia-toolkit 非官方支持 | GPU 功能受限 | 条件构建 + 静态二进制，文档说明局限性 |
| `/app/data` 空间不足 | dockerd 启动失败 | 文档说明最低空间要求，entrypoint 增加空间检查 |
| 内容感知存储（Content Store） | dockerd 启动失败 | 文档说明外容器需要 `--privileged` 或 `--storage-opt` |

## 预估工作量

| 阶段 | 天数 |
|------|------|
| Step 1: Dockerfile 改造 | 2-3 天 |
| Step 2: entrypoint 改造 | 2 天 |
| Step 3: 应用代码修改 | 1 天 |
| Step 4: 组件检测 | 0.5 天 |
| Step 5: docker-compose | 0.5 天 |
| Step 6: 配置项 | 0.5 天 |
| Step 7: 测试 | 2 天 |
| Step 8: 文档 | 1 天 |
| **合计** | **约 9-10 天** |
