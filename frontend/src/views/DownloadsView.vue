<script setup lang="ts">
import { ref, onMounted, onUnmounted, h } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { createDownload, listDownloads, cancelDownload, deleteDownload } from '@/api/download'
import type { Download, DownloadStatus } from '@/api/download'
import { formatFileSize, formatSpeed } from '@/utils/format'
import { useResponsive } from '@/composables/useResponsive'
import { useSettingsStore } from '@/stores/settings'
import VfListPanel from '@/components/VfListPanel.vue'

const { t } = useI18n()
const { isMobileOnly, isMobileOrTablet, isDesktop } = useResponsive()
const settingsStore = useSettingsStore()

// 搜索引擎状态
const urlInput = ref<string>('')
const searching = ref<boolean>(false)
const hasSubmitted = ref<boolean>(false)
const overwrite = ref<boolean>(false) // 文件冲突时覆盖还是自动重命名
const downloadDirMode = ref<'video' | 'output'>('video') // 下载路径：video=本地视频目录, output=输出目录

// 下载列表
const downloadingList = ref<Download[]>([])
const historyList = ref<Download[]>([])
const total = ref<number>(0)
const page = ref<number>(1)
const pageSize = ref<number>(10)
// 排序状态（空串表示默认排序：创建时间倒序）
const sortBy = ref<string>('')
const sortOrder = ref<'asc' | 'desc' | ''>('')

let pollTimer: number | null = null

// 下载状态对应的 LED class
function statusLedClass(status: DownloadStatus): string {
  switch (status) {
    case 'completed': return 'vf-led vf-led--green'
    case 'failed': return 'vf-led vf-led--red'
    case 'cancelled': return 'vf-led vf-led--cyan'
    case 'probing':
    case 'downloading': return 'vf-led vf-led--amber vf-led--pulse'
    default: return 'vf-led vf-led--cyan'
  }
}

// 状态文本
function statusText(status: DownloadStatus): string {
  const key = `downloads.status.${status}`
  const text = t(key)
  return text
}

// 验证 URL 格式（简单校验：不能为空，包含 ://）
function isValidUrl(url: string): boolean {
  return url.trim().length > 0 && url.includes('://')
}

// 支持的平台列表
const supportedPlatforms = [
  { name: 'YouTube', icon: 'youtube' },
  { name: 'Bilibili', icon: 'bilibili' },
  { name: 'Twitter/X', icon: 'twitter' },
  { name: 'Instagram', icon: 'instagram' },
  { name: 'TikTok', icon: 'tiktok' },
  { name: 'Facebook', icon: 'facebook' },
  { name: 'Twitch', icon: 'twitch' },
  { name: 'Vimeo', icon: 'vimeo' },
  { name: 'Niconico', icon: 'niconico' },
  { name: 'Dailymotion', icon: 'dailymotion' },
  { name: 'Reddit', icon: 'reddit' },
  { name: 'Tumblr', icon: 'tumblr' },
]

// 提交下载
async function handleSubmit(): Promise<void> {
  const url = urlInput.value.trim()
  if (!url) {
    ElMessage.warning(t('downloads.error.empty_url'))
    return
  }
  if (!isValidUrl(url)) {
    ElMessage.warning(t('downloads.error.invalid_url'))
    return
  }

  searching.value = true
  hasSubmitted.value = true

  try {
    // 根据选择的目录模式确定下载路径
    let downloadDir: string | undefined
    if (downloadDirMode.value === 'video') {
      downloadDir = settingsStore.setting.video_dir || undefined
    } else {
      downloadDir = settingsStore.setting.output_dir || undefined
    }
    const dl = await createDownload(url, overwrite.value, downloadDir)
    downloadingList.value.unshift(dl)
    urlInput.value = ''
    // 开始轮询
    startPolling()
  } catch {
    // 错误已在拦截器中处理
  } finally {
    searching.value = false
  }
}

// 回车提交
function handleKeydown(e: KeyboardEvent): void {
  if (e.key === 'Enter') {
    handleSubmit()
  }
}

