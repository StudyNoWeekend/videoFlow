#!/usr/bin/env bash
# ============================================================
# VideoFlow ワンクリックインストールスクリプト（日本語）
#
# 手順：
#   1. Docker がインストールされているか確認
#   2. Whisper ASR Webservice のデプロイ（CPU / GPU、モデル選択）
#   3. ladaapp/lada:latest イメージのプル
#   4. ffmpeg のローカルインストール（Linux / Windows / macOS 対応）
#   5. 本プロジェクトのデプロイ（config.yaml 設定をガイド後、docker run で起動）
# ============================================================

set -euo pipefail

# ------------------- カラー定義 -------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ------------------- グローバル変数 -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.yaml"
CONFIG_EXAMPLE="${SCRIPT_DIR}/config.yaml.example"

WHISPER_CONTAINER_NAME="whisper-asr-webservice"
WHISPER_PORT=9000
LADA_CONTAINER_NAME="lada-app"
LADA_PORT=8080

# ------------------- ユーティリティ関数 -------------------

print_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[OK]${NC} $*"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
print_step()    { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}"; }

# 確認（デフォルトは Yes）
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

# OS の検出
detect_os() {
    case "$(uname -s)" in
        Darwin*)                  echo "macos"   ;;
        Linux*)                   echo "linux"   ;;
        MINGW*|MSYS*|CYGWIN*)     echo "windows" ;;
        *)                        echo "unknown" ;;
    esac
}

# ============================================================
# ステップ 1：Docker の確認
# ============================================================
step1_check_docker() {
    print_step "ステップ 1/5：Docker の確認"

    if ! command -v docker &>/dev/null; then
        print_error "Docker が検出されませんでした。Docker をインストールしてから再実行してください。"
        print_error "インストールガイド: https://docs.docker.com/engine/install/"
        exit 1
    fi

    print_success "Docker を検出しました: $(docker --version)"

    # Docker デーモンが実行中か確認
    if ! docker info &>/dev/null; then
        print_error "Docker デーモンが実行されていません。Docker を起動してから再試行してください。"
        if [[ "$(detect_os)" == "macos" ]]; then
            print_info "ヒント: 「Docker Desktop」アプリを開いてください。"
        elif [[ "$(detect_os)" == "windows" ]]; then
            print_info "ヒント: 「Docker Desktop」を起動するか、WSL2 バックエンドが実行されていることを確認してください。"
        else
            print_info "ヒント: sudo systemctl start docker を実行してください"
        fi
        exit 1
    fi

    print_success "Docker デーモンは正常に実行されています。"
}

