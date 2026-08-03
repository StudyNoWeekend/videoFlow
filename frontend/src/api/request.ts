import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import { i18n } from '@/main'

const t = i18n.global.t

// 统一响应结构
export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
  trace_id: string
}

// 创建 axios 实例：开发态 baseURL 留空走相对路径，经 Vite 代理转发到后端；
// 生产态可由 .env.production 的 VITE_API_BASE_URL 显式指定
const request: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 响应拦截器：统一处理业务错误
request.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const res = response.data
    if (res.code !== 0) {
      ElMessage.error(res.msg || t('common.request_failed'))
      return Promise.reject(new Error(res.msg || t('common.request_failed')))
    }
    return response
  },
  (error) => {
    const msg = error.response?.data?.msg || error.message || t('common.network_error')
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export default request
