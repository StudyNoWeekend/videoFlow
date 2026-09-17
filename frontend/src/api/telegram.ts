import request, { type ApiResponse } from './request'

// Telegram 扫码登录状态
export type TelegramLoginStatus = '' | 'pending' | 'need_2fa' | 'success' | 'error'

// Telegram 连接与登录状态
export interface TelegramStatus {
  configured: boolean
  ready: boolean
  authenticated: boolean
  account?: string
  login_status: TelegramLoginStatus
  qr_url?: string
  error?: string
}

// 登录二维码
export interface TelegramQR {
  qr_url: string
  qr_image: string
}

/** 查询 Telegram 连接与登录状态 */
export function getTelegramStatus(): Promise<TelegramStatus> {
  return request.get<ApiResponse<TelegramStatus>>('/api/v1/telegram/status').then((res) => res.data.data)
}

/** 发起扫码登录，已在登录流程中时直接返回当前状态 */
export function startTelegramQRLogin(): Promise<TelegramStatus> {
  return request.post<ApiResponse<TelegramStatus>>('/api/v1/telegram/login/qr').then((res) => res.data.data)
}

/** 获取当前登录二维码（服务端渲染的 PNG data URI） */
export function getTelegramQRCode(): Promise<TelegramQR> {
  return request.get<ApiResponse<TelegramQR>>('/api/v1/telegram/login/qr').then((res) => res.data.data)
}

/** 提交两步验证密码 */
export function submitTelegram2FA(password: string): Promise<void> {
  return request.post<ApiResponse<void>>('/api/v1/telegram/login/2fa', { password }).then((res) => res.data.data)
}

/** 退出登录并清理本地会话 */
export function logoutTelegram(): Promise<void> {
  return request.post<ApiResponse<void>>('/api/v1/telegram/logout').then((res) => res.data.data)
}
