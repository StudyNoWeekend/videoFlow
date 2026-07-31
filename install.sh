#!/usr/bin/env bash
# ============================================================
# Video Captions 一键安装脚本
#
# 功能步骤：
#   1. 检测 Docker 是否安装
#   2. 部署 Whisper ASR Webservice（可选 CPU / GPU，可选模型）
#   3. 拉取 ladaapp/lada:latest 镜像
#   4. 本地安装 ffmpeg（支持 Linux / Windows / macOS）
#   5. 部署本项目（引导用户填写 config.yaml 后挂载 docker run）
# ============================================================

set -euo pipefail

# ------------------- 颜色定义 -------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ------------------- 全局变量 -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.yaml"
CONFIG_EXAMPLE="${SCRIPT_DIR}/config.yaml.example"

WHISPER_CONTAINER_NAME="whisper-asr-webservice"
WHISPER_PORT=9000
LADA_CONTAINER_NAME="lada-app"
LADA_PORT=8080

# ------------------- 工具函数 -------------------

print_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[OK]${NC} $*"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
print_step()    { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}"; }

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

# ============================================================
# 步骤 1：检测 Docker
# ============================================================
step1_check_docker() {
    print_step "步骤 1/5：检测 Docker"

    if ! command -v docker &>/dev/null; then
        print_error "未检测到 Docker，请先安装 Docker 后再运行本脚本。"
        print_error "安装指引: https://docs.docker.com/engine/install/"
        exit 1
    fi

    print_success "已检测到 Docker: $(docker --version)"

    # 检测 Docker 守护进程是否运行
    if ! docker info &>/dev/null; then
        print_error "Docker 守护进程未运行，请启动 Docker 后重试。"
        if [[ "$(detect_os)" == "macos" ]]; then
            print_info "提示: 请打开「Docker Desktop」应用。"
        elif [[ "$(detect_os)" == "windows" ]]; then
            print_info "提示: 请启动「Docker Desktop」或确保 WSL2 后端已运行。"
        else
            print_info "提示: 请执行 sudo systemctl start docker"
        fi
        exit 1
    fi

    print_success "Docker 守护进程运行正常。"
}

