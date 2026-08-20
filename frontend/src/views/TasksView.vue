<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { listTasks, retryTask, cancelTask, deleteTask } from '@/api/task'
import type { Task, TaskType } from '@/api/task'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const loading = ref<boolean>(false)
const taskList = ref<Task[]>([])
const activeType = ref<TaskType | ''>('')
const page = ref<number>(1)
const pageSize = ref<number>(10)
const total = ref<number>(0)
let pollTimer: number | null = null

// 轮询间隔选项（毫秒），空字符串表示不轮询
const pollingOptions = [
  { value: 5000, label: 'tasks.polling.5s' },
  { value: 10000, label: 'tasks.polling.10s' },
  { value: 60000, label: 'tasks.polling.1m' },
  { value: 1800000, label: 'tasks.polling.30m' },
  { value: 0, label: 'tasks.polling.off' },
]
const pollingInterval = ref<number>(5000)

const runningCount = computed(() => taskList.value.filter((t) => t.status === 'running').length)
const failedCount = computed(() => taskList.value.filter((t) => t.status === 'failed').length)

async function loadTasks(): Promise<void> {
  loading.value = true
  try {
    const res = await listTasks(page.value, pageSize.value, activeType.value || undefined)
    taskList.value = res.list
    total.value = res.total
    page.value = res.page
    pageSize.value = res.page_size
  } finally {
    loading.value = false
  }
}

// 轮询时只更新进度、状态、错误信息，不显示全表 loading，也不重置分页
async function pollTasks(): Promise<void> {
  try {
    const res = await listTasks(page.value, pageSize.value, activeType.value || undefined)
    // 只更新进度/状态/错误信息，保持现有数据
    for (const incoming of res.list) {
      const existing = taskList.value.find((t) => t.id === incoming.id)
      if (existing) {
        existing.status = incoming.status
        existing.progress = incoming.progress
        existing.progress_msg = incoming.progress_msg
        existing.error_msg = incoming.error_msg
      }
    }
    total.value = res.total
  } catch {
    // 静默处理轮询错误
  }
}

function handleTypeChange(type: TaskType | ''): void {
  activeType.value = type
  page.value = 1
  loadTasks()
}

async function handleRetry(task: Task): Promise<void> {
  try {
    await retryTask(task.id)
    ElMessage.success(t('tasks.retry.success'))
    await loadTasks()
  } catch {
    // 失败已由拦截器提示
  }
}

async function handleDelete(task: Task): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('tasks.delete.confirm'),
      t('common.notice'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    await deleteTask(task.id)
    ElMessage.success(t('tasks.delete.success'))
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel') {
      // 非取消操作
    }
  }
}

async function handleCancel(task: Task): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('tasks.cancel.confirm'),
      t('common.notice'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    await cancelTask(task.id)
    ElMessage.success(t('tasks.cancel.success'))
    task.status = 'cancelling'
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel') {
      // 非取消操作
    }
  }
}

function statusType(status: string): '' | 'success' | 'danger' | 'warning' | 'info' {
  switch (status) {
    case 'completed':
      return 'success'
    case 'running':
      return ''
    case 'failed':
      return 'danger'
    case 'cancelling':
      return 'warning'
    case 'cancelled':
    case 'pending':
      return 'info'
    default:
      return ''
  }
}

function statusText(status: string): string {
  const map: Record<string, string> = {
    pending: t('tasks.status.pending'),
    running: t('tasks.status.running'),
    completed: t('tasks.status.completed'),
    failed: t('tasks.status.failed'),
    cancelling: t('tasks.status.cancelling'),
    cancelled: t('tasks.status.cancelled'),
  }
  return map[status] || status
}

function typeText(type: string): string {
  const map: Record<string, string> = {
    subtitle: t('tasks.type.subtitle'),
    subtitle_burn: t('tasks.type.subtitle_burn'),
    deblur: t('tasks.type.deblur'),
    upscale: t('tasks.type.upscale'),
  }
  return map[type] || type
}

function statusLedClass(status: string): string {
  if (status === 'completed') return 'vf-led--green'
  if (status === 'failed') return 'vf-led--red'
  if (status === 'running' || status === 'cancelling') return 'vf-led--amber vf-led--pulse'
  return 'vf-led--cyan'
}

function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadTasks()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  page.value = 1
  loadTasks()
}

function handlePollingChange(value: number): void {
  pollingInterval.value = value
  startPolling()
}