# ============================================================
# ステップ 2：Whisper ASR Webservice のデプロイ
# ============================================================
step2_deploy_whisper() {
    print_step "ステップ 2/5：Whisper ASR Webservice のデプロイ"

    local os
    os="$(detect_os)"

    # --- CPU / GPU の選択 ---
    echo -e "${BOLD}実行モードを選択してください：${NC}"
    echo "  1) CPU モード（全プラットフォーム対応、macOS / GPU なしに推奨）"
    echo "  2) GPU モード（Linux + NVIDIA GPU のみ、より高速）"
    local mode_choice
    read -rp "$(echo -e "${YELLOW}番号を入力 [1]:${NC} ")" mode_choice
    mode_choice="${mode_choice:-1}"

    local image_tag=""
    local gpu_flag=""
    local asr_device="cpu"

    case "$mode_choice" in
        1)
            image_tag="latest"
            gpu_flag=""
            asr_device="cpu"
            print_info "CPU モードを選択しました。"
            ;;
        2)
            if [[ "$os" != "linux" ]]; then
                print_warn "GPU パススルーは macOS / Windows では利用できません（Docker はこれらのプラットフォームでは Linux VM で実行されます）。"
                if ! confirm "現在のシステムは Linux ではありません。GPU モードで続行しますか？" "n"; then
                    print_info "CPU モードに切り替えました。"
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
                print_info "GPU モードを選択しました。"
            fi
            ;;
        *)
            print_warn "無効な入力です。デフォルトで CPU モードを使用します。"
            image_tag="latest"
            asr_device="cpu"
            ;;
    esac

    # --- ASR エンジンの選択 ---
    echo ""
    echo -e "${BOLD}ASR エンジンを選択してください：${NC}"
    echo "  1) openai_whisper  - 公式 OpenAI Whisper（デフォルト）"
    echo "  2) faster_whisper   - CTranslate2 加速版、推論がより高速"
    echo "  3) whisperx         - 話者分離と字幕アライメント対応（HF_TOKEN 必要）"
    local engine_choice
    read -rp "$(echo -e "${YELLOW}番号を入力 [1]:${NC} ")" engine_choice
    engine_choice="${engine_choice:-1}"

    local asr_engine
    case "$engine_choice" in
        1) asr_engine="openai_whisper" ;;
        2) asr_engine="faster_whisper" ;;
        3) asr_engine="whisperx" ;;
        *) asr_engine="openai_whisper" ;;
    esac
    print_info "選択されたエンジン: ${asr_engine}"

    # --- モデルの選択 ---
    echo ""
    echo -e "${BOLD}字幕モデルを選択してください：${NC}"
    echo "  標準モデル（多言語）:"
    echo "    1) tiny            - 最速、精度最低 (~1GB)"
    echo "    2) base            - 高速、精度低め (~1GB) [デフォルト]"
    echo "    3) small           - バランス型 (~2GB)"
    echo "    4) medium          - 精度やや高め (~5GB)"
    echo "    5) large-v3        - 精度最高 (~10GB)"
    echo "    6) large-v3-turbo  - 高精度 + 高速 (~ turbo)"
    echo "  英語専用モデル:"
    echo "    7) base.en         - 英語 base"
    echo "    8) small.en        - 英語 small"
    echo "    9) medium.en       - 英語 medium"
    local model_choice
    read -rp "$(echo -e "${YELLOW}番号を入力 [2]:${NC} ")" model_choice
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
    print_info "選択されたモデル: ${asr_model}"

    # --- オプション：Hugging Face Token（whisperx で必要） ---
    local hf_token=""
    if [[ "$asr_engine" == "whisperx" ]]; then
        echo ""
        print_warn "whisperx エンジンには、話者分離モデルをダウンロードするための Hugging Face Token が必要です。"
        read -rp "$(echo -e "${YELLOW}HF_TOKEN を入力（空欄でスキップ、後で手動設定可能）:${NC} ")" hf_token
        if [[ -z "$hf_token" ]]; then
            print_warn "HF_TOKEN が入力されていません。whisperx が正常に動作しない可能性があります。"
            print_info "Token の取得: https://huggingface.co/settings/tokens"
        fi
    fi

    # --- オプション：モデルキャッシュの永続化 ---
    echo ""
    local cache_dir="${SCRIPT_DIR}/whisper-cache"
    if confirm "モデルキャッシュをローカルに永続化しますか？（後続の起動を高速化、推奨）" "y"; then
        mkdir -p "$cache_dir"
        print_info "キャッシュディレクトリ: ${cache_dir}"
    fi

    # --- 同名コンテナが存在する場合は先に削除 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$WHISPER_CONTAINER_NAME"; then
        print_warn "同名コンテナ ${WHISPER_CONTAINER_NAME} が存在します。旧コンテナを削除しています..."
        docker rm -f "$WHISPER_CONTAINER_NAME" &>/dev/null
    fi

    # --- イメージのプル ---
    local full_image="onerahmet/openai-whisper-asr-webservice:${image_tag}"
    print_info "イメージをプル中: ${full_image} ..."
    docker pull "$full_image"

    # --- 起動パラメータの構築 ---
    local run_args=(-d --name "$WHISPER_CONTAINER_NAME")
    [[ -n "$gpu_flag" ]] && run_args+=($gpu_flag)
    run_args+=(-p "${WHISPER_PORT}:9000")
    run_args+=(-e "ASR_MODEL=${asr_model}")
    run_args+=(-e "ASR_ENGINE=${asr_engine}")
    run_args+=(-e "ASR_DEVICE=${asr_device}")

    if [[ -n "$hf_token" ]]; then
        run_args+=(-e "HF_TOKEN=${hf_token}")
    fi

    # キャッシュの永続化
    if [[ -d "$cache_dir" ]]; then
        run_args+=(-v "${cache_dir}:/root/.cache")
    fi

    run_args+=("$full_image")

    # --- コンテナの起動 ---
    print_info "Whisper ASR コンテナを起動中..."
    docker run "${run_args[@]}"

    # サービスの準備完了を待機
    print_info "Whisper ASR サービスの準備を待っています（初回モデルダウンロードに時間がかかる場合があります）..."
    local max_wait=300
    local waited=0
    while ! curl -sf "http://localhost:${WHISPER_PORT}/docs" &>/dev/null; do
        sleep 5
        waited=$((waited + 5))
        if [[ $waited -ge $max_wait ]]; then
            print_warn "サービスが ${max_wait}s 内に完全に準備できませんでした。モデルのダウンロード中の可能性があります。"
            print_info "進捗は docker logs ${WHISPER_CONTAINER_NAME} で確認できます。"
            break
        fi
        echo -n "."
    done
    echo ""
    print_success "Whisper ASR Webservice が起動しました: http://localhost:${WHISPER_PORT}"
    print_info "Swagger ドキュメント: http://localhost:${WHISPER_PORT}/docs"
}