# ============================================================
# 步骤 2：部署 Whisper ASR Webservice
# ============================================================
step2_deploy_whisper() {
    print_step "步骤 2/5：部署 Whisper ASR Webservice"

    local os
    os="$(detect_os)"

    # --- 选择 CPU / GPU ---
    echo -e "${BOLD}请选择运行模式：${NC}"
    echo "  1) CPU 模式（所有平台通用，推荐 macOS / 无 GPU 用户）"
    echo "  2) GPU 模式（仅 Linux + NVIDIA GPU，速度更快）"
    local mode_choice
    read -rp "$(echo -e "${YELLOW}请输入序号 [1]:${NC} ")" mode_choice
    mode_choice="${mode_choice:-1}"

    local image_tag=""
    local gpu_flag=""
    local asr_device="cpu"

    case "$mode_choice" in
        1)
            image_tag="latest"
            gpu_flag=""
            asr_device="cpu"
            print_info "已选择 CPU 模式。"
            ;;
        2)
            if [[ "$os" != "linux" ]]; then
                print_warn "GPU 直通在 macOS / Windows 上不可用（Docker 在这些平台上运行于 Linux VM 中）。"
                if ! confirm "当前系统非 Linux，确定要继续使用 GPU 模式吗？" "n"; then
                    print_info "已切换为 CPU 模式。"
                    image_tag="latest"
                    asr_device="cpu"
                else
                    image_tag="latest-gpu"
                    gpu_flag="--gpus all"
                    asr_device="cuda"
                fi
            else
                image_tag="latest-gpu"
                gpu_flag="--gpus all"
                asr_device="cuda"
                print_info "已选择 GPU 模式。"
            fi
            ;;
        *)
            print_warn "无效输入，默认使用 CPU 模式。"
            image_tag="latest"
            asr_device="cpu"
            ;;
    esac

    # --- 选择 ASR 引擎 ---
    echo ""
    echo -e "${BOLD}请选择 ASR 引擎：${NC}"
    echo "  1) openai_whisper  — 官方 OpenAI Whisper（默认）"
    echo "  2) faster_whisper   — CTranslate2 加速版本，推理更快"
    echo "  3) whisperx         — 支持说话人分离与字幕对齐（需 HF_TOKEN）"
    local engine_choice
    read -rp "$(echo -e "${YELLOW}请输入序号 [1]:${NC} ")" engine_choice
    engine_choice="${engine_choice:-1}"

    local asr_engine
    case "$engine_choice" in
        1) asr_engine="openai_whisper" ;;
        2) asr_engine="faster_whisper" ;;
        3) asr_engine="whisperx" ;;
        *) asr_engine="openai_whisper" ;;
    esac
    print_info "已选择引擎: ${asr_engine}"

    # --- 选择模型 ---
    echo ""
    echo -e "${BOLD}请选择字幕模型：${NC}"
    echo "  标准模型（多语言）:"
    echo "    1) tiny            — 最快，精度最低 (~1GB)"
    echo "    2) base            — 快速，精度较低 (~1GB) [默认]"
    echo "    3) small           — 均衡 (~2GB)"
    echo "    4) medium          — 精度较高 (~5GB)"
    echo "    5) large-v3        — 精度最高 (~10GB)"
    echo "    6) large-v3-turbo  — 高精度 + 快速 (~ turbo)"
    echo "  英文专用模型:"
    echo "    7) base.en         — 英文 base"
    echo "    8) small.en        — 英文 small"
    echo "    9) medium.en       — 英文 medium"
    local model_choice
    read -rp "$(echo -e "${YELLOW}请输入序号 [2]:${NC} ")" model_choice
    model_choice="${model_choice:-2}"

    local asr_model
    case "$model_choice" in
        1) asr_model="tiny" ;;
        2) asr_model="base" ;;
        3) asr_model="small" ;;
        4) asr_model="medium" ;;
        5) asr_model="large-v3" ;;
        6) asr_model="large-v3-turbo" ;;
        7) asr_model="base.en" ;;
        8) asr_model="small.en" ;;
        9) asr_model="medium.en" ;;
        *) asr_model="base" ;;
    esac
    print_info "已选择模型: ${asr_model}"

    # --- 可选：Hugging Face Token（whisperx 需要）---
    local hf_token=""
    if [[ "$asr_engine" == "whisperx" ]]; then
        echo ""
        print_warn "whisperx 引擎需要 Hugging Face Token 来下载说话人分离模型。"
        read -rp "$(echo -e "${YELLOW}请输入 HF_TOKEN（留空则跳过，后续手动配置）:${NC} ")" hf_token
        if [[ -z "$hf_token" ]]; then
            print_warn "未提供 HF_TOKEN，whisperx 可能无法正常工作。"
            print_info "获取 Token: https://huggingface.co/settings/tokens"
        fi
    fi

    # --- 可选：持久化模型缓存 ---
    echo ""
    local cache_dir="${SCRIPT_DIR}/whisper-cache"
    if confirm "是否持久化模型缓存到本地？（加速后续启动，推荐）" "y"; then
        mkdir -p "$cache_dir"
        print_info "缓存目录: ${cache_dir}"
    fi

    # --- 如果容器已存在则先移除 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        print_warn "已存在同名容器 ${WHISPER_CONTAINER_NAME}，正在移除旧容器..."
        docker rm -f "$WHISPER_CONTAINER_NAME" &>/dev/null
    fi

    # --- 拉取镜像 ---
    local full_image="onerahmet/openai-whisper-asr-webservice:${image_tag}"
    print_info "正在拉取镜像: ${full_image} ..."
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
    if [[ -d "$cache_dir" ]]; then
        run_args+=(-v "${cache_dir}:/root/.cache")
    fi

    run_args+=("$full_image")

    # --- 启动容器 ---
    print_info "正在启动 Whisper ASR 容器..."
    docker run "${run_args[@]}"

    # 等待服务就绪
    print_info "等待 Whisper ASR 服务就绪（模型首次下载可能需要较长时间）..."
    local max_wait=300
    local waited=0
    while ! curl -sf "http://localhost:${WHISPER_PORT}/docs" &>/dev/null; do
        sleep 5
        waited=$((waited + 5))
        if [[ $waited -ge $max_wait ]]; then
            print_warn "服务在 ${max_wait}s 内未完全就绪，可能仍在下载模型。"
            print_info "可通过 docker logs ${WHISPER_CONTAINER_NAME} 查看进度。"
            break
        fi
        echo -n "."
    done
    echo ""
    print_success "Whisper ASR Webservice 已启动: http://localhost:${WHISPER_PORT}"
    print_info "Swagger 文档: http://localhost:${WHISPER_PORT}/docs"
}

