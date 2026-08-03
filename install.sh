#!/usr/bin/env bash
# ============================================================
# Video Captions 安装脚本 — 命令行模式
#
# 用法:
#   ./install.sh [command] [options]
#
# 命令:
#   list                  - 列出所有组件状态
#   install-asr           - 安装 Whisper ASR Webservice
#   install-lada          - 安装 Lada 视频修复工具
#   install-ffmpeg        - 安装 FFmpeg
#   install-all           - 安装所有组件
#   uninstall [component] - 卸载组件 (asr|lada|ffmpeg)
#   help                  - 显示帮助信息
# ============================================================

set -euo pipefail

# ------------------- 颜色定义 -------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ------------------- 全局变量 -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

WHISPER_CONTAINER_NAME="whisper-asr-webservice"
WHISPER_PORT=9000
LADA_CONTAINER_NAME="lada-app"
LADA_PORT=8080

# 语言检测: 如果 LANG 包含 zh_CN 则使用中文
if [[ "${LANG:-}" =~ zh_CN|zh_Hans|zh_CN\.* ]]; then
    LANG_ZH=1
else
    LANG_ZH=0
fi

# ------------------- 多语言消息函数 -------------------

msg() {
    local en="$1"
    local zh="$2"
    if [[ "$LANG_ZH" -eq 1 ]]; then
        echo -e "$zh"
    else
        echo -e "$en"
    fi
}

# ------------------- 工具函数 -------------------

print_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[OK]${NC} $*"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
print_step()    { echo -e "\n${CYAN}${BOLD}══════════ $* ══════════${NC}"; }
print_title()   { echo -e "${BOLD}$*${NC}"; }

# 确认操作，默认返回 Yes
confirm() {
    local msg="$1" default="${2:-y}"
    if [[ "$default" == "y" ]]; then
        read -rp "$(echo -e "${YELLOW}$msg [Y/n]${NC} ")" ans
        [[ -z "$ans" || "$ans" =~ ^[Yy]$ ]]
    else
        read -rp "$(echo -e "${YELLOW}$msg [y/N]${NC} ")" ans
        [[ "$ans" =~ ^[Yy]$ ]]
    fi
}

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Darwin*)                  echo "macos"   ;;
        Linux*)                   echo "linux"   ;;
        MINGW*|MSYS*|CYGWIN*)     echo "windows" ;;
        *)                        echo "unknown" ;;
    esac
}

# 检测 Docker 是否可用
check_docker() {
    if ! command -v docker &>/dev/null; then
        print_error "$(msg \
            "Docker is not installed. Please install Docker first: https://docs.docker.com/engine/install/" \
            "未检测到 Docker，请先安装 Docker。安装指引: https://docs.docker.com/engine/install/")"
        return 1
    fi
    if ! docker info &>/dev/null; then
        print_error "$(msg \
            "Docker daemon is not running. Please start Docker and try again." \
            "Docker 守护进程未运行，请启动 Docker 后重试。")"
        return 1
    fi
    return 0
}

# 简易 spinner 进度指示
spinner() {
    local pid=$1
    local msg="$2"
    local spin='-\|/'
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        i=$(( (i+1) % 4 ))
        printf "\r${CYAN}[%c]${NC} %s" "${spin:$i:1}" "$msg"
        sleep 0.2
    done
    printf "\r${GREEN}[✓]${NC} %s\n" "$msg"
}

# ------------------- 组件检测函数 -------------------

