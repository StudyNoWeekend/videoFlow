#!/usr/bin/env bash
# ============================================================
# VideoFlow One-Click Installation Script (English)
#
# Steps:
#   1. Check if Docker is installed
#   2. Deploy Whisper ASR Webservice (CPU / GPU, model selection)
#   3. Pull ladaapp/lada:latest image
#   4. Install ffmpeg locally (supports Linux / Windows / macOS)
#   5. Deploy this project (guide user to fill config.yaml, then docker run)
# ============================================================

set -euo pipefail

# ------------------- Color Definitions -------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ------------------- Global Variables -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.yaml"
CONFIG_EXAMPLE="${SCRIPT_DIR}/config.yaml.example"

WHISPER_CONTAINER_NAME="whisper-asr-webservice"
WHISPER_PORT=9000
LADA_CONTAINER_NAME="lada-app"
LADA_PORT=8080

# ------------------- Utility Functions -------------------

print_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[OK]${NC} $*"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
print_step()    { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}"; }

# Confirm action, defaults to Yes
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

# Detect operating system
detect_os() {
    case "$(uname -s)" in
        Darwin*)                  echo "macos"   ;;
        Linux*)                   echo "linux"   ;;
        MINGW*|MSYS*|CYGWIN*)     echo "windows" ;;
        *)                        echo "unknown" ;;
    esac
}

# ============================================================
# Step 1: Check Docker
# ============================================================
step1_check_docker() {
    print_step "Step 1/5: Check Docker"

    if ! command -v docker &>/dev/null; then
        print_error "Docker not found. Please install Docker before running this script."
        print_error "Installation guide: https://docs.docker.com/engine/install/"
        exit 1
    fi

    print_success "Docker detected: $(docker --version)"

    # Check if Docker daemon is running
    if ! docker info &>/dev/null; then
        print_error "Docker daemon is not running. Please start Docker and retry."
        if [[ "$(detect_os)" == "macos" ]]; then
            print_info "Hint: Please open the 'Docker Desktop' application."
        elif [[ "$(detect_os)" == "windows" ]]; then
            print_info "Hint: Please start 'Docker Desktop' or ensure the WSL2 backend is running."
        else
            print_info "Hint: Run sudo systemctl start docker"
        fi
        exit 1
    fi

    print_success "Docker daemon is running normally."
}

