#!/usr/bin/env bash
# ============================================================
# VideoFlow 一鍵安裝腳本（繁體中文）
#
# 功能步驟：
#   1. 檢測 Docker 是否安裝
#   2. 部署 Whisper ASR Webservice（可選 CPU / GPU，可選模型）
#   3. 拉取 ladaapp/lada:latest 映像
#   4. 本地安裝 ffmpeg（支援 Linux / Windows / macOS）
#   5. 部署本專案（引導使用者填寫 config.yaml 後掛載 docker run）
# ============================================================

set -euo pipefail

# ------------------- 顏色定義 -------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ------------------- 全域變數 -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.yaml"
CONFIG_EXAMPLE="${SCRIPT_DIR}/config.yaml.example"

WHISPER_CONTAINER_NAME="whisper-asr-webservice"
WHISPER_PORT=9000
LADA_CONTAINER_NAME="lada-app"
LADA_PORT=8080

# ------------------- 工具函式 -------------------

print_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[OK]${NC} $*"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
print_step()    { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}"; }

# 確認操作，預設回傳 Yes
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

# 檢測作業系統
detect_os() {
    case "$(uname -s)" in
        Darwin*)                  echo "macos"   ;;
        Linux*)                   echo "linux"   ;;
        MINGW*|MSYS*|CYGWIN*)     echo "windows" ;;
        *)                        echo "unknown" ;;
    esac
}

# ============================================================
# 步驟 1：檢測 Docker
# ============================================================
step1_check_docker() {
    print_step "步驟 1/5：檢測 Docker"

    if ! command -v docker &>/dev/null; then
        print_error "未檢測到 Docker，請先安裝 Docker 後再執行本腳本。"
        print_error "安裝指引: https://docs.docker.com/engine/install/"
        exit 1
    fi

    print_success "已檢測到 Docker: $(docker --version)"

    # 檢測 Docker 常駐程式是否執行
    if ! docker info &>/dev/null; then
        print_error "Docker 常駐程式未執行，請啟動 Docker 後重試。"
        if [[ "$(detect_os)" == "macos" ]]; then
            print_info "提示: 請開啟「Docker Desktop」應用程式。"
        elif [[ "$(detect_os)" == "windows" ]]; then
            print_info "提示: 請啟動「Docker Desktop」或確保 WSL2 後端已執行。"
        else
            print_info "提示: 請執行 sudo systemctl start docker"
        fi
        exit 1
    fi

    print_success "Docker 常駐程式執行正常。"
}