# 检测 Whisper ASR 容器状态
detect_whisper() {
    if ! command -v docker &>/dev/null; then
        echo "docker_missing"
        return
    fi
    if docker ps --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        local ver
        ver=$(docker inspect "$WHISPER_CONTAINER_NAME" --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
        echo "running:${ver}"
    elif docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        echo "stopped"
    else
        echo "missing"
    fi
}

# 检测 Lada 容器状态
detect_lada() {
    if ! command -v docker &>/dev/null; then
        echo "docker_missing"
        return
    fi
    if docker ps --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
        local ver
        ver=$(docker inspect "$LADA_CONTAINER_NAME" --format '{{.Config.Image}}' 2>/dev/null || echo "unknown")
        echo "running:${ver}"
    elif docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
        echo "stopped"
    else
        echo "missing"
    fi
}

# 检测 FFmpeg 状态
detect_ffmpeg() {
    if command -v ffmpeg &>/dev/null; then
        local ver
        ver=$(ffmpeg -version 2>&1 | head -n1 | sed 's/ffmpeg version //' | sed 's/ Copyright.*//')
        echo "installed:${ver}"
    else
        echo "missing"
    fi
}

# 格式化状态输出
format_status() {
    local status="$1"
    case "$status" in
        docker_missing)
            echo -e "${YELLOW}$(msg "Docker not found" "Docker 未安装")${NC}"
            ;;
        missing)
            echo -e "${RED}$(msg "Not installed" "未安装")${NC}"
            ;;
        stopped)
            echo -e "${YELLOW}$(msg "Stopped" "已停止")${NC}"
            ;;
        running:*)
            local ver="${status#running:}"
            echo -e "${GREEN}$(msg "Running" "运行中")${NC} (${ver})"
            ;;
        installed:*)
            local ver="${status#installed:}"
            echo -e "${GREEN}$(msg "Installed" "已安装")${NC} (${ver})"
            ;;
        *)
            echo "$status"
            ;;
    esac
}

# ------------------- 命令: list -------------------

cmd_list() {
    print_step "$(msg "Component Status" "组件状态检测")"

    printf "${BOLD}%-20s %s${NC}\n" "$(msg "Component" "组件")" "$(msg "Status" "状态")"
    printf "%-20s %s\n" "────────────────────" "──────────────────────────"

    local whisper_status
    whisper_status=$(detect_whisper)
    printf "%-20s %s\n" "Whisper ASR" "$(format_status "$whisper_status")"

    local lada_status
    lada_status=$(detect_lada)
    printf "%-20s %s\n" "Lada" "$(format_status "$lada_status")"

    local ffmpeg_status
    ffmpeg_status=$(detect_ffmpeg)
    printf "%-20s %s\n" "FFmpeg" "$(format_status "$ffmpeg_status")"

    echo ""
    msg \
        "Run './install.sh help' for usage information." \
        "运行 './install.sh help' 查看使用帮助。"
}

# ------------------- 命令: install-asr -------------------