# ============================================================
# ステップ 3：ladaapp/lada:latest のプル
# ============================================================
step3_pull_lada() {
    print_step "ステップ 3/5：ladaapp/lada:latest のプル"

    local lada_image="ladaapp/lada:latest"
    print_info "イメージをプル中: ${lada_image} ..."
    docker pull "$lada_image"
    print_success "イメージ ${lada_image} のプルが完了しました。"

    # Lada コンテナを今すぐ起動するか確認
    if confirm "Lada コンテナを今すぐ起動しますか？" "y"; then
        if docker ps -a --format '{{.Names}}' | grep -qx "$LADA_CONTAINER_NAME"; then
            print_warn "同名コンテナ ${LADA_CONTAINER_NAME} が存在します。旧コンテナを削除しています..."
            docker rm -f "$LADA_CONTAINER_NAME" &>/dev/null
        fi
        docker run -d --name "$LADA_CONTAINER_NAME" -p "${LADA_PORT}:8080" "$lada_image"
        print_success "Lada コンテナが起動しました: http://localhost:${LADA_PORT}"
    else
        print_info "Lada コンテナの起動をスキップしました。後で手動で起動できます。"
    fi
}

# ============================================================
# ステップ 4：ffmpeg のインストール（Linux / Windows / macOS 対応）
# ============================================================
step4_install_ffmpeg() {
    print_step "ステップ 4/5：ffmpeg のインストール"

    # インストール済みか確認
    if command -v ffmpeg &>/dev/null; then
        local current_ver
        current_ver="$(ffmpeg -version 2>&1 | head -n1)"
        print_success "ffmpeg は既にインストールされています: ${current_ver}"
        if ! confirm "ffmpeg を再インストール/アップグレードしますか？" "n"; then
            print_info "ffmpeg のインストールをスキップします。"
            return 0
        fi
    fi

    local os
    os="$(detect_os)"
    print_info "検出された OS: ${os}"

    case "$os" in
        macos)
            if command -v brew &>/dev/null; then
                print_info "Homebrew で ffmpeg をインストール中 ..."
                brew install ffmpeg
            else
                print_warn "Homebrew が検出されませんでした。Homebrew の自動インストールを試みます ..."
                if confirm "Homebrew をインストールしますか？" "y"; then
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                    brew install ffmpeg
                else
                    print_error "ffmpeg を手動でインストールしてください: https://ffmpeg.org/download.html"
                    exit 1
                fi
            fi
            ;;

        linux)
            if command -v apt-get &>/dev/null; then
                print_info "apt-get で ffmpeg をインストール中 ..."
                sudo apt-get update && sudo apt-get install -y ffmpeg
            elif command -v dnf &>/dev/null; then
                print_info "dnf で ffmpeg をインストール中 ..."
                sudo dnf install -y ffmpeg
            elif command -v yum &>/dev/null; then
                print_info "yum で ffmpeg をインストール中 ..."
                sudo yum install -y ffmpeg
            elif command -v pacman &>/dev/null; then
                print_info "pacman で ffmpeg をインストール中 ..."
                sudo pacman -S --noconfirm ffmpeg
            elif command -v apk &>/dev/null; then
                print_info "apk で ffmpeg をインストール中 ..."
                sudo apk add --no-cache ffmpeg
            else
                print_error "サポートされているパッケージマネージャが検出されませんでした。ffmpeg を手動でインストールしてください: https://ffmpeg.org/download.html"
                exit 1
            fi
            ;;

        windows)
            # Windows では winget を優先、次に choco
            if command -v winget &>/dev/null; then
                print_info "winget で ffmpeg をインストール中 ..."
                winget install --id Gyan.FFmpeg -e --accept-package-agreements --accept-source-agreements
            elif command -v choco &>/dev/null; then
                print_info "Chocolatey で ffmpeg をインストール中 ..."
                choco install ffmpeg -y
            else
                print_warn "winget も Chocolatey も検出されませんでした。"
                print_info "以下のいずれかの方法で ffmpeg をインストールしてください："
                echo "  1. winget のインストール:  https://learn.microsoft.com/windows/package-manager/winget/"
                echo "  2. Chocolatey のインストール: https://chocolatey.org/install"
                echo "  3. 手動ダウンロード:       https://www.gyan.dev/ffmpeg/builds/"
                echo ""
                read -rp "$(echo -e "${YELLOW}ffmpeg のインストール完了後、Enter を押して続行...${NC}")"
            fi
            ;;

        *)
            print_error "サポートされていない OS です。ffmpeg を手動でインストールしてください: https://ffmpeg.org/download.html"
            exit 1
            ;;
    esac

    # インストールの検証
    if command -v ffmpeg &>/dev/null; then
        print_success "ffmpeg のインストールに成功しました: $(ffmpeg -version 2>&1 | head -n1)"
    else
        print_error "インストール後も ffmpeg が PATH に検出されません。環境変数を確認してください。"
        exit 1
    fi
}