// 取消下载
async function handleCancel(id: string): Promise<void> {
  try {
    await cancelDownload(id)
    // 本地更新状态
    const dl = downloadingList.value.find((d) => d.id === id)
    if (dl) {
      dl.status = 'cancelled'
      dl.progress = 0
      dl.progress_msg = t('downloads.cancelled')
    }
    // 也从历史列表更新
    const hdl = historyList.value.find((d) => d.id === id)
    if (hdl) {
      hdl.status = 'cancelled'
      hdl.progress = 0
      hdl.progress_msg = t('downloads.cancelled')
    }
  } catch {
    // 错误已在拦截器中处理
  }
}

// 删除记录
async function handleDelete(id: string): Promise<void> {
  try {
    await ElMessageBox.confirm(
      h('div', { style: 'display:flex;flex-direction:column;gap:12px;' }, [
        h('p', { style: 'margin:0;' }, t('downloads.confirm_delete')),
        h('label', { style: 'display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer;' }, [
          h('input', {
            type: 'checkbox',
            id: 'delete-file-checkbox',
            style: 'width:16px;height:16px;cursor:pointer;',
          }),
          t('downloads.delete_file_label'),
        ]),
      ]),
      t('common.confirm'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
        dangerouslyUseHTMLString: false,
        customClass: 'delete-download-dialog',
      },
    )
    // 读取复选框状态
    const cb = document.getElementById('delete-file-checkbox') as HTMLInputElement | null
    const deleteFile = cb ? cb.checked : false

    await deleteDownload(id, deleteFile)
    downloadingList.value = downloadingList.value.filter((d) => d.id !== id)
    historyList.value = historyList.value.filter((d) => d.id !== id)
    total.value--
    ElMessage.success(t('downloads.delete_success'))
  } catch {
    // cancelled
  }
}

// 加载下载列表
async function loadDownloads(): Promise<void> {
  try {
    const res = await listDownloads(page.value, pageSize.value, sortBy.value || undefined, sortOrder.value || undefined)
    total.value = res.total
    historyList.value = res.list

    // 筛选出活跃的下载（pending/probing/downloading）
    downloadingList.value = res.list.filter(
      (d) => d.status === 'pending' || d.status === 'probing' || d.status === 'downloading'
    )

    // 如果无活跃下载，停止轮询
    if (downloadingList.value.length === 0 && pollTimer !== null) {
      stopPolling()
    }
  } catch {
    // ignore
  }
}

