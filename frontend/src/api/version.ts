import request, { type ApiResponse } from './request'

/**
 * 获取后端版本号
 */
export function getVersion(): Promise<string> {
  return request
    .get<ApiResponse<{ version: string }>>('/api/v1/version')
    .then((res) => res.data.data.version)
}
