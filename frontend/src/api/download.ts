import request, { type ApiResponse } from './request'

// 下载任务状态
export type DownloadStatus = 'pending' | 'probing' | 'downloading' | 'completed' | 'failed' | 'cancelled'

// 下载任务信息
export interface Download {
  id: string
  url: string
  status: DownloadStatus
  progress: number
  progress_msg?: string
  error_msg?: string
  file_name?: string
  file_size?: number
  duration?: number
  title?: string
  download_speed?: number
  total_size?: number
  downloaded_size?: number
  overwrite: boolean
  download_dir?: string
  created_at: number
  updated_at: number
}

// 分页响应
export interface DownloadListRes {
  list: Download[]
  total: number
  page: number
  page_size: number
}

/**
 * 创建下载任务
 * @param url 视频链接
 * @param overwrite 文件冲突时是否覆盖（false=自动重命名）
 * @param downloadDir 下载存放目录，为空则使用本地视频目录
 */
export function createDownload(url: string, overwrite = false, downloadDir?: string): Promise<Download> {
  return request
    .post<ApiResponse<Download>>('/api/v1/downloads', { url, overwrite, download_dir: downloadDir || undefined })
    .then((res) => res.data.data)
}

/**
 * 分页查询下载任务列表
 * @param page 页码
 * @param pageSize 每页数量
 * @param sortBy 排序字段，目前支持 'created_at'、'file_size'；为空时按创建时间倒序
 * @param order 排序方向：'asc' 正序 / 'desc' 倒序
 */
export function listDownloads(page: number, pageSize: number, sortBy?: string, order?: 'asc' | 'desc'): Promise<DownloadListRes> {
  return request
    .get<ApiResponse<DownloadListRes>>('/api/v1/downloads', {
      params: { page, page_size: pageSize, sort_by: sortBy, order },
    })
    .then((res) => res.data.data)
}

/**
 * 取消进行中的下载任务
 * @param id 下载任务 ID
 */
export function cancelDownload(id: string): Promise<Download> {
  return request
    .post<ApiResponse<Download>>(`/api/v1/downloads/${id}/cancel`)
    .then((res) => res.data.data)
}

/**
 * 删除下载记录
 * @param id 下载任务 ID
 * @param deleteFile 是否同时删除本地文件
 */
export function deleteDownload(id: string, deleteFile = false): Promise<void> {
  return request
    .delete<ApiResponse<void>>(`/api/v1/downloads/${id}`, { params: { delete_file: deleteFile ? '1' : '0' } })
    .then((res) => res.data.data)
}