cmd_install_asr() {
    local asr_engine="openai_whisper"
    local asr_model="base"
    local asr_device="cpu"
    local gpu_flag=""
    local hf_token=""
    local cache_dir=""

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --engine)
                shift
                asr_engine="$1"
                ;;
            --model)
                shift
                asr_model="$1"
                ;;
            --gpu)
                asr_device="cuda"
                gpu_flag="--gpus all"
                ;;
            --hf-token)
                shift
                hf_token="$1"
                ;;
            --cache-dir)
                shift
                cache_dir="$1"
                ;;
            *)
                print_error "$(msg \
                    "Unknown option: $1. Use --engine, --model, --gpu, --hf-token, --cache-dir" \
                    "未知选项: $1。支持: --engine, --model, --gpu, --hf-token, --cache-dir")"
                return 1
                ;;
        esac
        shift
    done

    print_step "$(msg "Installing Whisper ASR Webservice" "安装 Whisper ASR Webservice")"

    check_docker || return 1

    local os
    os="$(detect_os)"

    # 参数校验
    local valid_engines=("openai_whisper" "faster_whisper" "whisperx")
    local valid=0
    for e in "${valid_engines[@]}"; do
        [[ "$asr_engine" == "$e" ]] && valid=1
    done
    if [[ "$valid" -eq 0 ]]; then
        print_error "$(msg \
            "Invalid engine: $asr_engine. Supported: openai_whisper, faster_whisper, whisperx" \
            "无效的引擎: $asr_engine。支持: openai_whisper, faster_whisper, whisperx")"
        return 1
    fi

    # GPU 检测
    if [[ "$asr_device" == "cuda" ]] && [[ "$os" != "linux" ]]; then
        print_warn "$(msg \
            "GPU mode is only fully supported on Linux. Will attempt GPU mode anyway." \
            "GPU 模式仅在 Linux 上完全支持，将尝试继续。")"
    fi

    # 确定镜像 tag
    local image_tag="latest"
    if [[ "$asr_device" == "cuda" ]]; then
        image_tag="latest-gpu"
    fi

    # --- 如果容器已存在则先移除 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        print_warn "$(msg \
            "Removing existing container ${WHISPER_CONTAINER_NAME}..." \
            "正在移除旧容器 ${WHISPER_CONTAINER_NAME}...")"
        docker rm -f "$WHISPER_CONTAINER_NAME" &>/dev/null
    fi

    # --- 拉取镜像 ---
    local full_image="onerahmet/openai-whisper-asr-webservice:${image_tag}"
    print_info "$(msg \
        "Pulling image: ${full_image} ..." \
        "正在拉取镜像: ${full_image} ...")"
    docker pull "$full_image"

    # --- 构建启动参数 ---
    local run_args=(-d --name "$WHISPER_CONTAINER_NAME")
    [[ -n "$gpu_flag" ]] && run_args+=($gpu_flag)
    run_args+=(-p "${WHISPER_PORT}:9000")
    run_args+=(-e "ASR_MODEL=${asr_model}")
    run_args+=(-e "ASR_ENGINE=${asr_engine}")
    run_args+=(-e "ASR_DEVICE=${asr_device}")

    if [[ -n "$hf_token" ]]; then
        run_args+=(-e "HF_TOKEN=${hf_token}")
    fi

    # 持久化缓存
    if [[ -n "$cache_dir" ]]; then
        mkdir -p "$cache_dir"
        print_info "$(msg "Cache directory: ${cache_dir}" "缓存目录: ${cache_dir}")"
        run_args+=(-v "${cache_dir}:/root/.cache")
    fi

    run_args+=("$full_image")

    # --- 启动容器 ---
    print_info "$(msg \
        "Starting Whisper ASR container..." \
        "正在启动 Whisper ASR 容器...")"
    docker run "${run_args[@]}"

    # 等待服务就绪
    print_info "$(msg \
        "Waiting for Whisper ASR service (model download may take a while)..." \
        "等待 Whisper ASR 服务就绪（模型首次下载可能需要较长时间）...")"
    local max_wait=300
    local waited=0
    while ! curl -sf "http://localhost:${WHISPER_PORT}/docs" &>/dev/null; do
        sleep 5
        waited=$((waited + 5))
        if [[ $waited -ge $max_wait ]]; then
            print_warn "$(msg \
                "Service did not become ready within ${max_wait}s. Check logs: docker logs ${WHISPER_CONTAINER_NAME}" \
                "服务在 ${max_wait}s 内未完全就绪，查看日志: docker logs ${WHISPER_CONTAINER_NAME}")"
            break
        fi
        echo -n "."
    done
    echo ""
    print_success "$(msg \
        "Whisper ASR Webservice started: http://localhost:${WHISPER_PORT}" \
        "Whisper ASR Webservice 已启动: http://localhost:${WHISPER_PORT}")"
    print_info "$(msg \
        "Swagger docs: http://localhost:${WHISPER_PORT}/docs" \
        "Swagger 文档: http://localhost:${WHISPER_PORT}/docs")"
}

# ------------------- 命令: install-lada -------------------