# ============================================================
# 步骤 3：拉取 ladaapp/lada:latest
# ============================================================
step3_pull_lada() {
    print_step "步骤 3/5：拉取 ladaapp/lada:latest"

    local lada_image="ladaapp/lada:latest"
    print_info "正在拉取镜像: ${lada_image} ..."
    docker pull "$lada_image"
    print_success "镜像 ${lada_image} 拉取完成。"

    # 询问是否立即启动 Lada 容器
    if confirm "是否立即启动 Lada 容器？" "y"; then
        if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
            print_warn "已存在同名容器 ${LADA_CONTAINER_NAME}，正在移除旧容器..."
            docker rm -f "$LADA_CONTAINER_NAME" &>/dev/null
        fi
        docker run -d --name "$LADA_CONTAINER_NAME" -p "${LADA_PORT}:8080" "$lada_image"
        print_success "Lada 容器已启动: http://localhost:${LADA_PORT}"
    else
        print_info "已跳过启动 Lada 容器，后续可手动运行。"
    fi
}

# ============================================================
# 步骤 4：安装 ffmpeg（支持 Linux / Windows / macOS）
# ============================================================
step4_install_ffmpeg() {
    print_step "步骤 4/5：安装 ffmpeg"

    # 检测是否已安装
    if command -v ffmpeg &>/dev/null; then
        local current_ver
        current_ver="$(ffmpeg -version 2>&1 | head -n1)"
        print_success "ffmpeg 已安装: ${current_ver}"
        if ! confirm "是否重新安装/升级 ffmpeg？" "n"; then
            print_info "跳过 ffmpeg 安装。"
            return 0
        fi
    fi

    local os
    os="$(detect_os)"
    print_info "检测到操作系统: ${os}"

    case "$os" in
        macos)
            if command -v brew &>/dev/null; then
                print_info "使用 Homebrew 安装 ffmpeg ..."
                brew install ffmpeg
            else
                print_warn "未检测到 Homebrew，正在尝试自动安装 Homebrew ..."
                if confirm "是否安装 Homebrew？" "y"; then
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                    brew install ffmpeg
                else
                    print_error "请手动安装 ffmpeg: https://ffmpeg.org/download.html"
                    exit 1
                fi
            fi
            ;;

        linux)
            if command -v apt-get &>/dev/null; then
                print_info "使用 apt-get 安装 ffmpeg ..."
                sudo apt-get update && sudo apt-get install -y ffmpeg
            elif command -v dnf &>/dev/null; then
                print_info "使用 dnf 安装 ffmpeg ..."
                sudo dnf install -y ffmpeg
            elif command -v yum &>/dev/null; then
                print_info "使用 yum 安装 ffmpeg ..."
                sudo yum install -y ffmpeg
            elif command -v pacman &>/dev/null; then
                print_info "使用 pacman 安装 ffmpeg ..."
                sudo pacman -S --noconfirm ffmpeg
            elif command -v apk &>/dev/null; then
                print_info "使用 apk 安装 ffmpeg ..."
                sudo apk add --no-cache ffmpeg
            else
                print_error "未识别到支持的包管理器，请手动安装 ffmpeg: https://ffmpeg.org/download.html"
                exit 1
            fi
            ;;

        windows)
            # Windows 下优先尝试 winget，其次 choco
            if command -v winget &>/dev/null; then
                print_info "使用 winget 安装 ffmpeg ..."
                winget install --id Gyan.FFmpeg -e --accept-package-agreements --accept-source-agreements
            elif command -v choco &>/dev/null; then
                print_info "使用 Chocolatey 安装 ffmpeg ..."
                choco install ffmpeg -y
            else
                print_warn "未检测到 winget 或 Chocolatey。"
                print_info "请通过以下方式之一安装 ffmpeg："
                echo "  1. 安装 winget:  https://learn.microsoft.com/windows/package-manager/winget/"
                echo "  2. 安装 Chocolatey: https://chocolatey.org/install"
                echo "  3. 手动下载:     https://www.gyan.dev/ffmpeg/builds/"
                echo ""
                read -rp "$(echo -e "${YELLOW}ffmpeg 安装完成后按回车继续...${NC}")"
            fi
            ;;

        *)
            print_error "不支持的操作系统，请手动安装 ffmpeg: https://ffmpeg.org/download.html"
            exit 1
            ;;
    esac

    # 验证安装
    if command -v ffmpeg &>/dev/null; then
        print_success "ffmpeg 安装成功: $(ffmpeg -version 2>&1 | head -n1)"
    else
        print_error "ffmpeg 安装后仍未在 PATH 中检测到，请检查环境变量。"
        exit 1
    fi
}

