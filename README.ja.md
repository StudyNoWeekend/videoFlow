<div align="center">

# VideoFlow

### Whisper + FFmpeg に Web ベースの可視化管理を -- 動画ディレクトリをスキャンし、字幕を自動生成、壊れた動画はワンクリックでモザイク除去。

Whisper ASR + FFmpeg + Docker モザイク除去エンジンをベースに、Vue 3 + Element Plus で構築した Web 可視化管理インターフェースです。スクリプトの組み合わせから解放され、スキャンでの取り込み、タスク作成、リアルタイムの進捗、オンライン設定まで、すべてブラウザ上で完結します。バックエンドは Go（Gin + GORM + SQLite）で、Docker によるワンクリックデプロイに対応しています。

[![Stars](https://img.shields.io/github/stars/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=yellow)](https://github.com/StudyNoWeekend/videoFlow/stargazers)
[![Forks](https://img.shields.io/github/forks/StudyNoWeekend/videoFlow?style=flat-square&logo=github&color=blue)](https://github.com/StudyNoWeekend/videoFlow/network/members)
[![Issues](https://img.shields.io/github/issues/StudyNoWeekend/videoFlow?style=flat-square&logo=github)](https://github.com/StudyNoWeekend/videoFlow/issues)
[![License](https://img.shields.io/github/license/StudyNoWeekend/videoFlow?style=flat-square&color=green)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?style=flat-square&logo=vue.js&logoColor=white)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white)](./Dockerfile)

[💡 プロジェクトの魅力](#-プロジェクトの魅力) · [🎯 こんな方におすすめ](#-こんな方におすすめ) · [✨ 機能特性](#-機能特性) · [📥 クイックスタート](#-クイックスタート) · [🎬 使い方](#-使い方) · [🌐 技術スタック](#-技術スタック) · [🔧 設定](#-設定) · [📊 API インターフェース](#-api-インターフェース) · [❓ FAQ](#-faq) · [🗺️ Roadmap](#️-roadmap)

🌐 [简体中文](./README.md) · [繁體中文](./README.zh-TW.md) · [English](./README.en.md) · **日本語**

</div>

---

## 💡 プロジェクトの魅力

本プロジェクトの最大の魅力は、**Whisper による音声認識、FFmpeg による音声処理、Docker によるモザイク除去、高画質化という、本来ならスクリプトを組み合わせてつなぎ合わせる必要のある 4 つの処理を、すぐに使える Web 可視化管理プラットフォームとして統合したこと**です。

Whisper や FFmpeg を単体で使うのは難しくありませんが、「ディレクトリをスキャン -> 音声を抽出 -> ASR を呼び出し -> 字幕を生成 -> タスクを並行実行 -> 進捗をトラッキング -> 失敗時にリトライ -> オンラインで設定を変更」という一連の流れを完全なパイプラインとしてつなげるのは、コマンドラインやスクリプトではなかなかスムーズにいきません。VideoFlow は、基盤となるエンジンを変更することなく、その上に Web UI の層をかぶせます：

| 比較 | 手動組み合わせ（Whisper + FFmpeg スクリプト） | VideoFlow（本プロジェクト） |
| --- | --- | --- |
| 操作方式 | コマンドライン引数 + シェルスクリプト | ブラウザのグラフィカル UI、マウスでポチポチ |
| 字幕生成 | 音声の手動抽出、ASR の呼び出し、字幕ファイルの組み立て | ワンクリックでタスク作成、自動で 音声抽出 -> ASR -> 字幕生成 |
| モザイク除去 | Docker コマンドを手動実行、ターミナルを見張る | ワンクリックでモザイク除去タスク、CPU / CUDA / MPS / XPU に対応 |
| 高画質化 | 自分でツールを探し、パラメータ調整が大変 | ワンクリックで解像度アップ、Real-ESRGAN / Real-CUGAN / libplacebo を選択可能 |
| タスク管理 | 自分で記録、ターミナルを閉じたら消える | タスクリスト + リアルタイム進捗 + 失敗リトライ + 履歴確認 |
| 並行制御 | キュー / セマフォを自前実装 | スケジューラが設定された並行数に従って自動スケジュール |
| 設定変更 | 設定ファイルを編集、サービスを再起動 | オンラインで変更、保存すると即座にホット反映、データベースに永続化 |
| デプロイ形態 | 多くの依存パッケージをインストール | Docker シングルイメージ、設定をマウントするだけで実行 |

基盤として [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) の認識能力、[FFmpeg](https://ffmpeg.org/) の音声/動画処理、[`ladaapp/lada`](https://github.com/ladaapp/lada) のモザイク除去を再利用し、その上に HTTP API と Web フロントエンドをラップしています -- **コマンドラインの能力を、グラフィカル UI の体験で**。

## 🎯 こんな方におすすめ

| あなたは | 何に使えるか |
| --- | --- |
| **動画クリエイター / 個人メディア** | 動画に一括で字幕を生成、手書き起こしの手間を省き、SRT / VTT をエクスポートしてそのまま利用 |
| **字幕チーム / 翻訳愛好家** | Whisper で一括文字起こししてタイムラインを生成し、その後人力で校正すれば効率が倍増 |
| **壊れた動画をお持ちの方** | 動画が開かない？ lada Docker でワンクリックモザイク除去、多種の計算デバイスに対応 |
| **NAS / ホームサーバー愛好家** | Docker で常時稼働させ、定期スキャンでディレクトリを自動取り込み、新規動画に自動で字幕を生成 |
| **Whisper をローカルで動かしたい方** | スクリプトを書かずに、Web UI で ASR パラメータ（言語 / VAD / プロンプト）を設定するだけ |
| **コマンドラインが面倒な方** | すべてグラフィカル UI で、設定、スキャン、タスク進捗がひと目で分かる |
| **ローカルユーザー** | インストール済みの ffmpeg を自動的にインテリジェントに呼び出し、追加設定は不要 |

## ✨ 機能特性

- **入力 / 出力ディレクトリの分離** - 入力ディレクトリをスキャンして自動取り込み。タスクの成果物（字幕 / 焼き込み動画 / モザイク除去 / 高画質化動画）は設定可能な出力ディレクトリに出力され、タスクの状態は動画レコードの状態フィールドを基準に、タスクのライフサイクルでリアルタイムに同期
- **字幕生成** - Whisper ASR ベース、言語、VAD フィルタ、タスクタイプ、音声の事前エンコード、初期プロンプト、単語レベルのタイムスタンプ、複数の出力形式（json / srt / vtt / txt / tsv）に対応
- **モザイク除去** - Docker イメージ `ladaapp/lada` ベース、x86_64 CPU および NVIDIA CUDA GPU（Turing シリーズ以降、RTX 20xx ～ RTX 50xx シリーズ）に対応。CUDA デバイスは自動でコンテナに透過（`--gpus`）、GPU 障害の原因を自動的にヒント表示
- **高画質化** - Video2X（`ghcr.io/k4yt3x/video2x:latest`）で動画をより高解像度にアップスケール、Real-ESRGAN / Real-CUGAN / libplacebo プロセッサに対応。ターゲット解像度とノイズ軽減レベルはタスクごとに指定
- **タスク管理** - 作成 / 照会 / 削除（複数選択での一括削除対応、出力ファイルも同時に削除するチェックボックスあり）/ 失敗リトライ、タイプ別フィルタ、リアルタイムのプログレスバー、フロントエンドは 2 秒間隔でポーリング更新。実行中のタスクを優先表示し、同じ状態内では新しいものが先頭
- **タスクスケジューラ** - バックグラウンドのスケジューラが設定された並行数制限に従って字幕 / モザイク除去 / 高画質化タスクを実行。ポーリング間隔はオンラインで設定可能。キャンセル時は残った半製品の出力ファイルを自動クリーンアップ
- **ユーザー認証** - 初回起動時の管理者アカウント初期化ガイド、ログイン / パスワード変更 / パスワードリセット。業務 API はすべて JWT 認証で保護
- **ランタイム設定** - 統合された設定ページ、すべての設定をオンラインで変更し永続化（SQLite）、保存すると即座にホット反映
- **FFmpeg スマート呼び出し** - ローカルの ffmpeg を自動検出し、手動設定不要
- **エンジニアリング** - `trace_id` による全リンクトラッキング、統一レスポンス構造、zap による構造化ログ、グレースフルシャットダウン（再起動時に実行中タスクを自動的に失敗扱い）

## 📥 クイックスタート

### 方法 1：Docker デプロイ（推奨）

イメージは GitHub Container Registry に公開されています：

```bash
docker pull ghcr.io/studynoweekend/videoflow:latest
```

1. 設定ファイルを用意します（テンプレートを元に変更）：

```bash
cp backend/config/config.yaml.local config.yaml
# 按需修改 config.yaml：asr.url、video.dir 等
```

2. コンテナを起動します（ポート、設定、データ、ログはすべて `docker run` 時に指定可能）：

```bash
docker run -d --name videoflow \
  --restart unless-stopped \
  -e APP_HTTP_PORT=8080 \
  -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  # 動画ディレクトリ（入力、読み取り専用マウント）と出力ディレクトリ（書き込み可）をマウントし、設定ページでコンテナ内パスを設定
  -v /path/to/videos:/videos:ro \
  -v /path/to/output:/output \
  -e APP_VIDEO_DIR=/videos \
  -e APP_OUTPUT_DIR=/output \
  -v "$PWD/logs:/app/logs" \
  ghcr.io/studynoweekend/videoflow:latest
```

3. ヘルスチェック：

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"},"code":0,"msg":"success","trace_id":"..."}
```

**ポートを変更する場合**：2 箇所を変更するだけで済み、ポートの情報源は一元化されています：

```bash
docker run -d --name videoflow \
  -e APP_HTTP_PORT=9090 -p 9090:9090 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  -v /path/to/videos:/videos:ro \
  -v /path/to/output:/output \
  -e APP_VIDEO_DIR=/videos \
  -e APP_OUTPUT_DIR=/output \
  ghcr.io/studynoweekend/videoflow:latest
```

<details>
<summary><b>方法 2：ローカルでイメージをビルド</b></summary>

```bash
docker build -t video-captions:latest -f Dockerfile .
docker run -d --name videoflow \
  -e APP_HTTP_PORT=8080 -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config/config.yaml:ro" \
  -v "$PWD/data:/app/data" \
  -v /path/to/videos:/videos:ro \
  -v /path/to/output:/output \
  -e APP_VIDEO_DIR=/videos \
  -e APP_OUTPUT_DIR=/output \
  video-captions:latest
```

Dockerfile はマルチステージビルドです：`golang:1.25-alpine` でコンパイル（CGO）し、`alpine:3.20` で実行（ffmpeg 内蔵）します。`linux/amd64` と `linux/arm64` の両方に対応しています。

</details>

### 方法 3：ローカル開発デプロイ

**前提条件：** Go 1.25+、Node.js 22.18+、FFmpeg、および [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) が稼働していること。

```bash
# 后端
cd backend
cp config/config.yaml.local config/config.yaml   # 按需修改
go run ./cmd/api

# 前端（另开终端）
cd frontend
npm install
npm run dev
```

開発時、フロントエンドは Vite プロキシ経由で `/api` をバックエンドに転送します。ポートは環境変数 `APP_HTTP_PORT` で一元管理されます（デフォルト 8080、バックエンドと共有）：

```bash
APP_HTTP_PORT=9090 npm run dev        # 前端
APP_HTTP_PORT=9090 go run ./cmd/api   # 后端
```

ブラウザで Vite が表示するローカルアドレスを開くだけです。

## 🎬 使い方

1. サービスを起動し、ブラウザでフロントエンドのアドレスにアクセス。初回は管理者アカウントを初期化してログイン
2. 「設定」ページでローカルの動画ディレクトリ（入力）、タスクの出力ディレクトリ、ASR サービスのアドレスを設定し、保存
3. 「動画リスト」ページでスキャンをクリックすると、動画が自動で取り込まれます
4. 動画カードの「字幕を生成」をクリックして、字幕タスクを作成
5. 「タスク管理」ページでリアルタイムの進捗を確認（2 秒ごとに自動更新）
6. （任意）「モザイク除去」で壊れた動画を修復、または「高画質化」で動画をより高解像度にアップスケール
7. 字幕タスクの完了後、生成された字幕ファイルを確認

## 🌐 技術スタック

| レイヤー | 技術 | 説明 |
| --- | --- | --- |
| Web フレームワーク | [Gin](https://github.com/gin-gonic/gin) | HTTP ルーティングとミドルウェア |
| ORM | [GORM](https://github.com/go-gorm/gorm) | データの永続化 |
| データベース | SQLite（[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)） | CGO ドライバ |
| 設定 | [Viper](https://github.com/spf13/viper) | 設定ファイル + 環境変数による上書き |
| ログ | [Zap](https://github.com/uber-go/zap) | 構造化ログ |
| ASR | [Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) | 音声認識エンジン |
| 音声/動画 | [FFmpeg](https://ffmpeg.org/) / ffprobe | 音声抽出、再生時間の検出、スマートローカル呼び出し |
| モザイク除去 | [`ladaapp/lada`](https://github.com/ladaapp/lada) | Docker モザイク除去エンジン |
| 高画質化 | [Video2X](https://github.com/k4yt3x/video2x) | Docker 高画質化エンジン |
| フロントエンド | [Vue 3](https://vuejs.org/) + [Element Plus](https://element-plus.org/) + [Vite](https://vite.dev/) | グラフィカル UI |
| 状態管理 | [Pinia](https://pinia.vuejs.org/) | フロントエンドの状態 |
| HTTP クライアント | [Axios](https://axios-http.com/) | API リクエスト |

## 🔧 設定

設定の優先順位：**環境変数 > config.yaml > コードのデフォルト値**。実行時にフロントエンドの「設定」ページからオンラインで変更し、データベースに永続化することも可能です。

<details>
<summary><b>設定ファイル（config/config.yaml）</b></summary>

```yaml
app:
  name: videoFlow
  env: dev
  port: 8080
http:
  port: 8080
  read_timeout: 30
  write_timeout: 30
log:
  level: info
  path: logs
database:
  driver: sqlite
  dsn: data/app.db
video:
  dir: ""
output:
  # タスクの出力ディレクトリ（任意。空の場合は元動画と同じディレクトリ内のサブディレクトリに出力）
  dir: ""
scan:
  interval: 60
asr:
  url: http://127.0.0.1:9999/asr
  language: zh
  vad_filter: false
  task: transcribe
  encode: true
  initial_prompt: ""
  word_timestamps: false
  output: json
repair:
  docker_image: ladaapp/lada:latest
  device: cpu            # cpu / cuda:0 / mps / xpu:0
concurrency:
  subtitle: 2
  repair: 1
```

</details>

<details>
<summary><b>環境変数（Docker デプロイ用）</b></summary>

viper のプレフィックスは `APP_`、設定キーの `.` は `_` に変換されるため、`http.port` は環境変数 `APP_HTTP_PORT` に対応します。以下同様：

| 環境変数 | 設定項目 | デフォルト値 |
| --- | --- | --- |
| `APP_HTTP_PORT` | `http.port` | `8080` |
| `APP_VIDEO_DIR` | `video.dir` | `""` |
| `APP_OUTPUT_DIR` | `output.dir` | `""` |
| `APP_SCAN_INTERVAL` | `scan.interval` | `60` |
| `APP_ASR_URL` | `asr.url` | `http://127.0.0.1:9999/asr` |
| `APP_ASR_LANGUAGE` | `asr.language` | `zh` |
| `APP_REPAIR_DEVICE` | `repair.device` | `cpu` |
| `APP_DATABASE_DSN` | `database.dsn` | `data/app.db` |

</details>

<details>
<summary><b>Docker ボリュームマウント</b></summary>

| コンテナのパス | 用途 |
| --- | --- |
| `/app/config/config.yaml` | 設定ファイル（読み取り専用マウント） |
| `/app/data` | SQLite データベースの永続化（`data/app.db`） |
| `/app/logs` | ログ出力 |
| `/<コンテナ内動画ディレクトリ>` | ホストの動画ディレクトリをコンテナ内にマウント（例 `/videos`、読み取り専用）。設定ページで `video.dir` にこのパスを指定 |
| `/<コンテナ内出力ディレクトリ>` | タスクの出力ディレクトリ（例 `/output`、**書き込み可必須**）。設定ページで `output.dir` にこのパスを指定 |
| `/var/run/docker.sock` | （任意）モザイク除去 / 高画質化にはホストマシンの Docker socket をマウントする必要があります |

> **出力ディレクトリについて：** タスクの成果物（srt / 焼き込み動画 / モザイク除去 / 高画質化動画）は、設定ページの `output.dir` に指定したコンテナ内パス（例 `/output`）に出力されます。このマウントは書き込み可能である必要があります。入力ディレクトリは読み取り専用マウントのため、成果物を入力ディレクトリに書き込むことはできません。動画が入力ディレクトリのサブフォルダにある場合は、その相対構造をミラーリングします（例 `output/<サブフォルダ>/<動画名>/`）。

</details>

## 📊 API インターフェース

すべての API は共通プレフィックス `/api/v1` を持ち、レスポンス構造は以下の通りです：

```json
{
  "code": 0,
  "msg": "success",
  "data": {},
  "trace_id": "xxx"
}
```

`code=0` は成功、0 以外は業務エラーを示します。

> **認証について**：ヘルスチェック、認証 API、`/api/v1/version`、コンポーネントインストール進捗の SSE API を除き、その他の業務 API はログイン時に発行された Token をリクエストヘッダーに付ける必要があります（`Authorization: Bearer <token>`）。

<details>
<summary><b>認証 API</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `GET` | `/api/v1/auth/status` | 初期化 / ログイン状態の確認 |
| `POST` | `/api/v1/auth/init` | 初回実行時に管理者アカウントを初期化 |
| `POST` | `/api/v1/auth/login/password` | ユーザー名とパスワードでログイン、Token を返却 |
| `POST` | `/api/v1/auth/reset-token` | パスワードリセット用トークンを生成 |
| `POST` | `/api/v1/auth/reset-password` | トークンでパスワードをリセット |
| `POST` | `/api/v1/auth/change-password` | パスワード変更（ログインが必要） |

</details>

<details>
<summary><b>ヘルスチェック</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `GET` | `/health` | ヘルスチェック |
| `GET` | `/ready` | レディチェック |
| `GET` | `/api/v1/version` | 現在のバージョン |

</details>

<details>
<summary><b>動画 API</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `POST` | `/api/v1/videos/scan` | 動画ディレクトリをスキャンして取り込み |
| `GET` | `/api/v1/videos` | 動画リストのページング照会 |
| `POST` | `/api/v1/videos/batch-delete` | 動画レコードの一括削除（body に `ids`、任意で `delete_files` により出力ディレクトリも削除） |
| `PUT` | `/api/v1/videos/:id` | 動画情報の更新 |
| `DELETE` | `/api/v1/videos/:id` | 動画レコードの削除 |

</details>

<details>
<summary><b>タスク API</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | タスク作成（字幕 / 字幕焼き込み / モザイク除去 / 高画質化） |
| `GET` | `/api/v1/tasks` | タスクリストのページング照会（タイプでフィルタ可能、実行中タスクを優先表示） |
| `POST` | `/api/v1/tasks/batch-delete` | タスクレコードの一括削除（body に `ids`、任意で `delete_files` により出力ファイルも削除） |
| `POST` | `/api/v1/tasks/:id/retry` | 失敗タスクのリトライ |
| `DELETE` | `/api/v1/tasks/:id` | タスクの削除（任意で `?delete_files=true` により出力ファイルも削除） |

タスクステータス：`pending`（保留中）-> `running`（実行中）-> `completed`（完了）/ `failed`（失敗）/ `cancelled`（キャンセル済み）

> **タスク状態は動画レコードが基準**：動画ページのタスク状態列は、動画レコード内のタスク状態フィールド（`subtitle_status` / `subtitle_burn_status` / `deblur_status` / `upscale_status`）を直接読み取り、タスクのライフサイクルでリアルタイムに同期します（作成→保留中、取得→実行中、完了→完了、失敗→失敗、キャンセル→キャンセル、タスク削除→ロールバック/クリア）。タスクレコード削除時は、対応する出力ファイルも同時に削除するか選択できます。

> **高画質化タスクのパラメータ**：作成時にターゲット解像度（`target_width` / `target_height`）を指定します。プロセッサ（`upscale_processor`：`realesrgan` / `realcugan` / `libplacebo`）、モデル（`upscale_model`）、ノイズ軽減レベル（`upscale_noise_level`、-1 ～ 3）は必要に応じて設定可能です。

</details>

<details>
<summary><b>コンポーネント API</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `GET` | `/api/v1/components` | 各コンポーネントの状態を確認（Docker / FFmpeg / Whisper ASR / lada / Video2X） |
| `GET` | `/api/v1/components/install/progress/:session_id` | コンポーネントのインストール進捗（SSE リアルタイム配信、公開 API） |

</details>

<details>
<summary><b>ランタイム設定 API</b></summary>

| メソッド | パス | 説明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | ランタイム設定の取得 |
| `PUT` | `/api/v1/settings` | ランタイム設定の更新（保存すると即座にホット反映） |

</details>

## 📁 プロジェクト構成

```
videoFlow/
├── backend/
│   ├── cmd/api/              # プログラムエントリ main.go
│   ├── bootstrap/            # 設定、DB、ASR、FFmpeg、モザイク除去 などの初期化
│   ├── config/               # 設定ファイル（config.yaml.local がテンプレート）
│   ├── internal/
│   │   ├── controller/       # HTTP コントローラ
│   │   ├── logic/            # ビジネスロジック
│   │   ├── model/            # データモデルと永続化（GORM）
│   │   ├── dto/              # リクエスト/レスポンス DTO
│   │   ├── router/           # ルーティング登録
│   │   ├── asr/              # ASR クライアント
│   │   ├── ffmpeg/           # FFmpeg ローカル実行器
│   │   ├── repair/           # モザイク除去実行器
│   │   ├── upscale/          # 高画質化実行器
│   │   ├── subtitle/         # 字幕パース
│   │   ├── scanner/          # 動画ディレクトリスキャナ
│   │   └── scheduler/        # タスクスケジューラ
│   ├── enum/                 # 業務エラーコード
│   └── utils/                # ログ、レスポンスラッパー
├── frontend/
│   ├── src/
│   │   ├── api/              # API リクエストラッパー
│   │   ├── views/            # ページ（動画/タスク/設定）
│   │   ├── stores/           # Pinia 状態
│   │   ├── router/           # ルーティング
│   │   └── utils/            # ユーティリティ関数
│   └── vite.config.ts        # /api プロキシ設定を含む
├── Dockerfile                # マルチステージビルド
└── LICENSE
```

## 🛡️ 注意事項

- **FFmpeg は必須**：`ffmpeg.provider` は `local` 固定、イメージに ffmpeg が内蔵されています。ローカルでソースから実行する場合は各自でインストールしてください
- **タスク出力には書き込み可能なディレクトリが必要**：Docker デプロイ時は書き込み可能な出力ディレクトリ（例 `/output`）を必ずマウントし、`output.dir` を設定してください。未設定の場合は元動画の隣のサブディレクトリに出力され、入力ディレクトリが読み取り専用マウントだと書き込みに失敗します
- **モザイク除去 / 高画質化には Docker が必要**：コンテナデプロイ時にホストマシンの Docker socket をマウントする必要があります。この機能を使わない場合は無視でき、サービスの起動には影響しません
- **初回使用時は初期化が必要**：ブラウザでの初回アクセス時に管理者アカウントの初期化が案内され、ログイン後に業務機能を利用できます
- **ASR サービスは各自で用意**：[Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice) をご自身でデプロイし、`asr.url` を設定してください
- **データベースの永続化**：デフォルトは `data/app.db`、Docker デプロイ時は必ず `/app/data` ディレクトリをマウントしてください。そうしないと再起動でデータが失われます

## ❓ FAQ

<details>
<summary><b>起動時に <code>ffmpeg not found</code> と出る場合は？</b></summary>

`ffmpeg.provider` は `local` 固定で、ローカル（またはイメージ内）に ffmpeg が存在する必要があります。Docker イメージにはすでに内蔵されています。ソースから実行する場合は ffmpeg をインストールしてください。

</details>

<details>
<summary><b>モザイク除去機能が使えない？</b></summary>

モザイク除去は Docker に依存します。コンテナデプロイ時にホストマシンの Docker socket をマウントしてください：`-v /var/run/docker.sock:/var/run/docker.sock`。この機能を使わない場合は無視でき、サービスの起動には影響しません。

</details>

<details>
<summary><b>バックエンドのポートを変更するには？</b></summary>

環境変数 `APP_HTTP_PORT` が `http.port` を上書きします（バックエンドの viper とフロントエンドの Vite プロキシでこの変数を共有）。`docker run -e APP_HTTP_PORT=9090 -p 9090:9090` とするだけで OK です。

</details>

<details>
<summary><b>データベースはどこに？</b></summary>

デフォルトは `data/app.db`（SQLite）です。Docker デプロイ時は `/app/data` ディレクトリをマウントして永続化してください。

</details>

## 🗺️ Roadmap

**✅ 実装済み**

- 動画スキャン（手動 / 定期自動）+ 字幕生成（Whisper ASR）
- モザイク除去（lada）+ 高画質化（Video2X）
- 字幕ファイルの動画への書き込み（焼き込み）
- タスク管理（作成 / キャンセル / 失敗リトライ / リアルタイム進捗）
- タスクスケジューラ（並行数制御 / ポーリング間隔の設定可能）
- ランタイム設定のオンライン変更 + 永続化（SQLite）
- コンポーネント管理（検出とインストール：Docker / FFmpeg / Whisper ASR / lada / Video2X）
- ユーザー認証（ログイン / 初期化 / パスワード変更）
- Docker デプロイ + 実行時のポート指定 + マルチアーキテクチャイメージ（amd64 + arm64 ネイティブサポート）
- FFmpeg スマートローカル呼び出し

## 💌 謝辞

本プロジェクトは巨人の肩の上に成り立っています。以下のオープンソースプロジェクトとその作者に特別な感謝を申し上げます：

- **[Whisper ASR Webservice](https://github.com/ahmetoner/whisper-asr-webservice)** - by [@ahmetoner](https://github.com/ahmetoner)、OpenAI Whisper ベースの ASR HTTP サービス。VideoFlow の音声認識能力はすべてこれに基づいています。
- **[FFmpeg](https://ffmpeg.org/)** - 強力な音声/動画処理ツール。音声の抽出と再生時間の検出を担います。
- **[ladaapp/lada](https://github.com/ladaapp/lada)** - モザイク除去 Docker イメージ。
- **[Video2X](https://github.com/k4yt3x/video2x)** - 動画高画質化 Docker イメージ。
- **[Gin](https://github.com/gin-gonic/gin)** / **[GORM](https://github.com/go-gorm/gorm)** / **[Viper](https://github.com/spf13/viper)** / **[Zap](https://github.com/uber-go/zap)** - 優秀な Go 基盤ライブラリ。
- **[Vue 3](https://vuejs.org/)** / **[Element Plus](https://element-plus.org/)** / **[Vite](https://vite.dev/)** - フロントエンドの基盤。

## ⭐ Star history

[![Stargazers over time](https://api.star-history.com/svg?repos=StudyNoWeekend/videoFlow&type=Date)](https://star-history.com/#StudyNoWeekend/videoFlow&Date)

## 📜 ライセンス

[MIT License](./LICENSE) - 自由に使用、改変、配布できます。著作権表示を残すだけで構いません。

---

<div align="center">

**使ってみて役に立ったと思ったら、⭐ をいただけると作者にとって最大の励みになります。**

[⬆ トップへ戻る](#videoflow) · [📥 クイックスタート](#-クイックスタート) · [💬 Issue を報告](https://github.com/StudyNoWeekend/videoFlow/issues)

</div>
