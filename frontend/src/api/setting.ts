import request, { type ApiResponse } from './request'

// 统一设置响应/请求对象
export interface Setting {
  video_dir: string
  output_dir: string
  scan_interval: number
  asr_url: string
  asr_language: string
  asr_vad_filter: boolean
  asr_task: 'transcribe' | 'translate'
  asr_encode: boolean
  asr_initial_prompt: string
  asr_word_timestamps: boolean
  asr_output: 'txt' | 'vtt' | 'srt' | 'tsv' | 'json'
  repair_docker_image: string
  // 去马赛克计算设备，支持四种：cpu（CPU）、cuda:0（NVIDIA CUDA）、mps（Apple Silicon MPS）、xpu:0（Intel XPU）
  repair_device: 'cpu' | 'cuda:0' | 'mps' | 'xpu:0'
  subtitle_concurrency: number
  subtitle_burn_concurrency: number
  repair_concurrency: number
  scheduler_poll_interval: number
  upscale_docker_image: string
  upscale_device: 'cpu' | 'cuda:0' | 'mps' | 'xpu:0'
  upscale_concurrency: number
  // Telegram 下载配置：app_id/app_hash 需到 my.telegram.org 申请，留空表示停用
  telegram_app_id: string
  telegram_app_hash: string
  telegram_threads: number
  telegram_data_dir: string
  // 全局出站代理：Telegram 与 yt-dlp 共用，支持 socks5/socks5h/http/https
  proxy_url: string
  // yt-dlp 是否使用全局代理（代理为全局模式时可关掉，避免国内站点绕远）
  proxy_for_ytdlp: boolean
}

/**
 * 获取统一设置
 */
export function getSettings(): Promise<Setting> {
  return request
    .get<ApiResponse<Setting>>('/api/v1/settings')
    .then((res) => res.data.data)
}

/**
 * 更新统一设置
 * @param data 设置对象
 */
export function updateSettings(data: Setting): Promise<Setting> {
  return request
    .put<ApiResponse<Setting>>('/api/v1/settings', data)
    .then((res) => res.data.data)
}
