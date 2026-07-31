<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTasks, retryTask, deleteTask } from '@/api/task'
import type { Task, TaskType } from '@/api/task'
import { formatTime } from '@/utils/format'

// 加载状态
const loading = ref<boolean>(false)
// 任务列表
const taskList = ref<Task[]>([])
// 当前选中的任务类型过滤
const activeType = ref<TaskType | ''>('')
// 分页
const page = ref<number>(1)
const pageSize = ref<number>(10)
const total = ref<number>(0)
// 轮询定时器
let pollTimer: number | null = null

/**
 * 加载任务列表
 */
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

/**
 * 切换任务类型过滤
 * @param type 任务类型
 */
function handleTypeChange(type: TaskType | ''): void {
  activeType.value = type
  page.value = 1
  loadTasks()
}

/**
 * 重试失败任务
 * @param task 任务对象
 */
async function handleRetry(task: Task): Promise<void> {
  try {
    await retryTask(task.id)
    ElMessage.success('任务已重新提交')
    await loadTasks()
  } catch {
    // 失败已由拦截器提示
  }
}

/**
 * 删除任务
 * @param task 任务对象
 */
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

/**
 * 状态标签类型
 * @param status 状态
 */
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

/**
 * 状态中文展示
 * @param status 状态
 */
function statusText(status: string): string {
  const map: Record<string, string> = {
    pending: '待处理',
    running: '进行中',
    completed: '已完成',
    failed: '失败',
  }
  return map[status] || status
}

/**
 * 任务类型中文展示
 * @param type 任务类型
 */
function typeText(type: string): string {
  const map: Record<string, string> = {
    subtitle: '字幕',
    repair: '修复',
    // translate: '翻译',
  }
  return map[type] || type
}

/**
 * 分页变化
 * @param currentPage 当前页
 */
function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadTasks()
}

/**
 * 每页数量变化
 * @param size 每页数量
 */
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

/**
 * 开始轮询任务列表
 */
function startPolling(): void {
  stopPolling()
  pollTimer = window.setInterval(() => {
    loadTasks()
  }, 2000)
}

/**
 * 停止轮询任务列表
 */
function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}
</script>

<template>
  <div class="tasks-page">
    <h2>任务管理</h2>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-radio-group v-model="activeType" @change="handleTypeChange">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="subtitle">字幕</el-radio-button>
        <el-radio-button label="repair">修复</el-radio-button>
        <!-- <el-radio-button label="translate">翻译</el-radio-button> -->
      </el-radio-group>
      <el-button type="primary" @click="loadTasks">
        <el-icon><Refresh /></el-icon>刷新
      </el-button>
      <el-tag type="info">轮询中</el-tag>
    </div>

    <!-- 任务表格 -->
    <el-table v-loading="loading" :data="taskList" style="width: 100%">
      <el-table-column label="视频名" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.video?.name || row.video_id }}
        </template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag :type="row.task_type === 'repair' ? 'warning' : /* row.task_type === 'translate' ? 'success' : */ 'primary'">{{ typeText(row.task_type) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="进度" min-width="220">
        <template #default="{ row }">
          <el-progress :percentage="row.progress" :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : ''" />
          <div v-if="row.progress_msg" class="progress-msg">{{ row.progress_msg }}</div>
        </template>
      </el-table-column>
      <el-table-column label="错误信息" min-width="180" show-overflow-tooltip>
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
    </el-table>

    <!-- 分页 -->
    <div class="pagination">
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
</template>

<style scoped>
.tasks-page {
  padding: 20px;
}

.toolbar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.progress-msg {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}
</style>
