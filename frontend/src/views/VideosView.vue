<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { listVideos, scanVideos, deleteVideo, batchDeleteVideos, listDirFiles } from '@/api/video'
import { createTask, TASK_REQUIRED_COMPONENTS } from '@/api/task'
import type { TaskType } from '@/api/task'
import type { Video, TaskSnapshot, DirFile } from '@/api/video'
import SourceSelectDialog, { type SourceSelectOption } from '@/components/SourceSelectDialog.vue'
import UpscaleDialog from '@/components/UpscaleDialog.vue'
import { useSettingsStore } from '@/stores/settings'
import { useComponentStore } from '@/stores/component'
import { formatFileSize } from '@/utils/format'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const componentStore = useComponentStore()

const scanPath = ref<string>('')
const loading = ref<boolean>(false)
const scanning = ref<boolean>(false)
const videoList = ref<Video[]>([])
const page = ref<number>(1)
const pageSize = ref<number>(12)
const total = ref<number>(0)
// 大小列排序状态（服务端分页排序，空串表示默认按更新时间倒序）
const sortBy = ref<string>('')
const sortOrder = ref<'asc' | 'desc' | ''>('')
let pollTimer: number | null = null

// 多选批量删除
const selectedVideos = ref<Video[]>([])

// 选择处理源弹窗状态（有同名衍生视频时让用户选择对哪条视频执行任务）
const sourceDialogVisible = ref<boolean>(false)
const sourceDialogTitle = ref<string>('')
const sourceDialogVideo = ref<Video | null>(null)
const sourceDialogOptions = ref<SourceSelectOption[]>([])
const sourceDialogTaskType = ref<string>('deblur')

// 放大弹窗状态
const upscaleDialogVisible = ref<boolean>(false)
const upscaleDialogVideo = ref<Video | null>(null)
const upscaleDialogFiles = ref<Array<{name: string, path: string, width: number, height: number, size: number, fileType: string}>>([])

// 轮询间隔选项（毫秒），0 表示不轮询
const pollingOptions = [
  { value: 5000, label: 'videos.polling.5s' },
  { value: 10000, label: 'videos.polling.10s' },
  { value: 60000, label: 'videos.polling.1m' },
  { value: 1800000, label: 'videos.polling.30m' },
  { value: 0, label: 'videos.polling.off' },
]
const pollingInterval = ref<number>(0)

async function loadVideos(): Promise<void> {
  loading.value = true
  try {
    const res = await listVideos(page.value, pageSize.value, sortBy.value || undefined, sortOrder.value || undefined)
    videoList.value = res.list
    total.value = res.total
    page.value = res.page
    pageSize.value = res.page_size
  } finally {
    loading.value = false
  }
}

// 轮询时只更新进度条、状态、错误信息，保持现有数据
async function pollVideos(): Promise<void> {
  try {
    const res = await listVideos(page.value, pageSize.value, sortBy.value || undefined, sortOrder.value || undefined)
    for (const incoming of res.list) {
      const existing = videoList.value.find((v) => v.id === incoming.id)
      if (existing) {
        // 更新字幕任务快照
        if (incoming.subtitle_task) {
          if (existing.subtitle_task) {
            existing.subtitle_task.status = incoming.subtitle_task.status
            existing.subtitle_task.progress = incoming.subtitle_task.progress
            existing.subtitle_task.error_msg = incoming.subtitle_task.error_msg
            existing.subtitle_task.updated_at = incoming.subtitle_task.updated_at
          } else {
            existing.subtitle_task = incoming.subtitle_task
          }
        }
        // 更新字幕写入任务快照
        if (incoming.subtitle_burn_task) {
          if (existing.subtitle_burn_task) {
            existing.subtitle_burn_task.status = incoming.subtitle_burn_task.status
            existing.subtitle_burn_task.progress = incoming.subtitle_burn_task.progress
            existing.subtitle_burn_task.error_msg = incoming.subtitle_burn_task.error_msg
            existing.subtitle_burn_task.updated_at = incoming.subtitle_burn_task.updated_at
          } else {
            existing.subtitle_burn_task = incoming.subtitle_burn_task
          }
        }
        // 更新去马赛克任务快照
        if (incoming.deblur_task) {
          if (existing.deblur_task) {
            existing.deblur_task.status = incoming.deblur_task.status
            existing.deblur_task.progress = incoming.deblur_task.progress
            existing.deblur_task.error_msg = incoming.deblur_task.error_msg
            existing.deblur_task.updated_at = incoming.deblur_task.updated_at
          } else {
            existing.deblur_task = incoming.deblur_task
          }
        }
        // 更新放大任务快照
        if (incoming.upscale_task) {
          if (existing.upscale_task) {
            existing.upscale_task.status = incoming.upscale_task.status
            existing.upscale_task.progress = incoming.upscale_task.progress
            existing.upscale_task.error_msg = incoming.upscale_task.error_msg
            existing.upscale_task.updated_at = incoming.upscale_task.updated_at
          } else {
            existing.upscale_task = incoming.upscale_task
          }
        }
      }
    }
    total.value = res.total
  } catch {
    // 静默处理轮询错误
  }
}