// 轮询
function startPolling(): void {
  if (pollTimer !== null) return
  pollTimer = window.setInterval(async () => {
    await loadDownloads()
  }, 3000)
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(async () => {
		  // 加载设置（获取视频目录和输出目录路径）
		  try {
		    await settingsStore.loadSettings()
		  } catch {
		    // ignore
		  }
		  await loadDownloads()
		  // 只在有下载记录时切换到已提交视图，否则展示初始搜索页
		  if (total.value > 0) {
		    hasSubmitted.value = true
		  }
		  if (total.value > 0 && downloadingList.value.length > 0) {
		    startPolling()
		  }
		})

onUnmounted(() => {
  stopPolling()
})

// 排序变化
function handleSortChange({ prop, order }: { prop: string; order: string | null }): void {
  sortBy.value = prop === 'created_at' || prop === 'file_size' ? prop : ''
  sortOrder.value = order === 'ascending' ? 'asc' : order === 'descending' ? 'desc' : ''
  page.value = 1
  loadDownloads()
}

function isActive(status: DownloadStatus): boolean {
  return status === 'pending' || status === 'probing' || status === 'downloading'
}

function isCancelable(status: DownloadStatus): boolean {
  return status === 'pending' || status === 'probing' || status === 'downloading'
}

function handlePageChange(currentPage: number): void {
  page.value = currentPage
  loadDownloads()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  page.value = 1
  loadDownloads()
}
</script>

<template>
  <div class="downloads-view responsive-page" :class="{ 'has-submitted': hasSubmitted }">
    <!-- 初始态：搜索引擎风格 -->
    <template v-if="!hasSubmitted">
      <div class="search-landing">
        <div class="search-brand">
          <div class="brand-icon">
            <el-icon size="48"><Download /></el-icon>
          </div>
          <h1 class="brand-title">{{ $t('app.title') }}</h1>
          <p class="brand-subtitle">{{ $t('downloads.subtitle') }}</p>
        </div>

        <div class="search-box-wrapper">
          <div class="search-box">
            <div class="search-box-form">
              <el-input
                v-model="urlInput"
                :placeholder="$t('downloads.placeholder')"
                size="large"
                clearable
                @keydown="handleKeydown"
                class="search-box-input"
              >
                <template #prefix>
                  <el-icon><Link /></el-icon>
                </template>
              </el-input>
              <el-button
                type="primary"
                :loading="searching"
                @click="handleSubmit"
                class="search-btn"
              >
                <el-icon><Download /></el-icon>
                <span>{{ $t('downloads.download') }}</span>
              </el-button>
            </div>
          </div>
        </div>

        <!-- 文件冲突处理选项 -->
        <div class="overwrite-option">
          <label class="overwrite-label">
            <el-checkbox v-model="overwrite" size="small" />
            <span>{{ $t('downloads.overwrite_label') }}</span>
          </label>
        </div>

        <!-- 下载路径选择 -->
        <div class="download-path-selector">
          <label class="path-option" :class="{ active: downloadDirMode === 'video' }">
            <el-radio v-model="downloadDirMode" value="video" size="small" />
            <div class="path-info">
              <span class="path-label">{{ $t('downloads.path_video_dir') }}</span>
              <span class="path-value" :title="settingsStore.setting.video_dir || '-'">{{ settingsStore.setting.video_dir || '-' }}</span>
            </div>
          </label>
          <label class="path-option" :class="{ active: downloadDirMode === 'output' }">
            <el-radio v-model="downloadDirMode" value="output" size="small" />
            <div class="path-info">
              <span class="path-label">{{ $t('downloads.path_output_dir') }}</span>
              <span class="path-value" :title="settingsStore.setting.output_dir || '-'">{{ settingsStore.setting.output_dir || '-' }}</span>
            </div>
          </label>
        </div>

        <div class="platform-tags">
          <span class="platform-label">{{ $t('downloads.supported_platforms') }}</span>
          <div class="platform-list">
            <span
              v-for="p in supportedPlatforms"
              :key="p.name"
              class="platform-tag"
            >
              {{ p.name }}
            </span>
          </div>
        </div>
      </div>
    </template>

    <!-- 已提交：顶部搜索栏 + 进度区域 -->
    <template v-else>
      <!-- 顶栏缩小版搜索 -->
      <div class="top-search-bar vf-panel">
        <div class="top-search-form">
          <el-input
            v-model="urlInput"
            :placeholder="$t('downloads.placeholder')"
            clearable
            @keydown="handleKeydown"
            class="top-search-input"
          >
            <template #prefix>
              <el-icon><Link /></el-icon>
            </template>
          </el-input>
          <el-button
            type="primary"
            :loading="searching"
            @click="handleSubmit"
            round
            class="top-search-form-btn"
          >
            <el-icon><Download /></el-icon>
            <span class="hide-mobile">{{ $t('downloads.download') }}</span>
          </el-button>
        </div>
        <div class="top-search-options">
          <label class="overwrite-label">
            <el-checkbox v-model="overwrite" size="small" />
            <span>{{ $t('downloads.overwrite_label') }}</span>
          </label>
        </div>
        <div class="top-search-path">
          <label class="path-option" :class="{ active: downloadDirMode === 'video' }">
            <el-radio v-model="downloadDirMode" value="video" size="small" />
            <div class="path-info">
              <span class="path-label">{{ $t('downloads.path_video_dir') }}</span>
              <span class="path-value" :title="settingsStore.setting.video_dir || '-'">{{ settingsStore.setting.video_dir || '-' }}</span>
            </div>
          </label>
          <label class="path-option" :class="{ active: downloadDirMode === 'output' }">
            <el-radio v-model="downloadDirMode" value="output" size="small" />
            <div class="path-info">
              <span class="path-label">{{ $t('downloads.path_output_dir') }}</span>
              <span class="path-value" :title="settingsStore.setting.output_dir || '-'">{{ settingsStore.setting.output_dir || '-' }}</span>
            </div>
          </label>
        </div>
      </div>

      <!-- 活跃下载卡片 -->
      <div v-if="downloadingList.length > 0" class="active-downloads">
        <h3 class="section-title">{{ $t('downloads.active_title') }}</h3>
        <div class="download-cards">
          <div
            v-for="dl in downloadingList"
            :key="dl.id"
            class="download-card vf-panel"
          >
            <div class="card-header">
              <div class="card-info">
                <div class="card-title" :title="dl.title || dl.url">{{ dl.title || dl.url }}</div>
                <div class="card-url">{{ dl.url }}</div>
              </div>
              <div class="card-status">
                <span :class="statusLedClass(dl.status)"></span>
                <span class="card-status__text">{{ statusText(dl.status) }}</span>
              </div>
            </div>
            <div class="card-progress">
              <el-progress
                :percentage="dl.progress"
                :stroke-width="8"
                :status="dl.status === 'failed' ? 'exception' : undefined"
              />
              <div class="progress-meta">
                <div v-if="dl.progress_msg" class="progress-msg">{{ dl.progress_msg }}</div>
                <div class="progress-stats">
                  <span v-if="dl.download_speed && dl.download_speed > 0" class="stat-item">
                    <el-icon size="12"><TrendCharts /></el-icon>
                    {{ formatSpeed(dl.download_speed) }}
                  </span>
                  <span v-if="dl.downloaded_size && dl.total_size && dl.total_size > 0" class="stat-item">
                    {{ formatFileSize(dl.downloaded_size) }} / {{ formatFileSize(dl.total_size) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="card-actions">
              <el-button
                v-if="isCancelable(dl.status)"
                size="small"
                type="warning"
                plain
                @click="handleCancel(dl.id)"
              >
                {{ $t('downloads.cancel') }}
              </el-button>
              <el-button
                v-if="!isActive(dl.status)"
                size="small"
                type="danger"
                plain
                @click="handleDelete(dl.id)"
              >
                {{ $t('downloads.delete') }}
              </el-button>
            </div>
          </div>
        </div>
      </div>

      <!-- 历史下载记录 -->
      <div class="history-section">
        <div class="history-panel">
          <div class="history-panel__header">
            <div class="history-panel__title">
              <span class="vf-led vf-led--cyan"></span>
              <span>{{ $t('downloads.history_title') }}</span>
              <span class="history-panel__count">{{ total }}</span>
            </div>
          </div>

          <!-- 桌面端/平板端：表格模式 -->
          <el-table
            v-if="!isMobileOnly"
            :data="historyList"
            stripe
            style="width: 100%"
            @sort-change="handleSortChange"
          >
            <el-table-column prop="title" :label="$t('downloads.column.title')" min-width="180">
              <template #default="{ row }">
                <div class="cell-title">
                  <span class="title-text">{{ row.title || '- -' }}</span>
                  <span class="title-url">{{ row.url }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="$t('downloads.column.status')" width="120" align="center">
              <template #default="{ row }">
                <div class="status-cell">
                  <span :class="statusLedClass(row.status)"></span>
                  <span>{{ statusText(row.status) }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="progress" :label="$t('downloads.column.progress')" width="140">
              <template #default="{ row }">
                <el-progress
                  :percentage="row.progress"
                  :stroke-width="6"
                  :status="row.status === 'failed' ? 'exception' : undefined"
                />
              </template>
            </el-table-column>
            <el-table-column v-if="!isMobileOnly" prop="file_name" :label="$t('downloads.column.file')" min-width="140">
              <template #default="{ row }">
                <span v-if="row.file_name" class="vf-data-value">{{ row.file_name }}</span>
                <span v-else class="vf-data-label">- -</span>
              </template>
            </el-table-column>
            <el-table-column v-if="isDesktop || !isMobileOrTablet" :label="$t('downloads.column.size')" width="90" sortable="custom" prop="file_size">
              <template #default="{ row }">
                <span class="vf-data-value">{{ row.file_size ? formatFileSize(row.file_size) : '- -' }}</span>
              </template>
            </el-table-column>
            <el-table-column v-if="isDesktop || !isMobileOrTablet" :label="$t('downloads.column.created')" width="150" sortable="custom" prop="created_at">
              <template #default="{ row }">
                <span class="vf-data-value">{{ new Date(row.created_at * 1000).toLocaleString() }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('downloads.column.actions')" width="100" align="center">
              <template #default="{ row }">
                <div class="action-btns">
                  <el-button
                    v-if="isCancelable(row.status)"
                    size="small"
                    type="warning"
                    plain
                    @click="handleCancel(row.id)"
                  >
                    {{ $t('downloads.cancel') }}
                  </el-button>
                  <el-button
                    v-if="!isActive(row.status)"
                    size="small"
                    type="danger"
                    plain
                    @click="handleDelete(row.id)"
                  >
                    {{ $t('downloads.delete') }}
                  </el-button>
                </div>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty :description="$t('downloads.empty_history')" />
            </template>
          </el-table>

          <!-- 手机端（< 480px）：紧凑卡片模式 -->
          <div v-else class="history-cards">
            <div v-for="dl in historyList" :key="dl.id" class="history-card">
              <div class="history-card__top">
                <div class="history-card__info">
                  <div class="history-card__title" :title="dl.title || dl.url">{{ dl.title || dl.url }}</div>
                  <div class="history-card__url">{{ dl.url }}</div>
                </div>
                <span :class="statusLedClass(dl.status)"></span>
              </div>
              <div class="history-card__mid">
                <span class="history-card__status">{{ statusText(dl.status) }}</span>
                <span v-if="dl.progress > 0 && dl.progress < 100" class="history-card__progress">{{ dl.progress }}%</span>
              </div>
              <div class="history-card__meta">
                <span v-if="dl.file_name" class="history-card__file">{{ dl.file_name }}</span>
                <span v-if="dl.file_size" class="history-card__size">{{ formatFileSize(dl.file_size) }}</span>
              </div>
              <div class="history-card__actions">
                <el-button
                  v-if="isCancelable(dl.status)"
                  size="small" type="warning" plain round
                  @click="handleCancel(dl.id)"
                >{{ $t('downloads.cancel') }}</el-button>
                <el-button
                  v-if="!isActive(dl.status)"
                  size="small" type="danger" plain round
                  @click="handleDelete(dl.id)"
                >{{ $t('downloads.delete') }}</el-button>
              </div>
            </div>
            <div v-if="historyList.length === 0" class="empty-state">
              <el-empty :description="$t('downloads.empty_history')" />
            </div>
          </div>

          <!-- 分页 -->
          <div v-if="total > 0" class="history-panel__footer">
            <el-pagination
              v-model:current-page="page"
              v-model:page-size="pageSize"
              :total="total"
              :page-sizes="[10, 20, 50]"
              :layout="isMobileOnly ? 'prev, pager, next' : 'total, sizes, prev, pager, next'"
              size="small"
              @current-change="handlePageChange"
              @size-change="handleSizeChange"
            />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.downloads-view {
  min-height: calc(100vh - 100px);
  display: flex;
  flex-direction: column;
}

@media (max-width: 767px) {
  .downloads-view {
    min-height: calc(100vh - 52px - 56px);
  }
}

.downloads-view.has-submitted {
  display: block;
}

/* ===== 搜索引擎初始态 ===== */
.search-landing {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 32px;
  padding-bottom: 80px;
}

@media (max-width: 767px) {
  .search-landing {
    gap: 24px;
    padding-bottom: 40px;
  }
}

@media (max-width: 480px) {
  .search-landing {
    gap: 20px;
    padding-bottom: 20px;
    justify-content: flex-start;
    padding-top: 40px;
  }
}

.search-brand {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.brand-icon {
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--vf-accent-soft);
  border: 1px solid var(--vf-accent-border);
  border-radius: 20px;
  color: var(--vf-accent);
  box-shadow: var(--vf-glow-amber);
}

@media (max-width: 767px) {
  .brand-icon {
    width: 64px;
    height: 64px;
    border-radius: 16px;
  }
}

@media (max-width: 480px) {
  .brand-icon {
    width: 56px;
    height: 56px;
    border-radius: 14px;
  }
  .brand-icon :deep(.el-icon) {
    font-size: 32px !important;
  }
}

.brand-title {
  font-family: var(--vf-font-display);
  font-size: 36px;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--vf-text-primary);
  margin: 0;
}

@media (max-width: 767px) {
  .brand-title {
    font-size: 28px;
  }
}

@media (max-width: 480px) {
  .brand-title {
    font-size: 24px;
  }
}

.brand-subtitle {
  font-family: var(--vf-font-ui);
  font-size: 14px;
  color: var(--vf-text-muted);
  margin: 0;
}

.search-box-wrapper {
  width: 100%;
  max-width: 680px;
}

.search-box {
  width: 100%;
}

.search-box-form {
  display: flex;
  align-items: center;
  gap: 12px;
  position: relative;
}

.search-box-input {
  flex: 1;
  min-width: 0;
}

.search-box-input :deep(.el-input__wrapper) {
  padding: 4px 16px;
  height: 52px;
  border-radius: 26px !important;
  background: var(--vf-bg-elevated);
  border: 1px solid var(--vf-border);
  transition: all 0.3s ease;
}

.search-box-input :deep(.el-input__wrapper):hover,
.search-box-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--vf-accent);
  box-shadow: var(--vf-glow-amber);
}

.search-box-input :deep(.el-input__inner) {
  font-size: 16px;
  font-family: var(--vf-font-ui);
  height: 42px;
}

.search-btn {
  border-radius: 22px !important;
  padding: 0 24px !important;
  height: 44px !important;
  flex-shrink: 0;
}

.overwrite-option {
  display: flex;
  justify-content: center;
  margin-top: -16px;
  margin-bottom: 8px;
}

.overwrite-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--vf-font-ui);
  font-size: 12px;
  color: var(--vf-text-muted);
  cursor: pointer;
  user-select: none;
}

.overwrite-label:hover {
  color: var(--vf-text-secondary);
}

/* 下载路径选择器 */
.download-path-selector {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.path-option {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 6px 12px;
  border: 1px solid var(--vf-border-light);
  border-radius: var(--vf-radius-sm);
  background: var(--vf-bg-elevated);
  transition: all 0.2s ease;
  user-select: none;
}

.path-option:hover {
  border-color: var(--vf-accent-border);
  background: var(--vf-accent-soft);
}

.path-option.active {
  border-color: var(--vf-accent);
  background: var(--vf-accent-soft);
}

.path-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.path-label {
  font-family: var(--vf-font-ui);
  font-size: 12px;
  font-weight: 500;
  color: var(--vf-text-primary);
}

.path-value {
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
  max-width: 200px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.top-search-path {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 16px 12px;
  flex-wrap: wrap;
}

.top-search-path .path-option {
  padding: 4px 10px;
}

.top-search-options {
  display: flex;
  align-items: center;
  padding: 8px 16px 4px;
  gap: 8px;
}

.platform-tags {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  max-width: 680px;
}

.platform-label {
  font-family: var(--vf-font-ui);
  font-size: 12px;
  color: var(--vf-text-muted);
  letter-spacing: 0.04em;
}

.platform-list {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

.platform-tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-secondary);
  background: var(--vf-bg-panel);
  border: 1px solid var(--vf-border-light);
  border-radius: var(--vf-radius-sm);
  letter-spacing: 0.03em;
}

/* ===== 已提交布局 ===== */
.top-search-bar {
  margin-bottom: 24px;
  padding: 16px;
}

@media (max-width: 480px) {
  .top-search-bar {
    padding: 12px;
    margin-bottom: 16px;
  }
}

.top-search-form {
  display: flex;
  align-items: center;
  gap: 10px;
  position: relative;
}

.top-search-input {
  flex: 1;
  min-width: 0;
}

.top-search-input :deep(.el-input__wrapper) {
  padding: 2px 12px;
  height: 42px;
  border-radius: 21px !important;
}

.top-search-input :deep(.el-input__inner) {
  font-size: 14px;
  font-family: var(--vf-font-ui);
}

.top-search-form-btn {
  flex-shrink: 0;
  height: 42px;
  border-radius: 21px !important;
  padding: 0 20px !important;
}

@media (max-width: 480px) {
  .top-search-form-btn {
    padding: 0 14px !important;
  }
}

.section-title {
  font-family: var(--vf-font-display);
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--vf-text-primary);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--vf-border);
}