# ============================================================
# Step 2: Deploy Whisper ASR Webservice
# ============================================================
step2_deploy_whisper() {
    print_step "Step 2/5: Deploy Whisper ASR Webservice"

    local os
    os="$(detect_os)"

    # --- Select CPU / GPU ---
    echo -e "${BOLD}Select run mode:${NC}"
    echo "  1) CPU mode (universal for all platforms, recommended for macOS / no GPU)"
    echo "  2) GPU mode (Linux + NVIDIA GPU only, faster)"
    local mode_choice
    read -rp "$(echo -e "${YELLOW}Enter number [1]:${NC} ")" mode_choice
    mode_choice="${mode_choice:-1}"

    local image_tag=""
    local gpu_flag=""
    local asr_device="cpu"

    case "$mode_choice" in
        1)
            image_tag="latest"
            gpu_flag=""
            asr_device="cpu"
            print_info "CPU mode selected."
            ;;
        2)
            if [[ "$os" != "linux" ]]; then
                print_warn "GPU passthrough is not available on macOS / Windows (Docker runs in a Linux VM on these platforms)."
                if ! confirm "Current system is not Linux. Are you sure you want to continue with GPU mode?" "n"; then
                    print_info "Switched to CPU mode."
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
                print_info "GPU mode selected."
            fi
            ;;
        *)
            print_warn "Invalid input, defaulting to CPU mode."
            image_tag="latest"
            asr_device="cpu"
            ;;
    esac

    # --- Select ASR Engine ---
    echo ""
    echo -e "${BOLD}Select ASR engine:${NC}"
    echo "  1) openai_whisper  - Official OpenAI Whisper (default)"
    echo "  2) faster_whisper   - CTranslate2 accelerated, faster inference"
    echo "  3) whisperx         - Speaker diarization & subtitle alignment (requires HF_TOKEN)"
    local engine_choice
    read -rp "$(echo -e "${YELLOW}Enter number [1]:${NC} ")" engine_choice
    engine_choice="${engine_choice:-1}"

    local asr_engine
    case "$engine_choice" in
        1) asr_engine="openai_whisper" ;;
        2) asr_engine="faster_whisper" ;;
        3) asr_engine="whisperx" ;;
        *) asr_engine="openai_whisper" ;;
    esac
    print_info "Engine selected: ${asr_engine}"

    # --- Select Model ---
    echo ""
    echo -e "${BOLD}Select subtitle model:${NC}"
    echo "  Standard models (multilingual):"
    echo "    1) tiny            - Fastest, lowest accuracy (~1GB)"
    echo "    2) base            - Fast, lower accuracy (~1GB) [default]"
    echo "    3) small           - Balanced (~2GB)"
    echo "    4) medium          - Higher accuracy (~5GB)"
    echo "    5) large-v3        - Highest accuracy (~10GB)"
    echo "    6) large-v3-turbo  - High accuracy + fast (~ turbo)"
    echo "  English-only models:"
    echo "    7) base.en         - English base"
    echo "    8) small.en        - English small"
    echo "    9) medium.en       - English medium"
    local model_choice
    read -rp "$(echo -e "${YELLOW}Enter number [2]:${NC} ")" model_choice
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
    print_info "Model selected: ${asr_model}"

    # --- Optional: Hugging Face Token (required for whisperx) ---
    local hf_token=""
    if [[ "$asr_engine" == "whisperx" ]]; then
        echo ""
        print_warn "The whisperx engine requires a Hugging Face Token to download the speaker diarization model."
        read -rp "$(echo -e "${YELLOW}Enter HF_TOKEN (leave empty to skip, configure manually later):${NC} ")" hf_token
        if [[ -z "$hf_token" ]]; then
            print_warn "No HF_TOKEN provided, whisperx may not work properly."
            print_info "Get token: https://huggingface.co/settings/tokens"
        fi
    fi

    # --- Optional: Persist model cache ---
    echo ""
    local cache_dir="${SCRIPT_DIR}/whisper-cache"
    if confirm "Persist model cache locally? (speeds up subsequent starts, recommended)" "y"; then
        mkdir -p "$cache_dir"
        print_info "Cache directory: ${cache_dir}"
    fi

    # --- Remove existing container if present ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        print_warn "Container ${WHISPER_CONTAINER_NAME} already exists, removing old container..."
        docker rm -f "$WHISPER_CONTAINER_NAME" &>/dev/null
    fi

    # --- Pull image ---
    local full_image="onerahmet/openai-whisper-asr-webservice:${image_tag}"
    print_info "Pulling image: ${full_image} ..."
    docker pull "$full_image"

    # --- Build run arguments ---
    local run_args=(-d --name "$WHISPER_CONTAINER_NAME")
    [[ -n "$gpu_flag" ]] && run_args+=($gpu_flag)
    run_args+=(-p "${WHISPER_PORT}:9000")
    run_args+=(-e "ASR_MODEL=${asr_model}")
    run_args+=(-e "ASR_ENGINE=${asr_engine}")
    run_args+=(-e "ASR_DEVICE=${asr_device}")

    if [[ -n "$hf_token" ]]; then
        run_args+=(-e "HF_TOKEN=${hf_token}")
    fi

    # Persist cache
    if [[ -d "$cache_dir" ]]; then
        run_args+=(-v "${cache_dir}:/root/.cache")
    fi

    run_args+=("$full_image")

    # --- Start container ---
    print_info "Starting Whisper ASR container..."
    docker run "${run_args[@]}"

    # Wait for service to be ready
    print_info "Waiting for Whisper ASR service to be ready (first model download may take a while)..."
    local max_wait=300
    local waited=0
    while ! curl -sf "http://localhost:${WHISPER_PORT}/docs" &>/dev/null; do
        sleep 5
        waited=$((waited + 5))
        if [[ $waited -ge $max_wait ]]; then
            print_warn "Service not fully ready within ${max_wait}s, model may still be downloading."
            print_info "Check progress with: docker logs ${WHISPER_CONTAINER_NAME}"
            break
        fi
        echo -n "."
    done
    echo ""
    print_success "Whisper ASR Webservice started: http://localhost:${WHISPER_PORT}"
    print_info "Swagger docs: http://localhost:${WHISPER_PORT}/docs"
}

# ============================================================
# Step 3: Pull ladaapp/lada:latest
# ============================================================
step3_pull_lada() {
    print_step "Step 3/5: Pull ladaapp/lada:latest"

    local lada_image="ladaapp/lada:latest"
    print_info "Pulling image: ${lada_image} ..."
    docker pull "$lada_image"
    print_success "Image ${lada_image} pulled successfully."

    # Ask whether to start Lada container immediately
    if confirm "Start Lada container now?" "y"; then
        if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
            print_warn "Container ${LADA_CONTAINER_NAME} already exists, removing old container..."
            docker rm -f "$LADA_CONTAINER_NAME" &>/dev/null
        fi
        docker run -d --name "$LADA_CONTAINER_NAME" -p "${LADA_PORT}:8080" "$lada_image"
        print_success "Lada container started: http://localhost:${LADA_PORT}"
    else
        print_info "Skipped starting Lada container, you can run it manually later."
    fi
}