cmd_install_lada() {
    local device="cpu"
    local start_container=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --device)
                shift
                device="$1"
                ;;
            --start)
                start_container="yes"
                ;;
            *)
                print_error "$(msg \
                    "Unknown option: $1. Use --device [cpu|cuda|mps|xpu] or --start" \
                    "未知选项: $1。支持: --device [cpu|cuda|mps|xpu] 或 --start")"
                return 1
                ;;
        esac
        shift
    done

    print_step "$(msg "Installing Lada Video Repair Tool" "安装 Lada 视频修复工具")"

    check_docker || return 1

    local valid_devices=("cpu" "cuda" "mps" "xpu")
    local valid=0
    for d in "${valid_devices[@]}"; do
        [[ "$device" == "$d" ]] && valid=1
    done
    if [[ "$valid" -eq 0 ]]; then
        print_error "$(msg \
            "Invalid device: $device. Supported: cpu, cuda, mps, xpu" \
            "无效的设备: $device。支持: cpu, cuda, mps, xpu")"
        return 1
    fi

    local lada_image="ladaapp/lada:latest"
    print_info "$(msg \
        "Pulling image: ${lada_image} ..." \
        "正在拉取镜像: ${lada_image} ...")"
    docker pull "$lada_image"
    print_success "$(msg \
        "Image ${lada_image} pulled successfully." \
        "镜像 ${lada_image} 拉取完成。")"

    if [[ "$start_container" == "yes" ]]; then
        if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
            print_warn "$(msg \
                "Removing existing container ${LADA_CONTAINER_NAME}..." \
                "正在移除旧容器 ${LADA_CONTAINER_NAME}...")"
            docker rm -f "$LADA_CONTAINER_NAME" &>/dev/null
        fi

        local lada_run_args=(-d --name "$LADA_CONTAINER_NAME")
        if [[ "$device" != "cpu" ]]; then
            lada_run_args+=(--device "/dev/${device}")
        fi
        lada_run_args+=(-p "${LADA_PORT}:8080" "$lada_image")

        docker run "${lada_run_args[@]}"
        print_success "$(msg \
            "Lada container started: http://localhost:${LADA_PORT}" \
            "Lada 容器已启动: http://localhost:${LADA_PORT}")"
    else
        print_info "$(msg \
            "Skipped container startup. Run with --start to start it." \
            "已跳过启动容器。使用 --start 参数启动。")"
    fi
}

# ------------------- 命令: install-ffmpeg -------------------

