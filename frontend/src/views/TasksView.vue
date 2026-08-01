<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTasks, retryTask, deleteTask } from '@/api/task'
import type { Task, TaskType } from '@/api/task'
import { formatTime } from '@/utils/format'

const loading = ref<boolean>(false)
const taskList = ref<Task[]>([])
const activeType = ref<TaskType | ''>('')
const page = ref<number>(1)
const pageSize = ref<number>(10)
const total = ref<number>(0)
let pollTimer: number | null = null

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

function handleTypeChange(type: TaskType | ''): void {
  activeType.value = type
  page.value = 1
  loadTasks()
}

async function handleRetry(task: Task): Promise<void> {
  try {
    await retryTask(task.id)
    ElMessage.success('任务已重新提交')
    await loadTasks()
  } catch {
    // 失败已由拦截器提示
  }
}

async function handleDelete(task: Task): Promise<void> {
  try {
    await ElMessageBox.confirm(`确定删除该任务吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await deleteTask(task.id)
    ElMessage.success('删除成功')
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
    case 'pending':
      return 'info'
    default:
      return ''
  }
}

function statusText(status: string): string {
  const map: Record<string, string> = {
    pending: '待处理',
    running: '进行中',
    completed: '已完成',
    failed: '失败',
  }
  return map[status] || status
}

function typeText(type: string): string {
  const map: Record<string, string> = {
    subtitle: '字幕',
    repair: '修复',
  }
  return map[type] || type
}

function statusLedClass(status: string): string {
  if (status === 'completed') return 'vf-led--green'
  if (status === 'failed') return 'vf-led--red'
  if (status === 'running') return 'vf-led--amber vf-led--pulse'
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

onMounted(() => {
  loadTasks()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})

function startPolling(): void {
  stopPolling()
  pollTimer = window.setInterval(() => {
    loadTasks()
  }, 2000)
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}
</script>

<template>
  <div class="tasks-view">
    <div class="vf-panel">
      <div class="vf-panel__footer"></div>

      <div class="vf-panel-header">
        <div class="vf-panel-header__title">
          <span class="vf-led vf-led--cyan vf-led--pulse"></span>
          <span>任务监控台</span>
          <span class="header__count">{{ total }} 个任务</span>
        </div>

        <div class="monitor-stats">
          <div class="stat stat--running">
            <span class="vf-led vf-led--amber vf-led--pulse"></span>
            <span class="stat__label">运行中</span>
            <span class="stat__value">{{ runningCount }}</span>
          </div>
          <div class="stat stat--failed">
            <span class="vf-led vf-led--red"></span>
            <span class="stat__label">失败</span>
            <span class="stat__value">{{ failedCount }}</span>
          </div>
        </div>
      </div>

      <div class="panel-toolbar">
        <el-radio-group v-model="activeType" @change="handleTypeChange">
          <el-radio-button label="">全部</el-radio-button>
          <el-radio-button label="subtitle">字幕</el-radio-button>
          <el-radio-button label="repair">修复</el-radio-button>
        </el-radio-group>

        <div class="toolbar-right">
          <div class="poll-indicator">
            <span class="vf-led vf-led--green vf-led--pulse"></span>
            <span class="vf-data-label">轮询 2秒</span>
          </div>
          <el-button type="primary" @click="loadTasks">
            <el-icon><Refresh /></el-icon>刷新
          </el-button>
        </div>
      </div>

      <div class="panel-body panel-body--compact">
        <el-table v-loading="loading" :data="taskList" style="width: 100%">
          <el-table-column label="任务编号" width="130">
            <template #default="{ row }">
              <span class="task-id">#{{ row.id.slice(-8).toUpperCase() }}</span>
            </template>
          </el-table-column>
          <el-table-column label="视频名" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.video?.name || row.video_id }}
            </template>
          </el-table-column>
          <el-table-column label="类型" width="100">
            <template #default="{ row }">
              <el-tag :type="row.task_type === 'repair' ? 'warning' : 'primary'">{{ typeText(row.task_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="130">
            <template #default="{ row }">
              <div class="status-cell">
                <span :class="['vf-led', statusLedClass(row.status)]"></span>
                <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="进度" min-width="240">
            <template #default="{ row }">
              <el-progress :percentage="row.progress" :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : ''" />
              <div v-if="row.progress_msg" class="progress-msg">{{ row.progress_msg }}</div>
            </template>
          </el-table-column>
          <el-table-column label="错误信息" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.error_msg || '-' }}
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="170">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.status === 'failed'" type="warning" size="small" @click="handleRetry(row)">
                重试
              </el-button>
              <el-button v-if="row.status !== 'running'" type="danger" size="small" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <div class="empty-state">暂无任务数据</div>
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

.poll-indicator {
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
