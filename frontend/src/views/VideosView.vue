<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listVideos, scanVideos, deleteVideo } from '@/api/video'
import { createTask } from '@/api/task'
import type { Video, TaskSnapshot } from '@/api/video'
import { useSettingsStore } from '@/stores/settings'
import { formatDuration, formatFileSize } from '@/utils/format'

const settingsStore = useSettingsStore()

const scanPath = ref<string>('')
const loading = ref<boolean>(false)
const scanning = ref<boolean>(false)
const videoList = ref<Video[]>([])
const page = ref<number>(1)
const pageSize = ref<number>(12)
const total = ref<number>(0)

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

async function handleScan(): Promise<void> {
  const path = scanPath.value.trim() || settingsStore.setting.video_dir
  if (!path) {
    ElMessage.warning('请输入本地目录路径或先在设置中配置视频目录')
    return
  }
  scanning.value = true
  try {
    const res = await scanVideos(path)
    ElMessage.success(`扫描完成，共识别 ${res.scanned} 个视频`)
    await loadVideos()
  } finally {
    scanning.value = false
  }
}

async function handleDelete(video: Video): Promise<void> {
  try {
    await ElMessageBox.confirm(`确定删除视频记录 "${video.name}" 吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await deleteVideo(video.id)
    ElMessage.success('删除成功')
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
    ElMessage.success('字幕任务已创建')
    await loadVideos()
  } catch {
    // 请求失败已由拦截器提示
  }
}

async function handleRepair(video: Video): Promise<void> {
  if (isTaskRunning(video.repair_task)) return
  try {
    await createTask(video.id, 'repair')
    ElMessage.success('视频修复任务已创建')
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

function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadVideos()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  page.value = 1
  loadVideos()
}

onMounted(() => {
  settingsStore.init()
  loadVideos()
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
          <span>视频资源库</span>
          <span class="header__count">{{ total }} 个资源</span>
        </div>

        <div class="scan-control">
          <el-input
            v-model="scanPath"
            placeholder="输入本地视频目录路径，为空时使用配置目录"
            clearable
            style="width: 380px"
            @keyup.enter="handleScan"
          />
          <el-button type="primary" :loading="scanning" @click="handleScan">
            <el-icon><Search /></el-icon>扫描
          </el-button>
          <el-button @click="loadVideos">
            <el-icon><Refresh /></el-icon>刷新
          </el-button>
        </div>
      </div>

      <div class="panel-body">
        <el-row v-loading="loading" :gutter="16">
          <el-col v-for="video in videoList" :key="video.id" :xs="24" :sm="12" :md="8" :lg="6" :xl="4">
            <div class="asset-card">
              <div class="asset-card__header">
                <span class="asset-card__name" :title="video.name">{{ video.name }}</span>
                <span class="asset-card__id">#{{ video.id.slice(-6).toUpperCase() }}</span>
              </div>

              <div class="asset-card__metrics">
                <div class="metric">
                  <span class="vf-data-label">时长</span>
                  <span class="vf-data-value">{{ formatDuration(video.duration) }}</span>
                </div>
                <div class="metric">
                  <span class="vf-data-label">大小</span>
                  <span class="vf-data-value">{{ formatFileSize(video.size) }}</span>
                </div>
              </div>

              <div class="asset-card__path">
                <span class="vf-data-label">路径</span>
                <span class="path-value" :title="video.path">{{ video.path }}</span>
              </div>

              <div class="asset-card__signals">
                <div class="signal-row">
                  <span class="vf-data-label">字幕</span>
                  <span v-if="video.subtitle_task" :class="['vf-led', taskStatusLed(video.subtitle_task)]"></span>
                  <span v-else class="signal-empty">--</span>
                  <el-progress
                    v-if="video.subtitle_task"
                    :percentage="video.subtitle_task.progress"
                    :status="video.subtitle_task.status === 'failed' ? 'exception' : video.subtitle_task.status === 'completed' ? 'success' : ''"
                    :stroke-width="6"
                    class="signal-progress"
                  />
                </div>
                <div class="signal-row">
                  <span class="vf-data-label">修复</span>
                  <span v-if="video.repair_task" :class="['vf-led', taskStatusLed(video.repair_task)]"></span>
                  <span v-else class="signal-empty">--</span>
                  <el-progress
                    v-if="video.repair_task"
                    :percentage="video.repair_task.progress"
                    :status="video.repair_task.status === 'failed' ? 'exception' : video.repair_task.status === 'completed' ? 'success' : ''"
                    :stroke-width="6"
                    class="signal-progress"
                  />
                </div>
              </div>

              <div class="asset-card__actions">
                <el-button
                  type="primary"
                  size="small"
                  :disabled="isTaskRunning(video.subtitle_task)"
                  :loading="video.subtitle_task?.status === 'running'"
                  @click="handleSubtitle(video)"
                >
                  生成字幕
                </el-button>
                <el-button
                  type="warning"
                  size="small"
                  :disabled="isTaskRunning(video.repair_task)"
                  :loading="video.repair_task?.status === 'running'"
                  @click="handleRepair(video)"
                >
                  视频修复
                </el-button>
                <el-button type="danger" size="small" @click="handleDelete(video)">删除</el-button>
              </div>
            </div>
          </el-col>
        </el-row>

        <el-empty v-if="!loading && videoList.length === 0" description="暂无视频资源" />
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

.panel-body {
  padding: 16px;
  flex: 1;
}

.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}

.asset-card {
  background: var(--vf-bg-elevated);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  margin-bottom: 16px;
  overflow: hidden;
  transition: all 0.2s ease;
  position: relative;
}

.asset-card::before,
.asset-card::after {
  content: '';
  position: absolute;
  top: -1px;
  width: 6px;
  height: 6px;
  border: 1px solid var(--vf-border-light);
}

.asset-card::before {
  left: -1px;
  border-right: 0;
  border-bottom: 0;
}

.asset-card::after {
  right: -1px;
  border-left: 0;
  border-bottom: 0;
}

.asset-card:hover {
  border-color: var(--vf-border-active);
  box-shadow: 0 0 0 1px var(--vf-border-active), 0 6px 18px rgba(0, 0, 0, 0.4);
}

[data-theme='light'] .asset-card:hover {
  box-shadow: 0 0 0 1px var(--vf-border-active), 0 6px 18px rgba(0, 0, 0, 0.12);
}

.asset-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--vf-border);
  background: var(--vf-bg-panel-hover);
}

.asset-card__name {
  font-family: var(--vf-font-display);
  font-weight: 500;
  font-size: 14px;
  color: var(--vf-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.asset-card__id {
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
  letter-spacing: 0.05em;
}

.asset-card__metrics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1px;
  background: var(--vf-border);
  border-bottom: 1px solid var(--vf-border);
}

.metric {
  background: var(--vf-bg-elevated);
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.metric .vf-data-value {
  font-size: 13px;
}

.asset-card__path {
  padding: 10px 12px;
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.path-value {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-card__signals {
  padding: 10px 12px;
  border-bottom: 1px solid var(--vf-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.signal-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.signal-row .vf-data-label {
  width: 36px;
  flex-shrink: 0;
}

.signal-empty {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-disabled);
}

.signal-progress {
  flex: 1;
}

.asset-card__actions {
  padding: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  background: var(--vf-bg-panel-hover);
}
</style>
