import request, { type ApiResponse } from './request'

// 任务状态快照（视频列表仅展示状态徽标，具体进度/错误详见任务页）
export interface TaskSnapshot {
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelling' | 'cancelled'
  progress?: number
  error_msg?: string
  updated_at?: number
}

// 视频信息
export interface Video {
  id: string
  path: string
  name: string
  duration: number
  size: number
  width: number
  height: number
  created_at: number
  updated_at: number
  subtitle_task?: TaskSnapshot
  subtitle_burn_task?: TaskSnapshot
  deblur_task?: TaskSnapshot
  upscale_task?: TaskSnapshot
  output_dir?: string
  output_files?: OutputFile[]
}

// 输出目录中的文件信息
export interface OutputFile {
  name: string
  path?: string
  size: number
  is_video: boolean
  file_type: 'subtitle' | 'subtitled_video' | 'repaired_video' | 'upscaled_video' | 'unknown'
  updated_at: number
}

// 分页响应
export interface VideoListRes {
  list: Video[]
  total: number
  page: number
  page_size: number
}

// 扫描请求
export interface ScanReq {
  path?: string
}

// 扫描响应
export interface ScanRes {
  scanned: number
}

// 视频更新请求
export interface VideoUpdateReq {
  name: string
}

/**
 * 扫描本地视频目录
 * @param path 本地目录绝对路径，为空时使用配置的视频目录
 */
export function scanVideos(path?: string): Promise<ScanRes> {
  return request
    .post<ApiResponse<ScanRes>>('/api/v1/videos/scan', { path })
    .then((res) => res.data.data)
}

/**
 * 分页查询视频列表
 * @param page 页码
 * @param pageSize 每页数量
 */
export function listVideos(page: number, pageSize: number): Promise<VideoListRes> {
  return request
    .get<ApiResponse<VideoListRes>>('/api/v1/videos', {
      params: { page, page_size: pageSize },
    })
    .then((res) => res.data.data)
}

/**
 * 更新视频信息
 * @param id 视频 ID
 * @param data 更新内容
 */
export function updateVideo(id: string, data: VideoUpdateReq): Promise<Video> {
  return request
    .put<ApiResponse<Video>>(`/api/v1/videos/${id}`, data)
    .then((res) => res.data.data)
}

/**
 * 删除视频记录
 * @param id 视频 ID
 */
export function deleteVideo(id: string): Promise<void> {
  return request.delete<ApiResponse<void>>(`/api/v1/videos/${id}`).then((res) => res.data.data)
}

// 批量删除结果
export interface BatchDeleteRes {
  deleted: number
  skipped: number
}

/**
 * 批量删除视频记录
 * @param ids 视频 ID 列表
 * @param deleteFiles 是否同时删除视频对应的输出目录（srt、烧录/去马赛克/清晰度修复产物）
 */
export function batchDeleteVideos(ids: string[], deleteFiles: boolean): Promise<BatchDeleteRes> {
  return request
    .post<ApiResponse<BatchDeleteRes>>('/api/v1/videos/batch-delete', { ids, delete_files: deleteFiles })
    .then((res) => res.data.data)
}

// 视频目录中的文件信息
export interface DirFile {
  name: string
  path: string
  size: number
  width: number
  height: number
  file_type: string
  updated_at: number
}

/**
 * 查询视频目录中的文件列表
 * @param videoId 视频 ID
 */
export function listDirFiles(videoId: string): Promise<DirFile[]> {
  return request
    .get<ApiResponse<DirFile[]>>(`/api/v1/videos/${videoId}/dir-files`)
    .then((res) => res.data.data)
}