async function handleScan(): Promise<void> {
  const path = scanPath.value.trim() || settingsStore.setting.video_dir
  if (!path) {
    ElMessage.warning(t('videos.scan.empty_path'))
    return
  }
  scanning.value = true
  try {
    const res = await scanVideos(path)
    ElMessage.success(t('videos.scan.success', { count: res.scanned }))
    await loadVideos()
  } finally {
    scanning.value = false
  }
}

async function handleDelete(video: Video): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('videos.delete.confirm', { name: video.name }),
      t('common.notice'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    await deleteVideo(video.id)
    ElMessage.success(t('videos.delete.success'))
    await loadVideos()
  } catch (error) {
    if (error !== 'cancel') {
      // 非取消操作，已在拦截器中提示
    }
  }
}

function handleSelectionChange(rows: Video[]): void {
  selectedVideos.value = rows
}

async function handleBatchDelete(): Promise<void> {
  const ids = selectedVideos.value.map((v) => v.id)
  if (ids.length === 0) return

  // 在确认弹窗中提供“同时删除输出文件”勾选项，由用户决定删除范围
  const checkboxId = 'video-batch-delete-files'
  let confirmed = false
  try {
    await ElMessageBox.confirm(
      `<p style="margin: 0 0 12px;">${t('videos.batch_delete.confirm', { count: ids.length })}</p>
       <label style="display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 13px;">
         <input id="${checkboxId}" type="checkbox" style="accent-color: var(--vf-accent, #409eff);" />
         ${t('videos.batch_delete.delete_files_label')}
       </label>`,
      t('common.notice'),
      { dangerouslyUseHTMLString: true, confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    confirmed = true
  } catch (error) {
    confirmed = false
    if (error !== 'cancel') {
      // 非取消操作，已在拦截器中提示
    }
  }
  if (!confirmed) return

  const deleteFiles = (document.getElementById(checkboxId) as HTMLInputElement | null)?.checked ?? false
  try {
    const res = await batchDeleteVideos(ids, deleteFiles)
    const msg = deleteFiles
      ? t('videos.batch_delete.success_with_files', { deleted: res.deleted, skipped: res.skipped })
      : t('videos.batch_delete.success', { deleted: res.deleted, skipped: res.skipped })
    ElMessage.success(msg)
    selectedVideos.value = []
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

function isTaskRunning(task?: TaskSnapshot): boolean {
  return !!task && (task.status === 'pending' || task.status === 'running')
}

/**
 * 创建任务前预检该任务类型依赖的组件是否就绪。
 * store 中的组件状态可能过期，首次判为缺失时重新检测一次再定型，仍缺失则提示并返回 false。
 * 后端创建接口仍会做最终校验，此处仅用于尽早引导用户。
 */
async function ensureTaskComponents(taskType: TaskType): Promise<boolean> {
  if (componentStore.components.length === 0) {
    await componentStore.loadComponents()
  }
  const judge = (): string[] => {
    const missing: string[] = []
    for (const ct of TASK_REQUIRED_COMPONENTS[taskType] ?? []) {
      const comp = componentStore.getStatus(ct)
      if (comp?.status !== 'installed') {
        missing.push(comp?.name ?? ct)
      }
    }
    return missing
  }

  let missingNames = judge()
  if (missingNames.length > 0) {
    // 缓存可能过期，重新检测一次避免误拦
    await componentStore.loadComponents()
    missingNames = judge()
    if (missingNames.length > 0) {
      ElMessage.warning(t('videos.component_missing', { list: missingNames.join('、') }))
      return false
    }
  }
  return true
}

async function handleSubtitle(video: Video): Promise<void> {
  if (isTaskRunning(video.subtitle_task)) return
  if (!(await ensureTaskComponents('subtitle'))) return
  try {
    await createTask(video.id, 'subtitle')
    ElMessage.success(t('videos.subtitle.success'))
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

// 点击非字幕任务时实时扫描输出目录中的衍生视频（含历史/旧版本产物），
// 用作处理源选择；不依赖视频列表缓存快照，扫描失败按“无衍生视频”处理
async function loadDerivedVideos(video: Video): Promise<DirFile[]> {
  try {
    const files = await listDirFiles(video.id)
    return files.filter((f) => f.file_type !== 'original' && f.path)
  } catch {
    return []
  }
}

async function handleSubtitleBurn(video: Video): Promise<void> {
  if (isTaskRunning(video.subtitle_burn_task)) return
  if (!(await ensureTaskComponents('subtitle_burn'))) return

  // 实时扫描输出目录中的衍生视频（如去马赛克/清晰度修复视频）；没有则直接对原视频执行
  const derived = await loadDerivedVideos(video)
  if (derived.length === 0) {
    try {
      await createTask(video.id, 'subtitle_burn')
      ElMessage.success(t('videos.subtitle_burn.success'))
      await loadVideos()
    } catch {
      // 请求失败已由拦截器提示
    }
    return
  }

  // 有衍生视频：弹窗让用户选择处理源（原视频 / 衍生视频）
  sourceDialogVideo.value = video
  sourceDialogTitle.value = t('videos.dialog.subtitle_burn_title')
  sourceDialogTaskType.value = 'subtitle_burn'
  sourceDialogOptions.value = derived.map((f) => ({
    name: f.name,
    path: f.path,
    size: f.size,
    labelKey: getFileTypeLabel(f),
    tag: getFileTypeTag(f),
  }))
  sourceDialogVisible.value = true
}

async function handleDeblur(video: Video): Promise<void> {
  if (isTaskRunning(video.deblur_task)) return
  if (!(await ensureTaskComponents('deblur'))) return
  const successKey = 'videos.deblur.success'
  const titleKey = 'videos.dialog.deblur_title'

  // 实时扫描输出目录中的衍生视频（如字幕合成视频）；没有则直接对原视频执行
  const derived = await loadDerivedVideos(video)
  if (derived.length === 0) {
    try {
      await createTask(video.id, 'deblur')
      ElMessage.success(t(successKey))
      await loadVideos()
    } catch {
      // 请求失败已由拦截器提示
    }
    return
  }

  // 有衍生视频：弹窗让用户选择处理源（原视频 / 衍生视频）
  sourceDialogVideo.value = video
  sourceDialogTitle.value = t(titleKey)
  sourceDialogTaskType.value = 'deblur'
  sourceDialogOptions.value = derived.map((f) => ({
    name: f.name,
    path: f.path,
    size: f.size,
    labelKey: getFileTypeLabel(f),
    tag: getFileTypeTag(f),
  }))
  sourceDialogVisible.value = true
}

async function handleUpscale(video: Video): Promise<void> {
  if (isTaskRunning(video.upscale_task)) return
  if (!(await ensureTaskComponents('upscale'))) return
  try {
    const files = await listDirFiles(video.id)
    upscaleDialogVideo.value = video
    // 接口返回 file_type（snake_case），转换为弹窗所需的 fileType 字段
    upscaleDialogFiles.value = files.map((f) => ({
      name: f.name,
      path: f.path,
      width: f.width,
      height: f.height,
      size: f.size,
      fileType: f.file_type,
    }))
    upscaleDialogVisible.value = true
  } catch {
    // error handled by interceptor
  }
}

// 弹窗确认：以用户选择的视频为处理源创建任务（path 为空串表示原视频）
// overwrite 表示是否覆盖所选的衍生视频（仅衍生视频时勾选出现）
async function handleSourceConfirm(payload: { path: string; overwrite: boolean }): Promise<void> {
  const video = sourceDialogVideo.value
  if (!video) return
  const taskType = sourceDialogTaskType.value
  if (!(await ensureTaskComponents(taskType as TaskType))) return
  sourceDialogVideo.value = null
  try {
    await createTask(video.id, taskType as TaskType, payload.path || undefined, payload.overwrite)
    if (taskType === 'deblur') {
      ElMessage.success(t('videos.deblur.success'))
    } else {
      ElMessage.success(t('videos.subtitle_burn.success'))
    }
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

async function handleUpscaleConfirm(payload: { sourcePath: string; targetWidth: number; targetHeight: number; processor: string; model: string; noiseLevel: number; overwrite: boolean }): Promise<void> {
  const video = upscaleDialogVideo.value
  if (!video) return
  upscaleDialogVideo.value = null
  try {
    await createTask(video.id, 'upscale', payload.sourcePath, payload.overwrite, payload.targetWidth, payload.targetHeight, payload.processor, payload.model, payload.noiseLevel)
    ElMessage.success(t('videos.upscale.success'))
    await loadVideos()
  } catch {
    // handled by interceptor
  }
}

function taskStatusLed(task?: TaskSnapshot): string {
  if (!task) return ''
  if (task.status === 'completed') return 'vf-led--green'
  if (task.status === 'failed') return 'vf-led--red'
  if (task.status === 'running' || task.status === 'pending') return 'vf-led--amber vf-led--pulse'
  return ''
}

function taskStatusText(task?: TaskSnapshot): string {
  if (!task) return '--'
  return t('tasks.status.' + task.status)
}

// 输出文件类型标签配置
	const fileTypeLabels: Record<string, { label: string; tag: string }> = {
		  subtitle:        { label: 'videos.file.subtitle',        tag: 'primary' },
		  subtitled_video: { label: 'videos.file.subtitled_video', tag: 'warning' },
		  repaired_video:  { label: 'videos.file.repaired_video',  tag: 'danger' },
		  upscaled_video:  { label: 'videos.file.upscaled_video',  tag: 'success' },
		  unknown:         { label: 'videos.file.unknown',         tag: 'info' },
		}

function getFileTypeLabel(file: { file_type: string }): string {
  return fileTypeLabels[file.file_type]?.label ?? 'videos.file.unknown'
}

function getFileTypeTag(file: { file_type: string }): string {
  return fileTypeLabels[file.file_type]?.tag ?? 'info'
}

function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadVideos()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  page.value = 1
  loadVideos()
}

// 大小列排序：点击表头上下 icon 切换正序/倒序，order 为 null 时恢复默认排序
function handleSortChange({ prop, order }: { prop: string; order: string | null }): void {
  sortBy.value = prop === 'size' ? 'size' : ''
  sortOrder.value = order === 'ascending' ? 'asc' : order === 'descending' ? 'desc' : ''
  page.value = 1
  loadVideos()
}

function handlePollingChange(value: number): void {
  pollingInterval.value = value
  startPolling()
}

function startPolling(): void {
  stopPolling()
  if (pollingInterval.value <= 0) return
  pollTimer = window.setInterval(() => {
    pollVideos()
  }, pollingInterval.value)
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  settingsStore.init()
  componentStore.loadComponents()
  loadVideos()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="videos-view">
    <div class="vf-panel">
      <div class="vf-panel__footer"></div>

      <!-- 面板标题 / 扫描控制 -->
      <div class="vf-panel-header">
        <div class="vf-panel-header__title">
          <span class="vf-led vf-led--amber vf-led--pulse"></span>
          <span>{{ $t('videos.title') }}</span>
          <span class="header__count">{{ $t('videos.count', { count: total }) }}</span>
        </div>

        <div class="scan-control">
          <el-input
            v-model="scanPath"
            :placeholder="$t('videos.scan.placeholder')"
            clearable
            style="width: 380px"
            @keyup.enter="handleScan"
          />
          <el-button type="primary" :loading="scanning" @click="handleScan">
            <el-icon><Search /></el-icon>{{ $t('videos.scan') }}
          </el-button>
          <el-button @click="loadVideos">
            <el-icon><Refresh /></el-icon>{{ $t('videos.refresh') }}
          </el-button>
        </div>
      </div>

      <div class="panel-toolbar">
        <div class="toolbar-left">
          <el-button
            type="danger"
            size="small"
            :disabled="selectedVideos.length === 0"
            @click="handleBatchDelete"
          >
            <el-icon><Delete /></el-icon>{{ $t('videos.batch_delete') }}<span v-if="selectedVideos.length" class="batch-count">{{ selectedVideos.length }}</span>
          </el-button>
        </div>
        <div class="toolbar-right">
          <div class="poll-control">
            <span class="vf-data-label">{{ $t('videos.polling.label') }}</span>
            <el-select v-model="pollingInterval" size="small" style="width: 110px" @change="handlePollingChange">
              <el-option
                v-for="opt in pollingOptions"
                :key="opt.value"
                :label="$t(opt.label)"
                :value="opt.value"
              />
            </el-select>
            <span v-if="pollingInterval > 0" class="vf-led vf-led--green vf-led--pulse"></span>
          </div>
        </div>
      </div>

      <div class="panel-body panel-body--compact">
        <el-table
          v-loading="loading"
          :data="videoList"
          style="width: 100%"
          empty-text=""
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="45" />

          <el-table-column :label="$t('videos.column.name')" min-width="280">
            <template #default="{ row }">
              <div class="name-cell">
                <span class="name-icon">
                  <el-icon><VideoCamera /></el-icon>
                </span>
                <span class="name-text" :title="row.path">{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.type')" width="100">
            <template #default>
              <el-tag type="primary" size="small">{{ $t('videos.type.video') }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column
            :label="$t('videos.column.size')"
            prop="size"
            sortable="custom"
            :sort-orders="['ascending', 'descending']"
            width="110"
            @sort-change="handleSortChange"
          >
            <template #default="{ row }">
              <span class="size-value">{{ formatFileSize(row.size) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.subtitle')" width="100">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', taskStatusLed(row.subtitle_task)]"></span>
                <span class="status-text">{{ taskStatusText(row.subtitle_task) }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.subtitle_burn')" width="100">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', taskStatusLed(row.subtitle_burn_task)]"></span>
                <span class="status-text">{{ taskStatusText(row.subtitle_burn_task) }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.deblur')" width="100">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', taskStatusLed(row.deblur_task)]"></span>
                <span class="status-text">{{ taskStatusText(row.deblur_task) }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.upscale')" width="100">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', taskStatusLed(row.upscale_task)]"></span>
                <span class="status-text">{{ taskStatusText(row.upscale_task) }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.action')" width="350" fixed="right">
            <template #default="{ row }">
              <el-button
                type="primary"
                size="small"
                :disabled="isTaskRunning(row.subtitle_task)"
                :loading="row.subtitle_task?.status === 'running'"
                @click="handleSubtitle(row)"
              >
                {{ $t('videos.btn.subtitle') }}
              </el-button>
              <el-button
                type="primary"
                size="small"
                :disabled="isTaskRunning(row.subtitle_burn_task)"
                :loading="row.subtitle_burn_task?.status === 'running'"
                @click="handleSubtitleBurn(row)"
              >
                {{ $t('videos.btn.subtitle_burn') }}
              </el-button>
              <el-button
                type="warning"
                size="small"
                :disabled="isTaskRunning(row.deblur_task)"
                :loading="row.deblur_task?.status === 'running'"
                @click="handleDeblur(row)"
              >
                {{ $t('videos.btn.deblur') }}
              </el-button>
              <el-button
                type="success"
                size="small"
                :disabled="isTaskRunning(row.upscale_task)"
                :loading="row.upscale_task?.status === 'running'"
                @click="handleUpscale(row)"
              >
                {{ $t('videos.btn.upscale') }}
              </el-button>
              <el-button type="danger" size="small" @click="handleDelete(row)">{{ $t('videos.btn.delete') }}</el-button>
            </template>
          </el-table-column>

          <template #empty>
            <el-empty v-if="!loading" :description="$t('videos.empty')" />
          </template>
        </el-table>
      </div>

      <div class="panel-footer">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[12, 24, 48, 96]"
          layout="total, sizes, prev, pager, next"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </div>

    <!-- 选择处理源弹窗：存在同名衍生视频时，让用户选择对哪条视频执行任务 -->
    <SourceSelectDialog
      v-model="sourceDialogVisible"
      :title="sourceDialogTitle"
      :tipKey="sourceDialogTaskType === 'subtitle_burn' ? 'videos.source.subtitle_burn_tip' : 'videos.source.tip'"
      :video-name="sourceDialogVideo?.name || ''"
      :options="sourceDialogOptions"
      @confirm="handleSourceConfirm"
    />

    <UpscaleDialog
      v-model="upscaleDialogVisible"
      :video-name="upscaleDialogVideo?.name || ''"
      :files="upscaleDialogFiles"
      @confirm="handleUpscaleConfirm"
    />
  </div>
</template>

<style scoped>
.videos-view {
  padding: 20px;
  min-height: 100%;
}

.vf-panel {
  min-height: calc(100vh - 92px);
  display: flex;
  flex-direction: column;
}

.header__count {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  border: 1px solid var(--vf-border);
  padding: 2px 8px;
  border-radius: var(--vf-radius-sm);
  margin-left: 8px;
}

.scan-control {
  display: flex;
  align-items: center;
  gap: 10px;
}

.panel-toolbar {
  padding: 8px 16px;
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 14px;
}

.batch-count {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  margin-left: 4px;
  opacity: 0.85;
}

.poll-control {
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-body--compact {
  padding: 0;
}

.panel-body--compact :deep(.el-table__header-wrapper) {
  border-bottom: 1px solid var(--vf-border);
}

/* ========== 表内单元格样式 ========== */

.name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.name-icon {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  color: var(--vf-text-muted);
  font-size: 15px;
}

.name-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--vf-font-display);
  font-size: 13px;
}

.size-value {
  font-family: var(--vf-font-mono);
  font-size: 12px;
  color: var(--vf-text-secondary);
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}

.status-text {
  font-size: 11px;
  color: var(--vf-text-muted);
  white-space: nowrap;
}

.signal-progress {
  margin-top: 1px;
}

.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}
</style>