/* ===== 下载卡片 ===== */
.active-downloads {
  margin-bottom: 32px;
}

.download-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.download-card {
  padding: 16px;
}

@media (max-width: 480px) {
  .download-card {
    padding: 12px;
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.card-info {
  flex: 1;
  min-width: 0;
}

.card-title {
  font-family: var(--vf-font-ui);
  font-size: 14px;
  font-weight: 600;
  color: var(--vf-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 4px;
}

.card-url {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-status {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.card-status__text {
  font-family: var(--vf-font-ui);
  font-size: 12px;
  font-weight: 500;
  color: var(--vf-text-secondary);
}

.card-progress {
  margin-bottom: 12px;
}

.progress-msg {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  margin-top: 4px;
}

.progress-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 4px;
}

.progress-stats {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
}

.card-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* ===== 历史面板（统一使用 vf-panel 风格） ===== */
.history-section {
  flex: 1;
}

.history-panel {
  background: var(--vf-bg-panel);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  box-shadow: var(--vf-shadow-panel);
  position: relative;
  display: flex;
  flex-direction: column;
}

.history-panel__header {
  display: flex;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--vf-border);
  font-family: var(--vf-font-display);
  font-weight: 600;
  font-size: 14px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.history-panel__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.history-panel__count {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
  border: 1px solid var(--vf-border);
  padding: 2px 8px;
  border-radius: var(--vf-radius-sm);
  margin-left: 4px;
}

.history-panel__footer {
  padding: 12px 16px;
  border-top: 1px solid var(--vf-border);
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 767px) {
  .history-panel__footer {
    padding: 10px 12px;
    justify-content: center;
  }
}

/* 历史表格样式 */
.cell-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.title-text {
  font-family: var(--vf-font-ui);
  font-size: 13px;
  font-weight: 500;
  color: var(--vf-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.title-url {
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.action-btns {
  display: flex;
  gap: 4px;
  justify-content: center;
}

.empty-state {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

/* ===== 手机端历史卡片 ===== */
.history-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px;
}

.history-card {
  background: var(--vf-bg-elevated);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  padding: 10px 12px;
}

.history-card__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.history-card__info {
  flex: 1;
  min-width: 0;
}

.history-card__title {
  font-family: var(--vf-font-display);
  font-size: 13px;
  font-weight: 600;
  color: var(--vf-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-card__url {
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-card__mid {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.history-card__status {
  font-size: 11px;
  color: var(--vf-text-muted);
}

.history-card__progress {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
}

.history-card__meta {
  display: flex;
  gap: 12px;
  margin-bottom: 6px;
  font-family: var(--vf-font-mono);
  font-size: 10px;
  color: var(--vf-text-muted);
}

.history-card__actions {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
</style>