cmd_install_ffmpeg() {
    local reinstall=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --reinstall)
                reinstall="yes"
                ;;
            *)
                print_error "$(msg \
                    "Unknown option: $1. Use --reinstall" \
                    "未知选项: $1。支持: --reinstall")"
                return 1
                ;;
        esac
        shift
    done

    print_step "$(msg "Installing FFmpeg" "安装 FFmpeg")"

    # 检测是否已安装
    if command -v ffmpeg &>/dev/null; then
        local current_ver
        current_ver="$(ffmpeg -version 2>&1 | head -n1)"
        print_success "$(msg \
            "FFmpeg already installed: ${current_ver}" \
            "FFmpeg 已安装: ${current_ver}")"
        if [[ -z "$reinstall" ]]; then
            if ! confirm "$(msg \
                "Reinstall/upgrade FFmpeg?" \
                "是否重新安装/升级 FFmpeg？")" "n"; then
                print_info "$(msg \
                    "Skipping FFmpeg installation." \
                    "跳过 FFmpeg 安装。")"
                return 0
            fi
        fi
    fi

    local os
    os="$(detect_os)"
    print_info "$(msg "Detected OS: ${os}" "检测到操作系统: ${os}")"

    case "$os" in
        macos)
            if command -v brew &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via Homebrew ..." \
                    "使用 Homebrew 安装 FFmpeg ...")"
                brew install ffmpeg
            else
                print_warn "$(msg \
                    "Homebrew not found. Attempting to install Homebrew first..." \
                    "未检测到 Homebrew，正在尝试自动安装 Homebrew ...")"
                if confirm "$(msg \
                    "Install Homebrew?" \
                    "是否安装 Homebrew？")" "y"; then
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                    brew install ffmpeg
                else
                    print_error "$(msg \
                        "Please install FFmpeg manually: https://ffmpeg.org/download.html" \
                        "请手动安装 FFmpeg: https://ffmpeg.org/download.html")"
                    return 1
                fi
            fi
            ;;

        linux)
            if command -v apt-get &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via apt-get ..." \
                    "使用 apt-get 安装 FFmpeg ...")"
                sudo apt-get update && sudo apt-get install -y ffmpeg
            elif command -v dnf &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via dnf ..." \
                    "使用 dnf 安装 FFmpeg ...")"
                sudo dnf install -y ffmpeg
            elif command -v yum &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via yum ..." \
                    "使用 yum 安装 FFmpeg ...")"
                sudo yum install -y ffmpeg
            elif command -v pacman &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via pacman ..." \
                    "使用 pacman 安装 FFmpeg ...")"
                sudo pacman -S --noconfirm ffmpeg
            elif command -v apk &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via apk ..." \
                    "使用 apk 安装 FFmpeg ...")"
                sudo apk add --no-cache ffmpeg
            else
                print_error "$(msg \
                    "No supported package manager found. Please install FFmpeg manually: https://ffmpeg.org/download.html" \
                    "未识别到支持的包管理器，请手动安装 FFmpeg: https://ffmpeg.org/download.html")"
                return 1
            fi
            ;;

        windows)
            if command -v winget &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via winget ..." \
                    "使用 winget 安装 FFmpeg ...")"
                winget install --id Gyan.FFmpeg -e --accept-package-agreements --accept-source-agreements
            elif command -v choco &>/dev/null; then
                print_info "$(msg \
                    "Installing FFmpeg via Chocolatey ..." \
                    "使用 Chocolatey 安装 FFmpeg ...")"
                choco install ffmpeg -y
            else
                print_warn "$(msg \
                    "No winget or Chocolatey detected." \
                    "未检测到 winget 或 Chocolatey。")"
                print_info "$(msg \
                    "Please install FFmpeg via one of:" \
                    "请通过以下方式之一安装 FFmpeg：")"
                echo "  1. winget: https://learn.microsoft.com/windows/package-manager/winget/"
                echo "  2. Chocolatey: https://chocolatey.org/install"
                echo "  3. Manual: https://www.gyan.dev/ffmpeg/builds/"
                echo ""
                read -rp "$(echo -e "${YELLOW}$(msg \"Press Enter after installing FFmpeg...\" \"FFmpeg 安装完成后按回车继续...\")${NC}")"
            fi
            ;;

        *)
            print_error "$(msg \
                "Unsupported OS. Please install FFmpeg manually: https://ffmpeg.org/download.html" \
                "不支持的操作系统，请手动安装 FFmpeg: https://ffmpeg.org/download.html")"
            return 1
            ;;
    esac

    # 验证安装
    if command -v ffmpeg &>/dev/null; then
        print_success "$(msg \
            "FFmpeg installed successfully: $(ffmpeg -version 2>&1 | head -n1)" \
            "FFmpeg 安装成功: $(ffmpeg -version 2>&1 | head -n1)")"
    else
        print_error "$(msg \
            "FFmpeg not found in PATH after installation. Please check environment variables." \
            "FFmpeg 安装后仍未在 PATH 中检测到，请检查环境变量。")"
        return 1
    fi
}

# ------------------- 命令: uninstall -------------------