function startPolling(): void {
  stopPolling()
  if (pollingInterval.value <= 0) return
  pollTimer = window.setInterval(() => {
    pollTasks()
  }, pollingInterval.value)
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  loadTasks()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="tasks-view">
    <div class="vf-panel">
      <div class="vf-panel__footer"></div>

      <div class="vf-panel-header">
        <div class="vf-panel-header__title">
          <span class="vf-led vf-led--cyan vf-led--pulse"></span>
          <span>{{ $t('tasks.title') }}</span>
          <span class="header__count">{{ $t('tasks.count', { count: total }) }}</span>
        </div>

        <div class="monitor-stats">
          <div class="stat stat--running">
            <span class="vf-led vf-led--amber vf-led--pulse"></span>
            <span class="stat__label">{{ $t('tasks.running') }}</span>
            <span class="stat__value">{{ runningCount }}</span>
          </div>
          <div class="stat stat--failed">
            <span class="vf-led vf-led--red"></span>
            <span class="stat__label">{{ $t('tasks.failed') }}</span>
            <span class="stat__value">{{ failedCount }}</span>
          </div>
        </div>
      </div>

      <div class="panel-toolbar">
        <el-radio-group v-model="activeType" @change="handleTypeChange">
          <el-radio-button label="">{{ $t('tasks.filter.all') }}</el-radio-button>
          <el-radio-button label="subtitle">{{ $t('tasks.filter.subtitle') }}</el-radio-button>
          <el-radio-button label="subtitle_burn">{{ $t('tasks.filter.subtitle_burn') }}</el-radio-button>
          <el-radio-button label="deblur">{{ $t('tasks.filter.deblur') }}</el-radio-button>
          <el-radio-button label="upscale">{{ $t('tasks.filter.upscale') }}</el-radio-button>
        </el-radio-group>

        <div class="toolbar-right">
          <div class="poll-control">
            <span class="vf-data-label">{{ $t('tasks.polling.label') }}</span>
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
          <el-button type="primary" @click="loadTasks">
            <el-icon><Refresh /></el-icon>{{ $t('tasks.refresh') }}
          </el-button>
        </div>
      </div>

      <div class="panel-body panel-body--compact">
        <el-table v-loading="loading" :data="taskList" style="width: 100%">
          <el-table-column :label="$t('tasks.column.id')" width="130">
            <template #default="{ row }">
              <span class="task-id">#{{ row.id.slice(-8).toUpperCase() }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.video')" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.video?.name || row.video_id }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.type')" width="100">
            <template #default="{ row }">
              <el-tag :type="row.task_type === 'deblur' ? 'warning' : row.task_type === 'subtitle_burn' ? 'success' : row.task_type === 'upscale' ? 'success' : 'primary'">{{ typeText(row.task_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.status')" width="130">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', statusLedClass(row.status)]"></span>
                <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.progress')" min-width="240">
            <template #default="{ row }">
              <el-progress :percentage="row.progress" :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : ''" />
              <div v-if="row.progress_msg" class="progress-msg">{{ row.progress_msg }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.error')" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.error_msg || '-' }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.created')" width="170">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.column.action')" width="210" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'failed' || row.status === 'cancelled'"
                type="warning"
                size="small"
                @click="handleRetry(row)"
              >
                {{ $t('tasks.btn.retry') }}
              </el-button>
              <el-button
                v-if="row.status === 'running' || row.status === 'pending'"
                type="danger"
                plain
                size="small"
                @click="handleCancel(row)"
              >
                {{ $t('tasks.btn.cancel') }}
              </el-button>
              <el-button
                v-if="row.status !== 'running' && row.status !== 'cancelling'"
                type="danger"
                size="small"
                @click="handleDelete(row)"
              >
                {{ $t('tasks.btn.delete') }}
              </el-button>
            </template>
          </el-table-column>
          <template #empty>
            <div class="empty-state">{{ $t('tasks.empty') }}</div>
          </template>
        </el-table>
      </div>

      <div class="panel-footer">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.tasks-view {
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

.monitor-stats {
  display: flex;
  align-items: center;
  gap: 20px;
}

.stat {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  padding: 4px 10px;
  background: var(--vf-bg-panel);
}

.stat__label {
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
  letter-spacing: 0.06em;
}

.stat__value {
  font-family: var(--vf-font-mono);
  font-size: 14px;
  font-weight: 600;
  color: var(--vf-text-primary);
  min-width: 20px;
  text-align: right;
}

.panel-toolbar {
  padding: 12px 16px;
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.toolbar-right {
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

.task-id {
  font-family: var(--vf-font-mono);
  font-size: 12px;
  color: var(--vf-text-muted);
  letter-spacing: 0.04em;
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.progress-msg {
  margin-top: 4px;
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  line-height: 1.4;
}

.empty-state {
  padding: 40px 0;
  color: var(--vf-text-muted);
  font-size: 14px;
}

.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}
</style>