# ============================================================
# 步骤 5：部署本项目
# ============================================================
step5_deploy_project() {
    print_step "步骤 5/5：部署本项目"

    # --- 确保 config.yaml 存在 ---
    if [[ ! -f "$CONFIG_EXAMPLE" ]]; then
        print_error "未找到配置模板文件: ${CONFIG_EXAMPLE}"
        exit 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_info "首次运行，从模板创建 config.yaml ..."
        cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"
    fi

    # --- 引导用户编辑配置 ---
    echo ""
    echo -e "${BOLD}请检查并完善配置文件: ${CYAN}${CONFIG_FILE}${NC}"
    echo -e "${YELLOW}关键配置项说明：${NC}"
    echo "  whisper.model   — 需与步骤 2 选择的模型一致"
    echo "  whisper.engine  — 需与步骤 2 选择的引擎一致"
    echo "  whisper.host    — Whisper ASR 服务地址（默认 http://localhost:9000）"
    echo "  io.input_dir    — 输入视频/音频目录"
    echo "  io.output_dir   — 输出目录"
    echo "  container.image — 项目 Docker 镜像名"
    echo "  container.port  — 容器映射端口"
    echo ""

    if confirm "是否现在用编辑器打开 config.yaml 进行编辑？" "y"; then
        local editor="${EDITOR:-}"
        if [[ -z "$editor" ]]; then
            if command -v nano &>/dev/null; then
                editor="nano"
            elif command -v vim &>/dev/null; then
                editor="vim"
            elif command -v vi &>/dev/null; then
                editor="vi"
            elif command -v code &>/dev/null; then
                editor="code --wait"
            else
                print_warn "未找到可用编辑器，请手动编辑: ${CONFIG_FILE}"
                editor=""
            fi
        fi

        if [[ -n "$editor" ]]; then
            print_info "正在打开编辑器: ${editor}"
            $editor "$CONFIG_FILE"
        fi
    fi

    # --- 等待用户确认 ---
    if ! confirm "config.yaml 配置是否已完成？是否继续部署？" "n"; then
        print_warn "请完成配置后重新运行本脚本（已完成的步骤会自动跳过）。"
        print_info "或直接运行: bash install.sh"
        exit 0
    fi

    # --- 从 config.yaml 中读取关键配置（简易解析）---
    local project_image project_name project_port input_dir output_dir
    project_image=$(grep -E '^\s*image:' "$CONFIG_FILE" | head -1 | sed 's/.*image:\s*//' | tr -d '"' || echo "video-captions:latest")
    project_name=$(grep -E '^\s*name:' "$CONFIG_FILE" | head -1 | sed 's/.*name:\s*//' | tr -d '"' || echo "video-captions")
    project_port=$(grep -E '^\s*port:' "$CONFIG_FILE" | tail -1 | sed 's/.*port:\s*//' | tr -d ' ' || echo "9001")
    input_dir=$(grep -E '^\s*input_dir:' "$CONFIG_FILE" | sed 's/.*input_dir:\s*//' | tr -d '"' || echo "./input")
    output_dir=$(grep -E '^\s*output_dir:' "$CONFIG_FILE" | sed 's/.*output_dir:\s*//' | tr -d '"' || echo "./output")

    # 确保目录存在
    mkdir -p "$input_dir" "$output_dir"
    print_info "输入目录: ${input_dir}"
    print_info "输出目录: ${output_dir}"

    # --- 检查镜像是否存在 ---
    if ! docker image inspect "$project_image" &>/dev/null; then
        print_warn "本地未找到镜像: ${project_image}"
        if [[ -f "${SCRIPT_DIR}/Dockerfile" ]]; then
            if confirm "发现 Dockerfile，是否构建镜像？" "y"; then
                print_info "正在构建镜像: ${project_image} ..."
                docker build -t "$project_image" "$SCRIPT_DIR"
            else
                print_error "请先构建或拉取镜像: ${project_image}"
                exit 1
            fi
        else
            print_error "镜像 ${project_image} 不存在且未找到 Dockerfile。"
            print_info "请先构建项目镜像后重新运行，或修改 config.yaml 中的 container.image。"
            exit 1
        fi
    fi

    # --- 移除旧容器 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$project_name"; then
        print_warn "已存在同名容器 ${project_name}，正在移除旧容器..."
        docker rm -f "$project_name" &>/dev/null
    fi

    # --- 启动项目容器 ---
    print_info "正在启动项目容器..."
    docker run -d \
        --name "$project_name" \
        -p "${project_port}:${project_port}" \
        -v "${CONFIG_FILE}:/app/config.yaml" \
        -v "${input_dir}:/app/input" \
        -v "${output_dir}:/app/output" \
        --link "${WHISPER_CONTAINER_NAME}:whisper" \
        --link "${LADA_CONTAINER_NAME}:lada" \
        "$project_image"

    print_success "项目容器已启动: ${project_name}"
    print_info "访问端口: http://localhost:${project_port}"
    echo ""
    echo -e "${GREEN}${BOLD}========== 安装完成！==========${NC}"
    echo -e "  Whisper ASR : http://localhost:${WHISPER_PORT}/docs"
    echo -e "  Lada        : http://localhost:${LADA_PORT}"
    echo -e "  项目服务    : http://localhost:${project_port}"
    echo -e "  配置文件    : ${CONFIG_FILE}"
    echo -e "  查看日志    : docker logs -f ${project_name}"
    echo ""
}

# ============================================================
# 主流程
# ============================================================
main() {
    echo -e "${GREEN}${BOLD}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║     Video Captions 一键安装脚本               ║"
    echo "║     依赖: Docker / Whisper ASR / Lada / ffmpeg ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"

    step1_check_docker
    step2_deploy_whisper
    step3_pull_lada
    step4_install_ffmpeg
    step5_deploy_project
}

main "$@"