# ============================================================
# ステップ 5：本プロジェクトのデプロイ
# ============================================================
step5_deploy_project() {
    print_step "ステップ 5/5：本プロジェクトのデプロイ"

    # --- config.yaml の存在確認 ---
    if [[ ! -f "$CONFIG_EXAMPLE" ]]; then
        print_error "設定テンプレートファイルが見つかりません: ${CONFIG_EXAMPLE}"
        exit 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_info "初回実行です。テンプレートから config.yaml を作成しています ..."
        cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"
    fi

    # --- 設定編集のガイド ---
    echo ""
    echo -e "${BOLD}設定ファイルを確認・完成させてください: ${CYAN}${CONFIG_FILE}${NC}"
    echo -e "${YELLOW}主要な設定項目：${NC}"
    echo "  whisper.model   - ステップ 2 で選択したモデルと一致させる必要があります"
    echo "  whisper.engine  - ステップ 2 で選択したエンジンと一致させる必要があります"
    echo "  whisper.host    - Whisper ASR サービスのアドレス（デフォルト http://localhost:9000）"
    echo "  io.input_dir    - 入力動画/音声ディレクトリ"
    echo "  io.output_dir   - 出力ディレクトリ"
    echo "  container.image - プロジェクトの Docker イメージ名"
    echo "  container.port  - コンテナのマッピングポート"
    echo ""

    if confirm "エディタで config.yaml を開いて編集しますか？" "y"; then
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
                print_warn "利用可能なエディタが見つかりません。手動で編集してください: ${CONFIG_FILE}"
                editor=""
            fi
        fi

        if [[ -n "$editor" ]]; then
            print_info "エディタを開いています: ${editor}"
            $editor "$CONFIG_FILE"
        fi
    fi

    # --- ユーザーの確認を待機 ---
    if ! confirm "config.yaml の設定は完了しましたか？デプロイを続行しますか？" "n"; then
        print_warn "設定を完了させてから再実行してください（完了済みのステップは自動的にスキップされます）。"
        print_info "または次のコマンドを実行: bash install.ja.sh"
        exit 0
    fi

    # --- config.yaml から主要設定を読み取り（簡易解析） ---
    local project_image project_name project_port input_dir output_dir
    project_image=$(grep -E '^\s*image:' "$CONFIG_FILE" | head -1 | sed 's/.*image:\s*//' | tr -d '"' || echo "video-captions:latest")
    project_name=$(grep -E '^\s*name:' "$CONFIG_FILE" | head -1 | sed 's/.*name:\s*//' | tr -d '"' || echo "video-captions")
    project_port=$(grep -E '^\s*port:' "$CONFIG_FILE" | tail -1 | sed 's/.*port:\s*//' | tr -d ' ' || echo "9001")
    input_dir=$(grep -E '^\s*input_dir:' "$CONFIG_FILE" | sed 's/.*input_dir:\s*//' | tr -d '"' || echo "./input")
    output_dir=$(grep -E '^\s*output_dir:' "$CONFIG_FILE" | sed 's/.*output_dir:\s*//' | tr -d '"' || echo "./output")

    # ディレクトリの存在を確保
    mkdir -p "$input_dir" "$output_dir"
    print_info "入力ディレクトリ: ${input_dir}"
    print_info "出力ディレクトリ: ${output_dir}"

    # --- イメージの存在確認 ---
    if ! docker image inspect "$project_image" &>/dev/null; then
        print_warn "ローカルにイメージが見つかりません: ${project_image}"
        if [[ -f "${SCRIPT_DIR}/Dockerfile" ]]; then
            if confirm "Dockerfile が見つかりました。イメージをビルドしますか？" "y"; then
                print_info "イメージをビルド中: ${project_image} ..."
                docker build -t "$project_image" "$SCRIPT_DIR"
            else
                print_error "イメージをビルドまたはプルしてください: ${project_image}"
                exit 1
            fi
        else
            print_error "イメージ ${project_image} が存在せず、Dockerfile も見つかりませんでした。"
            print_info "プロジェクトイメージをビルドしてから再実行するか、config.yaml の container.image を変更してください。"
            exit 1
        fi
    fi

    # --- 旧コンテナの削除 ---
    if docker ps -a --format '{{.Names}}' | grep -qx "$project_name"; then
        print_warn "同名コンテナ ${project_name} が存在します。旧コンテナを削除しています..."
        docker rm -f "$project_name" &>/dev/null
    fi

    # --- プロジェクトコンテナの起動 ---
    print_info "プロジェクトコンテナを起動中..."
    docker run -d \
        --name "$project_name" \
        -p "${project_port}:${project_port}" \
        -v "${CONFIG_FILE}:/app/config.yaml" \
        -v "${input_dir}:/app/input" \
        -v "${output_dir}:/app/output" \
        --link "${WHISPER_CONTAINER_NAME}:whisper" \
        --link "${LADA_CONTAINER_NAME}:lada" \
        "$project_image"

    print_success "プロジェクトコンテナが起動しました: ${project_name}"
    print_info "アクセスポート: http://localhost:${project_port}"
    echo ""
    echo -e "${GREEN}${BOLD}========== インストール完了！==========${NC}"
    echo -e "  Whisper ASR : http://localhost:${WHISPER_PORT}/docs"
    echo -e "  Lada        : http://localhost:${LADA_PORT}"
    echo -e "  プロジェクト  : http://localhost:${project_port}"
    echo -e "  設定ファイル  : ${CONFIG_FILE}"
    echo -e "  ログ確認     : docker logs -f ${project_name}"
    echo ""
}

# ============================================================
# メイン処理
# ============================================================
main() {
    echo -e "${GREEN}${BOLD}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║     VideoFlow ワンクリックインストールスクリプト       ║"
    echo "║     依存: Docker / Whisper ASR / Lada / ffmpeg ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"

    step1_check_docker
    step2_deploy_whisper
    step3_pull_lada
    step4_install_ffmpeg
    step5_deploy_project
}

main "$@"