# ============================================================
# 步驟 2：部署 Whisper ASR Webservice
# ============================================================
step2_deploy_whisper() {
    print_step "步驟 2/5：部署 Whisper ASR Webservice"

    local os
    os="$(detect_os)"

    # --- 選擇 CPU / GPU ---
    echo -e "${BOLD}請選擇執行模式：${NC}"
    echo "  1) CPU 模式（所有平台通用，推薦 macOS / 無 GPU 使用者）"
    echo "  2) GPU 模式（僅 Linux + NVIDIA GPU，速度更快）"
    local mode_choice
    read -rp "$(echo -e "${YELLOW}請輸入序號 [1]:${NC} ")" mode_choice
    mode_choice="${mode_choice:-1}"

    local image_tag=""
    local gpu_flag=""
    local asr_device="cpu"

    case "$mode_choice" in
        1)
            image_tag="latest"
            gpu_flag=""
            asr_device="cpu"
            print_info "已選擇 CPU 模式。"
            ;;
        2)
            if [[ "$os" != "linux" ]]; then
                print_warn "GPU 直通在 macOS / Windows 上不可用（Docker 在這些平台上執行於 Linux VM 中）。"
                if ! confirm "當前系統非 Linux，確定要繼續使用 GPU 模式嗎？" "n"; then
                    print_info "已切換為 CPU 模式。"
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
                print_info "已選擇 GPU 模式。"
            fi
            ;;
        *)
            print_warn "無效輸入，預設使用 CPU 模式。"
            image_tag="latest"
            asr_device="cpu"
            ;;
    esac

    # --- 選擇 ASR 引擎 ---
    echo ""
    echo -e "${BOLD}請選擇 ASR 引擎：${NC}"
    echo "  1) openai_whisper  - 官方 OpenAI Whisper（預設）"
    echo "  2) faster_whisper   - CTranslate2 加速版本，推論更快"
    echo "  3) whisperx         - 支援說話人分離與字幕對齊（需 HF_TOKEN）"
    local engine_choice
    read -rp "$(echo -e "${YELLOW}請輸入序號 [1]:${NC} ")" engine_choice
    engine_choice="${engine_choice:-1}"

    local asr_engine
    case "$engine_choice" in
        1) asr_engine="openai_whisper" ;;
        2) asr_engine="faster_whisper" ;;
        3) asr_engine="whisperx" ;;
        *) asr_engine="openai_whisper" ;;
    esac
    print_info "已選擇引擎: ${asr_engine}"

    # --- 選擇模型 ---
    echo ""
    echo -e "${BOLD}請選擇字幕模型：${NC}"
    echo "  標準模型（多語言）:"
    echo "    1) tiny            - 最快，精度最低 (~1GB)"
    echo "    2) base            - 快速，精度較低 (~1GB) [預設]"
    echo "    3) small           - 均衡 (~2GB)"
    echo "    4) medium          - 精度較高 (~5GB)"
    echo "    5) large-v3        - 精度最高 (~10GB)"
    echo "    6) large-v3-turbo  - 高精度 + 快速 (~ turbo)"
    echo "  英文專用模型:"
    echo "    7) base.en         - 英文 base"
    echo "    8) small.en        - 英文 small"
    echo "    9) medium.en       - 英文 medium"
    local model_choice
    read -rp "$(echo -e "${YELLOW}請輸入序號 [2]:${NC} ")" model_choice
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
    print_info "已選擇模型: ${asr_model}"

    # --- 可選：Hugging Face Token（whisperx 需要） ---
    local hf_token=""
    if [[ "$asr_engine" == "whisperx" ]]; then
        echo ""
        print_warn "whisperx 引擎需要 Hugging Face Token 來下載說話人分離模型。"
        read -rp "$(echo -e "${YELLOW}請輸入 HF_TOKEN（留空則跳過，後續手動設定）:${NC} ")" hf_token
        if [[ -z "$hf_token" ]]; then
            print_warn "未提供 HF_TOKEN，whisperx 可能無法正常運作。"
            print_info "取得 Token: https://huggingface.co/settings/tokens"
        fi
    fi

    # --- 可選：持久化模型快取 ---
    echo ""
    local cache_dir="${SCRIPT_DIR}/whisper-cache"
    if confirm "是否持久化模型快取到本地？（加速後續啟動，推薦）" "y"; then
        mkdir -p "$cache_dir"
        print_info "快取目錄: ${cache_dir}"
    fi

    # --- 如果容器已存在則先移除 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        print_warn "已存在同名容器 ${WHISPER_CONTAINER_NAME}，正在移除舊容器..."
        docker rm -f "$WHISPER_CONTAINER_NAME" &>/dev/null
    fi

    # --- 拉取映像 ---
    local full_image="onerahmet/openai-whisper-asr-webservice:${image_tag}"
    print_info "正在拉取映像: ${full_image} ..."
    docker pull "$full_image"

    # --- 建立啟動參數 ---
    local run_args=(-d --name "$WHISPER_CONTAINER_NAME")
    [[ -n "$gpu_flag" ]] && run_args+=($gpu_flag)
    run_args+=(-p "${WHISPER_PORT}:9000")
    run_args+=(-e "ASR_MODEL=${asr_model}")
    run_args+=(-e "ASR_ENGINE=${asr_engine}")
    run_args+=(-e "ASR_DEVICE=${asr_device}")

    if [[ -n "$hf_token" ]]; then
        run_args+=(-e "HF_TOKEN=${hf_token}")
    fi

    # 持久化快取
    if [[ -d "$cache_dir" ]]; then
        run_args+=(-v "${cache_dir}:/root/.cache")
    fi

    run_args+=("$full_image")

    # --- 啟動容器 ---
    print_info "正在啟動 Whisper ASR 容器..."
    docker run "${run_args[@]}"

    # 等待服務就緒
    print_info "等待 Whisper ASR 服務就緒（模型首次下載可能需要較長時間）..."
    local max_wait=300
    local waited=0
    while ! curl -sf "http://localhost:${WHISPER_PORT}/docs" &>/dev/null; do
        sleep 5
        waited=$((waited + 5))
        if [[ $waited -ge $max_wait ]]; then
            print_warn "服務在 ${max_wait}s 內未完全就緒，可能仍在下載模型。"
            print_info "可透過 docker logs ${WHISPER_CONTAINER_NAME} 查看進度。"
            break
        fi
        echo -n "."
    done
    echo ""
    print_success "Whisper ASR Webservice 已啟動: http://localhost:${WHISPER_PORT}"
    print_info "Swagger 文件: http://localhost:${WHISPER_PORT}/docs"
}

