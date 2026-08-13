<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { listVideos, scanVideos, deleteVideo } from '@/api/video'
import { createTask } from '@/api/task'
import type { Video, TaskSnapshot, OutputFile } from '@/api/video'
import SourceSelectDialog, { type SourceSelectOption } from '@/components/SourceSelectDialog.vue'
import { useSettingsStore } from '@/stores/settings'
import { formatDuration, formatFileSize } from '@/utils/format'

const { t } = useI18n()
const settingsStore = useSettingsStore()

const scanPath = ref<string>('')
const loading = ref<boolean>(false)
const scanning = ref<boolean>(false)
const videoList = ref<Video[]>([])
const page = ref<number>(1)
const pageSize = ref<number>(12)
const total = ref<number>(0)
let pollTimer: number | null = null

// 选择处理源弹窗状态（有同名衍生视频时让用户选择对哪条视频执行任务）
const sourceDialogVisible = ref<boolean>(false)
const sourceDialogTitle = ref<string>('')
const sourceDialogVideo = ref<Video | null>(null)
const sourceDialogOptions = ref<SourceSelectOption[]>([])

// 轮询间隔选项（毫秒），0 表示不轮询
const pollingOptions = [
  { value: 5000, label: 'videos.polling.5s' },
  { value: 10000, label: 'videos.polling.10s' },
  { value: 60000, label: 'videos.polling.1m' },
  { value: 1800000, label: 'videos.polling.30m' },
  { value: 0, label: 'videos.polling.off' },
]
const pollingInterval = ref<number>(0)

// 树形表格展开行 key 集合
const expandedRows = ref<string[]>([])
const defaultExpandAll = ref<boolean>(true)

// 输出文件行唯一 key 前缀
const OUTPUT_ROW_PREFIX = '__output__'

function getRowKey(row: Video | OutputFile): string {
  if ('id' in row && row.id) {
    return row.id
  }
  return OUTPUT_ROW_PREFIX + (row as OutputFile).name
}

// 判断是否为输出文件子行
function isOutputRow(row: Video | OutputFile): boolean {
  return !('id' in row) || !row.id
}