cmd_uninstall() {
    local component="${1:-}"

    if [[ -z "$component" ]]; then
        print_error "$(msg \
            "Please specify a component to uninstall: asr, lada, or ffmpeg" \
            "请指定要卸载的组件: asr, lada, ffmpeg")"
        return 1
    fi

    case "$component" in
        asr)
            print_step "$(msg "Uninstalling Whisper ASR" "卸载 Whisper ASR")"
            if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
                print_info "$(msg "Stopping container..." "正在停止容器...")"
                docker stop "$WHISPER_CONTAINER_NAME" &>/dev/null || true
                print_info "$(msg "Removing container..." "正在删除容器...")"
                docker rm "$WHISPER_CONTAINER_NAME" &>/dev/null || true
            else
                print_info "$(msg "No container found." "未找到容器。")"
            fi
            # 检查并删除镜像
            if docker image inspect onerahmet/openai-whisper-asr-webservice &>/dev/null; then
                print_info "$(msg "Removing image..." "正在删除镜像...")"
                docker rmi onerahmet/openai-whisper-asr-webservice &>/dev/null || true
            fi
            print_success "$(msg "Whisper ASR uninstalled." "Whisper ASR 已卸载。")"
            ;;

        lada)
            print_step "$(msg "Uninstalling Lada" "卸载 Lada")"
            if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
                print_info "$(msg "Stopping container..." "正在停止容器...")"
                docker stop "$LADA_CONTAINER_NAME" &>/dev/null || true
                print_info "$(msg "Removing container..." "正在删除容器...")"
                docker rm "$LADA_CONTAINER_NAME" &>/dev/null || true
            else
                print_info "$(msg "No container found." "未找到容器。")"
            fi
            if docker image inspect ladaapp/lada &>/dev/null; then
                print_info "$(msg "Removing image..." "正在删除镜像...")"
                docker rmi ladaapp/lada &>/dev/null || true
            fi
            print_success "$(msg "Lada uninstalled." "Lada 已卸载。")"
            ;;

        ffmpeg)
            print_step "$(msg "Uninstalling FFmpeg" "卸载 FFmpeg")"
            local os
            os="$(detect_os)"
            case "$os" in
                macos)
                    echo -e "  ${YELLOW}brew uninstall ffmpeg${NC}"
                    ;;
                linux)
                    echo -e "  ${YELLOW}sudo apt-get remove ffmpeg   # Debian/Ubuntu${NC}"
                    echo -e "  ${YELLOW}sudo dnf remove ffmpeg       # Fedora${NC}"
                    echo -e "  ${YELLOW}sudo yum remove ffmpeg       # CentOS/RHEL${NC}"
                    echo -e "  ${YELLOW}sudo pacman -R ffmpeg        # Arch Linux${NC}"
                    ;;
                windows)
                    echo -e "  ${YELLOW}winget uninstall Gyan.FFmpeg${NC}"
                    echo -e "  ${YELLOW}choco uninstall ffmpeg${NC}"
                    ;;
            esac
            print_info "$(msg \
                "FFmpeg was installed via system package manager. Please use the command above to uninstall." \
                "FFmpeg 是通过系统包管理器安装的，请使用以上命令卸载。")"
            ;;

        *)
            print_error "$(msg \
                "Unknown component: $component. Use: asr, lada, or ffmpeg" \
                "未知组件: $component。请使用: asr, lada, ffmpeg")"
            return 1
            ;;
    esac
}

# ------------------- 命令: install-all -------------------

cmd_install_all() {
    print_step "$(msg "Installing All Components" "安装所有组件")"

    check_docker || return 1

    cmd_install_asr
    echo ""
    cmd_install_lada --start
    echo ""
    cmd_install_ffmpeg
    echo ""

    print_success "$(msg \
        "All components installed successfully!" \
        "所有组件安装完成！")"
}

# ------------------- 命令: help -------------------

