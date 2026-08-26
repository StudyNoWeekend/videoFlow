import request, { type ApiResponse } from './request'
import type { ComponentType } from './component'
import type { Video } from './video'

// 任务类型
export type TaskType = 'subtitle' | 'subtitle_burn' | 'deblur' | 'upscale' | 'download'

// 创建任务所需的组件（与后端 component.TaskTypeDependencies 保持一致）
export const TASK_REQUIRED_COMPONENTS: Record<TaskType, ComponentType[]> = {
  subtitle: ['ffmpeg', 'whisper_asr'],
  subtitle_burn: ['ffmpeg'],
  deblur: ['docker', 'lada'],
  upscale: ['docker', 'video2x'],
  download: ['yt-dlp'],
}

// 任务状态
export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelling' | 'cancelled'

// 任务信息
export interface Task {
  id: string
  video_id: string
  task_type: TaskType
  status: TaskStatus
  source_path?: string
  /** 是否覆盖处理源文件（仅选择了衍生视频时有效） */
  overwrite?: boolean
  progress: number
  progress_msg?: string
  result?: unknown
  error_msg?: string
  retry_count: number
  created_at: number
  updated_at: number
  video?: Video
}

// 分页响应
export interface TaskListRes {
  list: Task[]
  total: number
  page: number
  page_size: number
}

// 创建任务请求
export interface TaskCreateReq {
  video_id: string
  task_type: TaskType
  /** 可选：实际处理源文件路径，为空时默认使用关联视频 */
  source_path?: string
  /** 可选：是否覆盖处理源文件（仅选择了衍生视频时有效） */
  overwrite?: boolean
  /** 可选：放大任务的输出宽度（目标分辨率） */
  target_width?: number
  /** 可选：放大任务的输出高度（目标分辨率） */
  target_height?: number
  /** 可选：清晰度修复处理器类型（仅 upscale 任务） */
  upscale_processor?: string
  /** 可选：清晰度修复模型/着色器名称（仅 upscale 任务） */
  upscale_model?: string
  /** 可选：降噪等级（仅 upscale 任务，-1=无/保守，0-3 递增） */
  upscale_noise_level?: number
}

/**
 * 创建任务
 * @param videoId 视频 ID
 * @param taskType 任务类型
 * @param sourcePath 可选：实际处理源文件路径，为空时默认使用关联视频
 * @param overwrite 可选：是否覆盖处理源文件（仅选择了衍生视频时有效，true 时结果直接替换所选视频）
 * @param targetWidth 可选：放大目标宽度
 * @param targetHeight 可选：放大目标高度
 * @param upscaleProcessor 可选：清晰度修复处理器（仅 upscale）
 * @param upscaleModel 可选：清晰度修复模型/着色器（仅 upscale）
 * @param upscaleNoiseLevel 可选：降噪等级（仅 upscale，-1=无/保守，0-3 递增）
 */
export function createTask(
  videoId: string,
  taskType: TaskType,
  sourcePath?: string,
  overwrite?: boolean,
  targetWidth?: number,
  targetHeight?: number,
  upscaleProcessor?: string,
  upscaleModel?: string,
  upscaleNoiseLevel?: number,
): Promise<Task> {
  const body: Record<string, any> = {
    video_id: videoId,
    task_type: taskType,
  }
  if (sourcePath) body.source_path = sourcePath
  if (overwrite !== undefined) body.overwrite = overwrite
  if (targetWidth !== undefined) body.target_width = targetWidth
  if (targetHeight !== undefined) body.target_height = targetHeight
  if (upscaleProcessor !== undefined) body.upscale_processor = upscaleProcessor
  if (upscaleModel !== undefined) body.upscale_model = upscaleModel
  if (upscaleNoiseLevel !== undefined) body.upscale_noise_level = upscaleNoiseLevel
  return request
    .post<ApiResponse<Task>>('/api/v1/tasks', body)
    .then((res) => res.data.data)
}

/**
 * 分页查询任务列表
 * @param page 页码
 * @param pageSize 每页数量
 * @param type 任务类型过滤
 * @param sortBy 排序字段，目前支持 'created_at'、'updated_at'；为空时按运行中优先+创建时间倒序
 * @param order 排序方向：'asc' 正序 / 'desc' 倒序
 */
export function listTasks(page: number, pageSize: number, type?: TaskType, sortBy?: string, order?: 'asc' | 'desc'): Promise<TaskListRes> {
  return request
    .get<ApiResponse<TaskListRes>>('/api/v1/tasks', {
      params: { page, page_size: pageSize, task_type: type, sort_by: sortBy, order },
    })
    .then((res) => res.data.data)
}

/**
 * 重试失败任务
 * @param id 任务 ID
 */
export function retryTask(id: string): Promise<Task> {
  return request
    .post<ApiResponse<Task>>(`/api/v1/tasks/${id}/retry`)
    .then((res) => res.data.data)
}

/**
 * 取消任务：等待中直接取消，运行中会中断正在执行的逻辑
 * @param id 任务 ID
 */
export function cancelTask(id: string): Promise<Task> {
  return request
    .post<ApiResponse<Task>>(`/api/v1/tasks/${id}/cancel`)
    .then((res) => res.data.data)
}

/**
 * 删除任务
 * @param id 任务 ID
 * @param deleteFiles 是否同时删除任务对应的输出文件（srt/烧录/去马赛克/清晰度修复产物）
 */
export function deleteTask(id: string, deleteFiles = false): Promise<void> {
  return request
    .delete<ApiResponse<void>>(`/api/v1/tasks/${id}`, { params: { delete_files: deleteFiles } })
    .then((res) => res.data.data)
}

// 批量删除结果
export interface BatchDeleteRes {
  deleted: number
  skipped: number
}

/**
 * 批量删除任务记录，运行中的任务会被跳过
 * @param ids 任务 ID 列表
 * @param deleteFiles 是否同时删除任务对应的输出文件
 */
export function batchDeleteTasks(ids: string[], deleteFiles = false): Promise<BatchDeleteRes> {
  return request
    .post<ApiResponse<BatchDeleteRes>>('/api/v1/tasks/batch-delete', { ids, delete_files: deleteFiles })
    .then((res) => res.data.data)
}