# ============================================================
# Step 4: Install ffmpeg (supports Linux / Windows / macOS)
# ============================================================
step4_install_ffmpeg() {
    print_step "Step 4/5: Install ffmpeg"

    # Check if already installed
    if command -v ffmpeg &>/dev/null; then
        local current_ver
        current_ver="$(ffmpeg -version 2>&1 | head -n1)"
        print_success "ffmpeg is already installed: ${current_ver}"
        if ! confirm "Reinstall/upgrade ffmpeg?" "n"; then
            print_info "Skipping ffmpeg installation."
            return 0
        fi
    fi

    local os
    os="$(detect_os)"
    print_info "Detected operating system: ${os}"

    case "$os" in
        macos)
            if command -v brew &>/dev/null; then
                print_info "Installing ffmpeg via Homebrew ..."
                brew install ffmpeg
            else
                print_warn "Homebrew not detected, attempting to install Homebrew automatically ..."
                if confirm "Install Homebrew?" "y"; then
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                    brew install ffmpeg
                else
                    print_error "Please install ffmpeg manually: https://ffmpeg.org/download.html"
                    exit 1
                fi
            fi
            ;;

        linux)
            if command -v apt-get &>/dev/null; then
                print_info "Installing ffmpeg via apt-get ..."
                sudo apt-get update && sudo apt-get install -y ffmpeg
            elif command -v dnf &>/dev/null; then
                print_info "Installing ffmpeg via dnf ..."
                sudo dnf install -y ffmpeg
            elif command -v yum &>/dev/null; then
                print_info "Installing ffmpeg via yum ..."
                sudo yum install -y ffmpeg
            elif command -v pacman &>/dev/null; then
                print_info "Installing ffmpeg via pacman ..."
                sudo pacman -S --noconfirm ffmpeg
            elif command -v apk &>/dev/null; then
                print_info "Installing ffmpeg via apk ..."
                sudo apk add --no-cache ffmpeg
            else
                print_error "No supported package manager detected. Please install ffmpeg manually: https://ffmpeg.org/download.html"
                exit 1
            fi
            ;;

        windows)
            # On Windows, try winget first, then choco
            if command -v winget &>/dev/null; then
                print_info "Installing ffmpeg via winget ..."
                winget install --id Gyan.FFmpeg -e --accept-package-agreements --accept-source-agreements
            elif command -v choco &>/dev/null; then
                print_info "Installing ffmpeg via Chocolatey ..."
                choco install ffmpeg -y
            else
                print_warn "Neither winget nor Chocolatey detected."
                print_info "Please install ffmpeg using one of the following methods:"
                echo "  1. Install winget:  https://learn.microsoft.com/windows/package-manager/winget/"
                echo "  2. Install Chocolatey: https://chocolatey.org/install"
                echo "  3. Manual download:    https://www.gyan.dev/ffmpeg/builds/"
                echo ""
                read -rp "$(echo -e "${YELLOW}Press Enter after ffmpeg installation is complete...${NC}")"
            fi
            ;;

        *)
            print_error "Unsupported operating system. Please install ffmpeg manually: https://ffmpeg.org/download.html"
            exit 1
            ;;
    esac

    # Verify installation
    if command -v ffmpeg &>/dev/null; then
        print_success "ffmpeg installed successfully: $(ffmpeg -version 2>&1 | head -n1)"
    else
        print_error "ffmpeg still not detected in PATH after installation. Please check your environment variables."
        exit 1
    fi
}