async function loadVideos(): Promise<void> {
  loading.value = true
  try {
    const res = await listVideos(page.value, pageSize.value)
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
    const res = await listVideos(page.value, pageSize.value)
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

function isTaskRunning(task?: TaskSnapshot): boolean {
  return !!task && (task.status === 'pending' || task.status === 'running')
}

async function handleSubtitle(video: Video): Promise<void> {
  if (isTaskRunning(video.subtitle_task)) return
  try {
    await createTask(video.id, 'subtitle')
    ElMessage.success(t('videos.subtitle.success'))
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

async function handleSubtitleBurn(video: Video): Promise<void> {
  if (isTaskRunning(video.subtitle_burn_task)) return
  try {
    await createTask(video.id, 'subtitle_burn')
    ElMessage.success(t('videos.subtitle_burn.success'))
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

async function handleDeblur(video: Video): Promise<void> {
  if (isTaskRunning(video.deblur_task)) return
  const successKey = 'videos.deblur.success'
  const titleKey = 'videos.dialog.deblur_title'

  // 检测是否存在同名衍生视频（如字幕合成视频）；没有则直接对原视频执行
  const derived = (video.output_files || []).filter((f) => f.is_video && f.path)
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
  sourceDialogOptions.value = derived.map((f) => ({
    name: f.name,
    path: f.path!,
    size: f.size,
    labelKey: getFileTypeLabel(f),
    tag: getFileTypeTag(f),
  }))
  sourceDialogVisible.value = true
}

// 弹窗确认：以用户选择的视频为处理源创建任务（path 为空串表示原视频）
async function handleSourceConfirm(path: string): Promise<void> {
  const video = sourceDialogVideo.value
  if (!video) return
  sourceDialogVideo.value = null
  try {
    await createTask(video.id, 'deblur', path || undefined)
    ElMessage.success(t('videos.deblur.success'))
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
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
  translated:      { label: 'videos.file.translated',      tag: 'success' },
  subtitled_video: { label: 'videos.file.subtitled_video', tag: 'warning' },
  repaired_video:  { label: 'videos.file.repaired_video',  tag: 'danger' },
  unknown:         { label: 'videos.file.unknown',         tag: 'info' },
}

function getFileTypeLabel(file: OutputFile): string {
  return fileTypeLabels[file.file_type]?.label ?? 'videos.file.unknown'
}

function getFileTypeTag(file: OutputFile): string {
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
          :row-key="getRowKey"
          :tree-props="{ children: 'output_files' }"
          :default-expand-all="defaultExpandAll"
          style="width: 100%"
          empty-text=""
        >
          <el-table-column :label="$t('videos.column.name')" min-width="280">
            <template #default="{ row }">
              <div class="name-cell" :class="{ 'name-cell--child': isOutputRow(row) }">
                <span class="name-icon">
                  <el-icon v-if="isOutputRow(row)"><Document /></el-icon>
                  <el-icon v-else><VideoCamera /></el-icon>
                </span>
                <span class="name-text" :title="row.name">{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.type')" width="100">
            <template #default="{ row }">
              <template v-if="!isOutputRow(row)">
                <el-tag type="primary" size="small">{{ $t('videos.type.video') }}</el-tag>
              </template>
              <template v-else>
                <el-tag :type="getFileTypeTag(row as OutputFile)" size="small">
                  {{ $t(getFileTypeLabel(row as OutputFile)) }}
                </el-tag>
              </template>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.size')" width="110">
            <template #default="{ row }">
              <span class="size-value">{{ formatFileSize(row.size) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.subtitle')" width="100">
            <template #default="{ row }">
              <template v-if="!isOutputRow(row)">
                <div class="status-cell">
                  <span :class="['vf-led', taskStatusLed((row as Video).subtitle_task)]"></span>
                  <span class="status-text">{{ taskStatusText((row as Video).subtitle_task) }}</span>
                </div>
                <el-progress
                  v-if="(row as Video).subtitle_task"
                  :percentage="(row as Video).subtitle_task!.progress"
                  :status="(row as Video).subtitle_task!.status === 'failed' ? 'exception' : (row as Video).subtitle_task!.status === 'completed' ? 'success' : ''"
                  :stroke-width="4"
                  class="signal-progress"
                />
              </template>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.subtitle_burn')" width="100">
            <template #default="{ row }">
              <template v-if="!isOutputRow(row)">
                <div class="status-cell">
                  <span :class="['vf-led', taskStatusLed((row as Video).subtitle_burn_task)]"></span>
                  <span class="status-text">{{ taskStatusText((row as Video).subtitle_burn_task) }}</span>
                </div>
                <el-progress
                  v-if="(row as Video).subtitle_burn_task"
                  :percentage="(row as Video).subtitle_burn_task!.progress"
                  :status="(row as Video).subtitle_burn_task!.status === 'failed' ? 'exception' : (row as Video).subtitle_burn_task!.status === 'completed' ? 'success' : ''"
                  :stroke-width="4"
                  class="signal-progress"
                />
              </template>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.deblur')" width="100">
            <template #default="{ row }">
              <template v-if="!isOutputRow(row)">
                <div class="status-cell">
                  <span :class="['vf-led', taskStatusLed((row as Video).deblur_task)]"></span>
                  <span class="status-text">{{ taskStatusText((row as Video).deblur_task) }}</span>
                </div>
                <el-progress
                  v-if="(row as Video).deblur_task"
                  :percentage="(row as Video).deblur_task!.progress"
                  :status="(row as Video).deblur_task!.status === 'failed' ? 'exception' : (row as Video).deblur_task!.status === 'completed' ? 'success' : ''"
                  :stroke-width="4"
                  class="signal-progress"
                />
              </template>
            </template>
          </el-table-column>

          <el-table-column :label="$t('videos.column.action')" width="350" fixed="right">
            <template #default="{ row }">
              <template v-if="!isOutputRow(row)">
                <el-button
                  type="primary"
                  size="small"
                  :disabled="isTaskRunning((row as Video).subtitle_task)"
                  :loading="(row as Video).subtitle_task?.status === 'running'"
                  @click="handleSubtitle(row as Video)"
                >
                  {{ $t('videos.btn.subtitle') }}
                </el-button>
                <el-button
                  type="primary"
                  size="small"
                  :disabled="isTaskRunning((row as Video).subtitle_burn_task)"
                  :loading="(row as Video).subtitle_burn_task?.status === 'running'"
                  @click="handleSubtitleBurn(row as Video)"
                >
                  {{ $t('videos.btn.subtitle_burn') }}
                </el-button>
                <el-button
                  type="warning"
                  size="small"
                  :disabled="isTaskRunning((row as Video).deblur_task)"
                  :loading="(row as Video).deblur_task?.status === 'running'"
                  @click="handleDeblur(row as Video)"
                >
                  {{ $t('videos.btn.deblur') }}
                </el-button>
                <el-button type="danger" size="small" @click="handleDelete(row as Video)">{{ $t('videos.btn.delete') }}</el-button>
              </template>
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
      :video-name="sourceDialogVideo?.name || ''"
      :options="sourceDialogOptions"
      @confirm="handleSourceConfirm"
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
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 14px;
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

.name-cell--child {
  padding-left: 28px;
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

/* 子行背景 */
.panel-body--compact :deep(.el-table__body tr.el-table__row--level-1 td) {
  background: var(--vf-bg-panel-hover);
}

.panel-body--compact :deep(.el-table__body tr.el-table__row--level-1:hover td) {
  background: var(--vf-bg-elevated);
}

.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}
</style>