# ============================================================
# 步驟 3：拉取 ladaapp/lada:latest
# ============================================================
step3_pull_lada() {
    print_step "步驟 3/5：拉取 ladaapp/lada:latest"

    local lada_image="ladaapp/lada:latest"
    print_info "正在拉取映像: ${lada_image} ..."
    docker pull "$lada_image"
    print_success "映像 ${lada_image} 拉取完成。"

    # 詢問是否立即啟動 Lada 容器
    if confirm "是否立即啟動 Lada 容器？" "y"; then
        if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
            print_warn "已存在同名容器 ${LADA_CONTAINER_NAME}，正在移除舊容器..."
            docker rm -f "$LADA_CONTAINER_NAME" &>/dev/null
        fi
        docker run -d --name "$LADA_CONTAINER_NAME" -p "${LADA_PORT}:8080" "$lada_image"
        print_success "Lada 容器已啟動: http://localhost:${LADA_PORT}"
    else
        print_info "已跳過啟動 Lada 容器，後續可手動執行。"
    fi
}

# ============================================================
# 步驟 4：安裝 ffmpeg（支援 Linux / Windows / macOS）
# ============================================================
step4_install_ffmpeg() {
    print_step "步驟 4/5：安裝 ffmpeg"

    # 檢測是否已安裝
    if command -v ffmpeg &>/dev/null; then
        local current_ver
        current_ver="$(ffmpeg -version 2>&1 | head -n1)"
        print_success "ffmpeg 已安裝: ${current_ver}"
        if ! confirm "是否重新安裝/升級 ffmpeg？" "n"; then
            print_info "跳過 ffmpeg 安裝。"
            return 0
        fi
    fi

    local os
    os="$(detect_os)"
    print_info "檢測到作業系統: ${os}"

    case "$os" in
        macos)
            if command -v brew &>/dev/null; then
                print_info "使用 Homebrew 安裝 ffmpeg ..."
                brew install ffmpeg
            else
                print_warn "未檢測到 Homebrew，正在嘗試自動安裝 Homebrew ..."
                if confirm "是否安裝 Homebrew？" "y"; then
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                    brew install ffmpeg
                else
                    print_error "請手動安裝 ffmpeg: https://ffmpeg.org/download.html"
                    exit 1
                fi
            fi
            ;;

        linux)
            if command -v apt-get &>/dev/null; then
                print_info "使用 apt-get 安裝 ffmpeg ..."
                sudo apt-get update && sudo apt-get install -y ffmpeg
            elif command -v dnf &>/dev/null; then
                print_info "使用 dnf 安裝 ffmpeg ..."
                sudo dnf install -y ffmpeg
            elif command -v yum &>/dev/null; then
                print_info "使用 yum 安裝 ffmpeg ..."
                sudo yum install -y ffmpeg
            elif command -v pacman &>/dev/null; then
                print_info "使用 pacman 安裝 ffmpeg ..."
                sudo pacman -S --noconfirm ffmpeg
            elif command -v apk &>/dev/null; then
                print_info "使用 apk 安裝 ffmpeg ..."
                sudo apk add --no-cache ffmpeg
            else
                print_error "未識別到支援的套件管理器，請手動安裝 ffmpeg: https://ffmpeg.org/download.html"
                exit 1
            fi
            ;;

        windows)
            # Windows 下優先嘗試 winget，其次 choco
            if command -v winget &>/dev/null; then
                print_info "使用 winget 安裝 ffmpeg ..."
                winget install --id Gyan.FFmpeg -e --accept-package-agreements --accept-source-agreements
            elif command -v choco &>/dev/null; then
                print_info "使用 Chocolatey 安裝 ffmpeg ..."
                choco install ffmpeg -y
            else
                print_warn "未檢測到 winget 或 Chocolatey。"
                print_info "請透過以下方式之一安裝 ffmpeg："
                echo "  1. 安裝 winget:  https://learn.microsoft.com/windows/package-manager/winget/"
                echo "  2. 安裝 Chocolatey: https://chocolatey.org/install"
                echo "  3. 手動下載:     https://www.gyan.dev/ffmpeg/builds/"
                echo ""
                read -rp "$(echo -e "${YELLOW}ffmpeg 安裝完成後按 Enter 繼續...${NC}")"
            fi
            ;;

        *)
            print_error "不支援的作業系統，請手動安裝 ffmpeg: https://ffmpeg.org/download.html"
            exit 1
            ;;
    esac

    # 驗證安裝
    if command -v ffmpeg &>/dev/null; then
        print_success "ffmpeg 安裝成功: $(ffmpeg -version 2>&1 | head -n1)"
    else
        print_error "ffmpeg 安裝後仍未在 PATH 中檢測到，請檢查環境變數。"
        exit 1
    fi
}

