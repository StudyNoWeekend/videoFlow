import request, { type ApiResponse } from './request'

// 组件状态
export type ComponentStatus = 'installed' | 'missing' | 'installing' | 'error'
export type ComponentType = 'docker' | 'ffmpeg' | 'whisper_asr' | 'lada' | 'video2x' | 'yt-dlp'

export interface ComponentInfo {
  type: ComponentType
  name: string
  status: ComponentStatus
  version: string
  error_msg?: string
  description: string
  needs_docker: boolean
}

export interface ComponentInstallReq {
  component_type: 'lada' | 'ffmpeg' | 'video2x'
}

export interface ComponentUninstallReq {
  component_type: 'lada' | 'ffmpeg' | 'video2x'
}

export interface ProgressEvent {
  session_id: string
  step: string
  progress: number
  log: string
  status: string
  error?: string
}

/**
 * 获取组件列表
 */
export function getComponents(): Promise<ComponentInfo[]> {
  return request
    .get<ApiResponse<{ components: ComponentInfo[] }>>('/api/v1/components')
    .then((res) => res.data.data.components)
}

/**
 * 安装组件
 */
export function installComponent(data: ComponentInstallReq): Promise<{ session_id: string }> {
  return request
    .post<ApiResponse<{ session_id: string }>>('/api/v1/components/install', data)
    .then((res) => res.data.data)
}

/**
 * 重装组件
 */
export function reinstallComponent(data: ComponentInstallReq): Promise<{ session_id: string }> {
  return request
    .post<ApiResponse<{ session_id: string }>>('/api/v1/components/reinstall', data)
    .then((res) => res.data.data)
}

/**
 * 卸载组件
 */
export function uninstallComponent(data: ComponentUninstallReq): Promise<{ session_id: string }> {
  return request
    .post<ApiResponse<{ session_id: string }>>('/api/v1/components/uninstall', data)
    .then((res) => res.data.data)
}

/**
 * 获取指定组件的活跃安装 session
 */
export function getActiveSession(componentType: string): Promise<string> {
  return request
    .get<ApiResponse<{ session_id: string }>>(`/api/v1/components/active-session/${componentType}`)
    .then((res) => res.data.data.session_id)
}

/**
 * 获取指定组件的历史安装事件（用于页面刷新后回放日志）
 */
export function getInstallHistory(componentType: string): Promise<ProgressEvent[]> {
  return request
    .get<ApiResponse<{ events: ProgressEvent[] }>>(`/api/v1/components/install/history/${componentType}`)
    .then((res) => res.data.data.events)
}