cmd_help() {
    echo -e "${GREEN}${BOLD}"
    echo "╔═══════════════════════════════════════════════╗"
    msg \
        "║     Video Captions Installation Script         ║" \
        "║     Video Captions 安装脚本                    ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"

    msg \
        "Usage: ./install.sh [command] [options]" \
        "用法: ./install.sh [命令] [选项]"

    echo ""
    echo -e "${BOLD}$(msg "Commands:" "命令列表：")${NC}"
    echo "  list                    $(msg "- List all component statuses" "- 列出所有组件状态")"
    echo "  install-asr             $(msg "- Install Whisper ASR Webservice" "- 安装 Whisper ASR Webservice")"
    echo "  install-lada            $(msg "- Install Lada video repair tool" "- 安装 Lada 视频修复工具")"
    echo "  install-ffmpeg          $(msg "- Install FFmpeg" "- 安装 FFmpeg")"
    echo "  install-all             $(msg "- Install all components" "- 安装所有组件")"
    echo "  uninstall [component]   $(msg "- Uninstall a component" "- 卸载组件")"
    echo "  help                    $(msg "- Show this help message" "- 显示帮助信息")"

    echo ""
    echo -e "${BOLD}$(msg "install-asr options:" "install-asr 选项：")${NC}"
    echo "  --engine [engine]       $(msg "ASR engine (default: openai_whisper)" "ASR 引擎 (默认: openai_whisper)")"
    echo "                          $(msg "Supported: openai_whisper, faster_whisper, whisperx" "支持: openai_whisper, faster_whisper, whisperx")"
    echo "  --model [model]         $(msg "Model size (default: base)" "模型大小 (默认: base)")"
    echo "                          $(msg "Supported: tiny, base, small, medium, large-v3, large-v3-turbo" "支持: tiny, base, small, medium, large-v3, large-v3-turbo")"
    echo "  --gpu                   $(msg "Use GPU mode (default: CPU)" "使用 GPU 模式 (默认: CPU)")"
    echo "  --hf-token [token]      $(msg "HuggingFace token for whisperx" "WhisperX 的 HuggingFace Token")"
    echo "  --cache-dir [path]      $(msg "Persist model cache directory" "持久化模型缓存目录")"

    echo ""
    echo -e "${BOLD}$(msg "install-lada options:" "install-lada 选项：")${NC}"
    echo "  --device [device]       $(msg "Device to use (default: cpu)" "运行设备 (默认: cpu)")"
    echo "                          $(msg "Supported: cpu, cuda, mps, xpu" "支持: cpu, cuda, mps, xpu")"
    echo "  --start                 $(msg "Start container after pull" "拉取后启动容器")"

    echo ""
    echo -e "${BOLD}$(msg "install-ffmpeg options:" "install-ffmpeg 选项：")${NC}"
    echo "  --reinstall             $(msg "Reinstall even if already installed" "强制重新安装")"

    echo ""
    echo -e "${BOLD}$(msg "uninstall:" "uninstall 用法：")${NC}"
    echo "  ./install.sh uninstall asr          $(msg "- Uninstall Whisper ASR" "- 卸载 Whisper ASR")"
    echo "  ./install.sh uninstall lada         $(msg "- Uninstall Lada" "- 卸载 Lada")"
    echo "  ./install.sh uninstall ffmpeg       $(msg "- Uninstall FFmpeg" "- 卸载 FFmpeg")"

    echo ""
    echo -e "${BOLD}$(msg "Examples:" "示例：")${NC}"
    echo "  ./install.sh list"
    echo "  ./install.sh install-asr --engine faster_whisper --model medium --gpu"
    echo "  ./install.sh install-asr --engine whisperx --hf-token hf_xxxx --cache-dir ./cache"
    echo "  ./install.sh install-lada --device mps --start"
    echo "  ./install.sh install-ffmpeg --reinstall"
    echo "  ./install.sh uninstall lada"
}

# ============================================================
# 主入口
# ============================================================
main() {
    if [[ $# -eq 0 ]]; then
        cmd_help
        exit 0
    fi

    local command="$1"
    shift

    case "$command" in
        list)
            cmd_list
            ;;
        install-asr)
            cmd_install_asr "$@"
            ;;
        install-lada)
            cmd_install_lada "$@"
            ;;
        install-ffmpeg)
            cmd_install_ffmpeg "$@"
            ;;
        install-all)
            cmd_install_all
            ;;
        uninstall)
            cmd_uninstall "$@"
            ;;
        help|--help|-h)
            cmd_help
            ;;
        *)
            print_error "$(msg \
                "Unknown command: $command. Run './install.sh help' for usage." \
                "未知命令: $command。运行 './install.sh help' 查看帮助。")"
            exit 1
            ;;
    esac
}

main "$@"