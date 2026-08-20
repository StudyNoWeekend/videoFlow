import request, { type ApiResponse } from './request'

// ---- 请求类型 ----

export interface InitReq {
  username: string
  password: string
  confirm_password: string
}

export interface LoginPwdReq {
  username: string
  password: string
}

export interface ChangePwdReq {
  old_password: string
  new_password: string
  confirm_password: string
}

export interface ResetPasswordReq {
  token: string
  new_password: string
  confirm_password: string
}

// ---- 响应类型 ----

export interface AuthStatusRes {
  initialized: boolean
}

export interface UserInfo {
  id: string
  username: string
}

export interface LoginRes {
  token: string
  user: UserInfo
}

/**
 * 查询系统初始化状态
 */
export function checkAuthStatus(): Promise<AuthStatusRes> {
  return request
    .get<ApiResponse<AuthStatusRes>>('/api/v1/auth/status')
    .then((res) => res.data.data)
}

/**
 * 系统初始化
 */
export function initSystem(data: InitReq): Promise<LoginRes> {
  return request
    .post<ApiResponse<LoginRes>>('/api/v1/auth/init', data)
    .then((res) => res.data.data)
}

/**
 * 密码登录
 */
export function loginByPassword(data: LoginPwdReq): Promise<LoginRes> {
  return request
    .post<ApiResponse<LoginRes>>('/api/v1/auth/login/password', data)
    .then((res) => res.data.data)
}

/**
 * 通过重置令牌重置密码
 */
export function resetPassword(data: ResetPasswordReq): Promise<void> {
  return request
    .post<ApiResponse<null>>('/api/v1/auth/reset-password', data)
    .then(() => undefined)
}

/**
 * 触发生成重置令牌（令牌只输出到服务器日志，不返回给前端）
 */
export function generateResetToken(username: string): Promise<void> {
  return request
    .post<ApiResponse<null>>('/api/v1/auth/reset-token', { username })
    .then(() => undefined)
}

/**
 * 修改密码（需登录）
 */
export function changePassword(data: ChangePwdReq): Promise<void> {
  return request
    .post<ApiResponse<null>>('/api/v1/auth/change-password', data)
    .then(() => undefined)
}
