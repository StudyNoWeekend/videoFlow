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
