import request, { type ApiResponse } from './request'
import type { Video } from './video'

// 任务类型
export type TaskType = 'subtitle' | 'subtitle_burn' | 'deblur' | 'translate'

// 任务状态
export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelling' | 'cancelled'

// 任务信息
export interface Task {
  id: string
  video_id: string
  task_type: TaskType
  status: TaskStatus
  source_path?: string
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
}

/**
 * 创建任务
 * @param videoId 视频 ID
 * @param taskType 任务类型
 * @param sourcePath 可选：实际处理源文件路径，为空时默认使用关联视频
 */
export function createTask(videoId: string, taskType: TaskType, sourcePath?: string): Promise<Task> {
  return request
    .post<ApiResponse<Task>>('/api/v1/tasks', {
      video_id: videoId,
      task_type: taskType,
      ...(sourcePath ? { source_path: sourcePath } : {}),
    })
    .then((res) => res.data.data)
}

/**
 * 分页查询任务列表
 * @param page 页码
 * @param pageSize 每页数量
 * @param type 任务类型过滤
 */
export function listTasks(page: number, pageSize: number, type?: TaskType): Promise<TaskListRes> {
  return request
    .get<ApiResponse<TaskListRes>>('/api/v1/tasks', {
      params: { page, page_size: pageSize, task_type: type },
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
 */
export function deleteTask(id: string): Promise<void> {
  return request.delete<ApiResponse<void>>(`/api/v1/tasks/${id}`).then((res) => res.data.data)
}