# ============================================================
# 步驟 5：部署本專案
# ============================================================
step5_deploy_project() {
    print_step "步驟 5/5：部署本專案"

    # --- 確保 config.yaml 存在 ---
    if [[ ! -f "$CONFIG_EXAMPLE" ]]; then
        print_error "未找到設定範本檔案: ${CONFIG_EXAMPLE}"
        exit 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_info "首次執行，從範本建立 config.yaml ..."
        cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"
    fi

    # --- 引導使用者編輯設定 ---
    echo ""
    echo -e "${BOLD}請檢查並完善設定檔: ${CYAN}${CONFIG_FILE}${NC}"
    echo -e "${YELLOW}關鍵設定項說明：${NC}"
    echo "  whisper.model   - 需與步驟 2 選擇的模型一致"
    echo "  whisper.engine  - 需與步驟 2 選擇的引擎一致"
    echo "  whisper.host    - Whisper ASR 服務位址（預設 http://localhost:9000）"
    echo "  io.input_dir    - 輸入影片/音訊目錄"
    echo "  io.output_dir   - 輸出目錄"
    echo "  container.image - 專案 Docker 映像名稱"
    echo "  container.port  - 容器對應連接埠"
    echo ""

    if confirm "是否現在用編輯器開啟 config.yaml 進行編輯？" "y"; then
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
                print_warn "未找到可用編輯器，請手動編輯: ${CONFIG_FILE}"
                editor=""
            fi
        fi

        if [[ -n "$editor" ]]; then
            print_info "正在開啟編輯器: ${editor}"
            $editor "$CONFIG_FILE"
        fi
    fi

    # --- 等待使用者確認 ---
    if ! confirm "config.yaml 設定是否已完成？是否繼續部署？" "n"; then
        print_warn "請完成設定後重新執行本腳本（已完成的步驟會自動跳過）。"
        print_info "或直接執行: bash install.zh-TW.sh"
        exit 0
    fi

    # --- 從 config.yaml 中讀取關鍵設定（簡易解析） ---
    local project_image project_name project_port input_dir output_dir
    project_image=$(grep -E '^\s*image:' "$CONFIG_FILE" | head -1 | sed 's/.*image:\s*//' | tr -d '"' || echo "video-captions:latest")
    project_name=$(grep -E '^\s*name:' "$CONFIG_FILE" | head -1 | sed 's/.*name:\s*//' | tr -d '"' || echo "video-captions")
    project_port=$(grep -E '^\s*port:' "$CONFIG_FILE" | tail -1 | sed 's/.*port:\s*//' | tr -d ' ' || echo "9001")
    input_dir=$(grep -E '^\s*input_dir:' "$CONFIG_FILE" | sed 's/.*input_dir:\s*//' | tr -d '"' || echo "./input")
    output_dir=$(grep -E '^\s*output_dir:' "$CONFIG_FILE" | sed 's/.*output_dir:\s*//' | tr -d '"' || echo "./output")

    # 確保目錄存在
    mkdir -p "$input_dir" "$output_dir"
    print_info "輸入目錄: ${input_dir}"
    print_info "輸出目錄: ${output_dir}"

    # --- 檢查映像是否存在 ---
    if ! docker image inspect "$project_image" &>/dev/null; then
        print_warn "本地未找到映像: ${project_image}"
        if [[ -f "${SCRIPT_DIR}/Dockerfile" ]]; then
            if confirm "發現 Dockerfile，是否建置映像？" "y"; then
                print_info "正在建置映像: ${project_image} ..."
                docker build -t "$project_image" "$SCRIPT_DIR"
            else
                print_error "請先建置或拉取映像: ${project_image}"
                exit 1
            fi
        else
            print_error "映像 ${project_image} 不存在且未找到 Dockerfile。"
            print_info "請先建置專案映像後重新執行，或修改 config.yaml 中的 container.image。"
            exit 1
        fi
    fi

    # --- 移除舊容器 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$project_name"; then
        print_warn "已存在同名容器 ${project_name}，正在移除舊容器..."
        docker rm -f "$project_name" &>/dev/null
    fi

    # --- 啟動專案容器 ---
    print_info "正在啟動專案容器..."
    docker run -d \
        --name "$project_name" \
        -p "${project_port}:${project_port}" \
        -v "${CONFIG_FILE}:/app/config.yaml" \
        -v "${input_dir}:/app/input" \
        -v "${output_dir}:/app/output" \
        --link "${WHISPER_CONTAINER_NAME}:whisper" \
        --link "${LADA_CONTAINER_NAME}:lada" \
        "$project_image"

    print_success "專案容器已啟動: ${project_name}"
    print_info "存取連接埠: http://localhost:${project_port}"
    echo ""
    echo -e "${GREEN}${BOLD}========== 安裝完成！==========${NC}"
    echo -e "  Whisper ASR : http://localhost:${WHISPER_PORT}/docs"
    echo -e "  Lada        : http://localhost:${LADA_PORT}"
    echo -e "  專案服務    : http://localhost:${project_port}"
    echo -e "  設定檔      : ${CONFIG_FILE}"
    echo -e "  查看日誌    : docker logs -f ${project_name}"
    echo ""
}

# ============================================================
# 主流程
# ============================================================
main() {
    echo -e "${GREEN}${BOLD}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║     VideoFlow 一鍵安裝腳本（繁體中文）         ║"
    echo "║     相依: Docker / Whisper ASR / Lada / ffmpeg ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"

    step1_check_docker
    step2_deploy_whisper
    step3_pull_lada
    step4_install_ffmpeg
    step5_deploy_project
}

main "$@"
