<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listVideos, scanVideos, deleteVideo } from '@/api/video'
import { createTask } from '@/api/task'
import type { Video, TaskSnapshot } from '@/api/video'
import { useSettingsStore } from '@/stores/settings'
import { formatDuration, formatFileSize } from '@/utils/format'

const settingsStore = useSettingsStore()

// 扫描目录路径
const scanPath = ref<string>('')
// 加载状态
const loading = ref<boolean>(false)
// 扫描加载状态
const scanning = ref<boolean>(false)
// 视频列表数据
const videoList = ref<Video[]>([])
// 分页信息
const page = ref<number>(1)
const pageSize = ref<number>(12)
const total = ref<number>(0)

/**
 * 加载视频列表
 */
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

/**
 * 扫描目录
 */
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

/**
 * 删除视频记录
 * @param video 视频对象
 */
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

/**
 * 判断任务是否运行中
 * @param task 任务快照
 */
function isTaskRunning(task?: TaskSnapshot): boolean {
  return !!task && (task.status === 'pending' || task.status === 'running')
}

/**
 * 发起字幕任务
 * @param video 视频对象
 */
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

/**
 * 发起修复任务
 * @param video 视频对象
 */
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

/**
 * 发起翻译任务（暂时禁用）
 * @param video 视频对象
 */
// async function handleTranslate(video: Video): Promise<void> {
//   if (isTaskRunning(video.translate_task)) return
//   if (!video.subtitle_task || video.subtitle_task.status !== 'completed') {
//     ElMessage.warning('请先完成字幕生成')
//     return
//   }
//   try {
//     await createTask(video.id, 'translate')
//     ElMessage.success('翻译任务已创建')
//     await loadVideos()
//   } catch {
//     // 请求失败已由拦截器提示
//   }
// }

/**
 * 处理分页变化
 * @param currentPage 当前页
 */
function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadVideos()
}

/**
 * 处理每页数量变化
 * @param size 每页数量
 */
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
  <div class="videos-page">
    <h2>视频列表</h2>

    <!-- 扫描区域 -->
    <div class="scan-bar">
      <el-input
        v-model="scanPath"
        placeholder="请输入本地视频目录路径，为空时使用配置目录"
        clearable
        style="width: 400px"
        @keyup.enter="handleScan"
      />
      <el-button type="primary" :loading="scanning" @click="handleScan">
        <el-icon><Search /></el-icon>扫描
      </el-button>
      <el-button @click="loadVideos">
        <el-icon><Refresh /></el-icon>刷新
      </el-button>
    </div>

    <!-- 视频卡片列表 -->
    <el-row v-loading="loading" :gutter="16">
      <el-col v-for="video in videoList" :key="video.id" :xs="24" :sm="12" :md="8" :lg="6">
        <el-card class="video-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="video-name" :title="video.name">{{ video.name }}</span>
            </div>
          </template>
          <div class="video-info">
            <p class="info-item" :title="video.path">
              <span class="info-label">路径：</span>{{ video.path }}
            </p>
            <p class="info-item">
              <span class="info-label">时长：</span>{{ formatDuration(video.duration) }}
            </p>
            <p class="info-item">
              <span class="info-label">大小：</span>{{ formatFileSize(video.size) }}
            </p>
          </div>

          <div class="task-status">
            <div v-if="video.subtitle_task" class="status-row">
              <span class="status-label">字幕：</span>
              <el-progress
                :percentage="video.subtitle_task.progress"
                :status="video.subtitle_task.status === 'failed' ? 'exception' : video.subtitle_task.status === 'completed' ? 'success' : ''"
                :stroke-width="8"
              />
            </div>
            <div v-if="video.repair_task" class="status-row">
              <span class="status-label">修复：</span>
              <el-progress
                :percentage="video.repair_task.progress"
                :status="video.repair_task.status === 'failed' ? 'exception' : video.repair_task.status === 'completed' ? 'success' : ''"
                :stroke-width="8"
              />
            </div>
            <!-- 翻译任务进度（暂时禁用）
            <div v-if="video.translate_task" class="status-row">
              <span class="status-label">翻译：</span>
              <el-progress
                :percentage="video.translate_task.progress"
                :status="video.translate_task.status === 'failed' ? 'exception' : video.translate_task.status === 'completed' ? 'success' : ''"
                :stroke-width="8"
              />
            </div>
            -->
          </div>

          <div class="card-actions">
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
            <!-- 翻译字幕按钮（暂时禁用）
            <el-tooltip
              v-if="!video.subtitle_task || video.subtitle_task.status !== 'completed'"
              content="请先完成字幕生成"
              placement="top"
            >
              <el-button
                type="success"
                size="small"
                disabled
              >
                翻译字幕
              </el-button>
            </el-tooltip>
            <el-button
              v-else
              type="success"
              size="small"
              :disabled="isTaskRunning(video.translate_task)"
              :loading="video.translate_task?.status === 'running'"
              @click="handleTranslate(video)"
            >
              翻译字幕
            </el-button>
            -->
            <el-button type="danger" size="small" @click="handleDelete(video)">删除</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-empty v-if="!loading && videoList.length === 0" description="暂无视频" />

    <!-- 分页 -->
    <div class="pagination">
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
</template>

<style scoped>
.videos-page {
  padding: 20px;
}

.scan-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.video-card {
  margin-bottom: 16px;
}

.card-header {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.video-name {
  font-weight: 500;
}

.video-info {
  margin-bottom: 12px;
}

.info-item {
  margin: 4px 0;
  font-size: 13px;
  color: #606266;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.info-label {
  color: #909399;
}

.task-status {
  margin-bottom: 12px;
}

.status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
}

.status-label {
  width: 48px;
  color: #606266;
  flex-shrink: 0;
}

.card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