# ============================================================
# Step 5: Deploy this project
# ============================================================
step5_deploy_project() {
    print_step "Step 5/5: Deploy this project"

    # --- Ensure config.yaml exists ---
    if [[ ! -f "$CONFIG_EXAMPLE" ]]; then
        print_error "Config template file not found: ${CONFIG_EXAMPLE}"
        exit 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_info "First run, creating config.yaml from template ..."
        cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"
    fi

    # --- Guide user to edit config ---
    echo ""
    echo -e "${BOLD}Please review and complete the config file: ${CYAN}${CONFIG_FILE}${NC}"
    echo -e "${YELLOW}Key config items:${NC}"
    echo "  whisper.model   - Must match the model selected in Step 2"
    echo "  whisper.engine  - Must match the engine selected in Step 2"
    echo "  whisper.host    - Whisper ASR service address (default http://localhost:9000)"
    echo "  io.input_dir    - Input video/audio directory"
    echo "  io.output_dir   - Output directory"
    echo "  container.image - Project Docker image name"
    echo "  container.port  - Container mapped port"
    echo ""

    if confirm "Open config.yaml in an editor now?" "y"; then
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
                print_warn "No available editor found, please edit manually: ${CONFIG_FILE}"
                editor=""
            fi
        fi

        if [[ -n "$editor" ]]; then
            print_info "Opening editor: ${editor}"
            $editor "$CONFIG_FILE"
        fi
    fi

    # --- Wait for user confirmation ---
    if ! confirm "Is config.yaml setup complete? Continue with deployment?" "n"; then
        print_warn "Please complete the config and re-run this script (completed steps will be skipped automatically)."
        print_info "Or run: bash install.en.sh"
        exit 0
    fi

    # --- Read key config from config.yaml (simple parsing) ---
    local project_image project_name project_port input_dir output_dir
    project_image=$(grep -E '^\s*image:' "$CONFIG_FILE" | head -1 | sed 's/.*image:\s*//' | tr -d '"' || echo "video-captions:latest")
    project_name=$(grep -E '^\s*name:' "$CONFIG_FILE" | head -1 | sed 's/.*name:\s*//' | tr -d '"' || echo "video-captions")
    project_port=$(grep -E '^\s*port:' "$CONFIG_FILE" | tail -1 | sed 's/.*port:\s*//' | tr -d ' ' || echo "9001")
    input_dir=$(grep -E '^\s*input_dir:' "$CONFIG_FILE" | sed 's/.*input_dir:\s*//' | tr -d '"' || echo "./input")
    output_dir=$(grep -E '^\s*output_dir:' "$CONFIG_FILE" | sed 's/.*output_dir:\s*//' | tr -d '"' || echo "./output")

    # Ensure directories exist
    mkdir -p "$input_dir" "$output_dir"
    print_info "Input directory: ${input_dir}"
    print_info "Output directory: ${output_dir}"

    # --- Check if image exists ---
    if ! docker image inspect "$project_image" &>/dev/null; then
        print_warn "Image not found locally: ${project_image}"
        if [[ -f "${SCRIPT_DIR}/Dockerfile" ]]; then
            if confirm "Dockerfile found, build the image?" "y"; then
                print_info "Building image: ${project_image} ..."
                docker build -t "$project_image" "$SCRIPT_DIR"
            else
                print_error "Please build or pull the image first: ${project_image}"
                exit 1
            fi
        else
            print_error "Image ${project_image} does not exist and no Dockerfile found."
            print_info "Please build the project image first and re-run, or modify container.image in config.yaml."
            exit 1
        fi
    fi

    # --- Remove old container ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$project_name"; then
        print_warn "Container ${project_name} already exists, removing old container..."
        docker rm -f "$project_name" &>/dev/null
    fi

    # --- Start project container ---
    print_info "Starting project container..."
    docker run -d \
        --name "$project_name" \
        -p "${project_port}:${project_port}" \
        -v "${CONFIG_FILE}:/app/config.yaml" \
        -v "${input_dir}:/app/input" \
        -v "${output_dir}:/app/output" \
        --link "${WHISPER_CONTAINER_NAME}:whisper" \
        --link "${LADA_CONTAINER_NAME}:lada" \
        "$project_image"

    print_success "Project container started: ${project_name}"
    print_info "Access port: http://localhost:${project_port}"
    echo ""
    echo -e "${GREEN}${BOLD}========== Installation complete! ==========${NC}"
    echo -e "  Whisper ASR : http://localhost:${WHISPER_PORT}/docs"
    echo -e "  Lada        : http://localhost:${LADA_PORT}"
    echo -e "  Project     : http://localhost:${project_port}"
    echo -e "  Config file : ${CONFIG_FILE}"
    echo -e "  View logs   : docker logs -f ${project_name}"
    echo ""
}

# ============================================================
# Main
# ============================================================
main() {
    echo -e "${GREEN}${BOLD}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║     VideoFlow One-Click Installation Script    ║"
    echo "║     Dependencies: Docker / Whisper / Lada / ffmpeg ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"

    step1_check_docker
    step2_deploy_whisper
    step3_pull_lada
    step4_install_ffmpeg
    step5_deploy_project
}

main "$@"
