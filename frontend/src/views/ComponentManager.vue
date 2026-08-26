<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useComponentStore } from '@/stores/component'
import { getActiveSession, getInstallHistory } from '@/api/component'
import type { ComponentInfo, ComponentInstallReq } from '@/api/component'
import VfListPanel from '@/components/VfListPanel.vue'

const { t } = useI18n()

const componentStore = useComponentStore()
const refreshing = ref(false)

// Dialog states
const installDialogVisible = ref(false)
const installComponentType = ref<string>('')
const installSessionId = ref<string>('')
const installLogs = ref<string[]>([])
const installStatus = ref<string>('')

// SSE ref
let eventSource: EventSource | null = null

// Log dialog visible
const logDialogVisible = ref(false)

async function handleRefresh(): Promise<void> {
  refreshing.value = true
  await componentStore.loadComponents()
  await reconnectActiveSessions()
  refreshing.value = false
}

function statusColor(status: string): string {
  switch (status) {
    case 'installed':
      return 'var(--vf-green)'
    case 'missing':
      return 'var(--vf-text-muted)'
    case 'installing':
      return 'var(--vf-accent)'
    case 'error':
      return 'var(--vf-red)'
    default:
      return 'var(--vf-text-muted)'
  }
}

function statusIcon(status: string): string {
  switch (status) {
    case 'installed':
      return 'SuccessFilled'
    case 'missing':
      return 'CircleCloseFilled'
    case 'installing':
      return 'Loading'
    case 'error':
      return 'WarningFilled'
    default:
      return 'QuestionFilled'
  }
}

async function handleInstall(comp: ComponentInfo): Promise<void> {
  // Lada uses disposable containers - no config needed, just pull the image directly
  if (comp.type === 'lada' || comp.type === 'video2x') {
    updateLocalComponentStatus(comp.type, 'installing')
    installLogs.value = [t('components.install.preparing_lada')]
    installStatus.value = 'running'

    const data: ComponentInstallReq = {
      component_type: comp.type as ComponentInstallReq['component_type'],
    }
    try {
      const res = await componentStore.install(data)
      installSessionId.value = res
      startSSE(res)
    } catch {
      installLogs.value.push(t('components.install.start_failed'))
      installStatus.value = 'failed'
    }
    return
  }

  installComponentType.value = comp.type
  installLogs.value = []
  installStatus.value = ''
  installDialogVisible.value = true
}

async function confirmInstall(): Promise<void> {
  // 立即更新本地组件状态为 installing
  updateLocalComponentStatus(installComponentType.value, 'installing')

  installDialogVisible.value = false
  installLogs.value = [t('components.install.preparing')]
  installStatus.value = 'running'

  const data: ComponentInstallReq = {
    component_type: installComponentType.value as ComponentInstallReq['component_type'],
  }

  try {
    const res = await componentStore.install(data)
    installSessionId.value = res
    startSSE(res)
  } catch {
    installLogs.value.push(t('components.install.start_failed'))
    installStatus.value = 'failed'
  }
}

/** 更新 store 中指定组件的状态（不请求后端，仅本地修改） */
function updateLocalComponentStatus(type: string, status: string): void {
  const list = componentStore.components
  const comp = list.find((c) => c.type === type)
  if (comp) {
    (comp as Record<string, any>).status = status
  }
}

/** 点击卡片查看日志 */
async function handleShowLog(comp: ComponentInfo): Promise<void> {
  // 如果已经有本地日志，直接展示
  if (installComponentType.value === comp.type && installLogs.value.length > 0) {
    logDialogVisible.value = true
    return
  }
  // 否则尝试从后端获取活跃 session
  try {
    const sessionId = await getActiveSession(comp.type)
    if (sessionId) {
      installComponentType.value = comp.type
      installSessionId.value = sessionId
      installLogs.value = [t('components.install.reconnecting')]
      installStatus.value = 'running'
      startSSE(sessionId)
      return
    }
  } catch {
    // ignore
  }

  // 没有活跃 session，尝试加载历史安装日志
  try {
    const history = await getInstallHistory(comp.type)
    if (history && history.length > 0) {
      installComponentType.value = comp.type
      installLogs.value = []
      let finalStatus = ''
      for (const event of history) {
        if (event.log) {
          installLogs.value.push(`${event.step}: ${event.log}`)
        }
        finalStatus = event.status
      }
      installStatus.value = finalStatus
      logDialogVisible.value = true
      return
    }
  } catch {
    // ignore
  }

  // 既没有活跃 session 也没有历史日志
  if (comp.status === 'installing') {
    updateLocalComponentStatus(comp.type, 'error')
    ElMessage.info(t('components.install.interrupted'))
  } else {
    ElMessage.info(t('components.install.no_logs'))
  }
}

function startSSE(sessionId: string): void {
  const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
  const url = `${baseUrl}/api/v1/components/install/progress/${sessionId}`

  if (eventSource) {
    eventSource.close()
  }

  installSessionId.value = sessionId
  installLogs.value = [...installLogs.value, t('components.install.connecting')]
  installStatus.value = 'running'
  logDialogVisible.value = true

  eventSource = new EventSource(url)
  eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data)
    // 跳过初始连接事件
    if (data.status === 'connected') {
      installLogs.value = [...installLogs.value, t('components.install.connected')]
      return
    }
    if (data.step) {
      installLogs.value = [...installLogs.value, `${data.step}: ${data.log}`]
    }
    installStatus.value = data.status

    if (data.status === 'completed') {
      ElMessage.success(t('components.install.completed'))
      eventSource?.close()
      eventSource = null
      componentStore.loadComponents()
    } else if (data.status === 'failed') {
      ElMessage.error(t('components.install.failed', { error: data.error || t('components.install.unknown_error') }))
      eventSource?.close()
      eventSource = null
      componentStore.loadComponents()
    }
  }
  eventSource.onerror = () => {
    installLogs.value = [...installLogs.value, t('components.install.sse_disconnected')]
    installStatus.value = 'error'
    eventSource?.close()
    eventSource = null
  }
}

async function handleReinstall(comp: ComponentInfo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('components.dialog.reinstall.confirm', { name: comp.name }),
      t('components.dialog.reinstall.title'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    updateLocalComponentStatus(comp.type, 'installing')
    installComponentType.value = comp.type
    installLogs.value = [t('components.install.preparing_reinstall')]
    installStatus.value = 'running'

    const data: ComponentInstallReq = {
      component_type: comp.type as ComponentInstallReq['component_type'],
    }
    const res = await componentStore.install(data)
    installSessionId.value = res
    startSSE(res)
  } catch {
    // cancelled
  }
}

async function handleUninstall(comp: ComponentInfo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('components.dialog.uninstall.confirm', { name: comp.name }),
      t('components.dialog.uninstall.title'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
    )
    updateLocalComponentStatus(comp.type, 'installing')
    const res = await componentStore.uninstall({ component_type: comp.type as ComponentInstallReq['component_type'] })
    installSessionId.value = res
    startSSE(res)
  } catch {
    // cancelled
  }
}

/** 加载后检查是否有活跃 session，自动恢复连接 */
async function reconnectActiveSessions(): Promise<void> {
  const installing = componentStore.components.filter((c) => c.status === 'installing')
  for (const comp of installing) {
    try {
      const sessionId = await getActiveSession(comp.type)
      if (sessionId) {
        installComponentType.value = comp.type
        installSessionId.value = sessionId
        installLogs.value = [t('components.install.reconnecting')]
        installStatus.value = 'running'
        startSSE(sessionId)
        return // 只恢复第一个
      }
    } catch {
      // ignore
    }
  }

  // 服务重启后，installing 状态的组件如果后端没有活跃 session，标记为 error
  for (const comp of componentStore.components) {
    if (comp.status === 'installing') {
      if (installStatus.value !== 'running') {
        updateLocalComponentStatus(comp.type, 'error')
      }
    }
  }
}

onMounted(async () => {
  await componentStore.loadComponents()
  await reconnectActiveSessions()
})
</script>

<template>
  <div class="component-manager responsive-page">
    <VfListPanel
      :title="$t('components.title')"
      led-color="amber"
      :led-pulse="true"
      :show-pagination="false"
      :show-polling="false"
      @refresh="handleRefresh"
    >
      <template #toolbar-right>
        <el-button :loading="refreshing" @click="handleRefresh">
          <el-icon><Refresh /></el-icon>{{ $t('components.refresh') }}
        </el-button>
      </template>

      <div class="cm-grid">
        <div v-for="comp in componentStore.components" :key="comp.type" class="cm-card">
          <div class="cm-card__header">
            <div class="cm-card__icon" :style="{ borderColor: statusColor(comp.status) }">
              <el-icon :size="24" :color="statusColor(comp.status)">
                <component :is="statusIcon(comp.status)" />
              </el-icon>
            </div>
            <div class="cm-card__info">
              <div class="cm-card__name">{{ comp.name }}</div>
              <div class="cm-card__desc">{{ comp.description }}</div>
            </div>
          </div>

          <div class="cm-card__status">
            <span class="cm-badge" :class="'cm-badge--' + comp.status">
              <span class="cm-badge__dot" :style="{ background: statusColor(comp.status) }"></span>
              <span>{{ $t('components.status.' + comp.status) }}</span>
            </span>
            <span v-if="comp.version" class="cm-version">{{ comp.version }}</span>
          </div>

          <div class="cm-card__actions">
            <!-- Whisper ASR & yt-dlp: 仅展示检测状态，安装由用户在外部自行完成 -->
            <template v-if="comp.type === 'whisper_asr'">
              <el-text v-if="comp.status === 'missing' || comp.status === 'error'">{{ $t('components.whisper_not_deployed') }}</el-text>
            </template>
            <template v-else-if="comp.type === 'yt-dlp'">
              <el-text v-if="comp.status === 'missing' || comp.status === 'error'">{{ $t('components.ytdlp_not_deployed') }}</el-text>
            </template>
            <template v-else>
              <template v-if="comp.status === 'installing'">
                <el-button type="primary" size="small" @click="handleShowLog(comp)">
                  {{ $t('components.btn.progress') }}
                </el-button>
                <el-button type="warning" size="small" @click="handleReinstall(comp)">
                  {{ $t('components.btn.reinstall') }}
                </el-button>
              </template>
              <template v-else-if="comp.status === 'missing' || comp.status === 'error'">
                <el-button type="primary" size="small" @click="handleInstall(comp)">
                  {{ $t('components.btn.install') }}
                </el-button>
                <el-button v-if="comp.status === 'error'" type="warning" size="small" @click="handleReinstall(comp)">
                  {{ $t('components.btn.reinstall') }}
                </el-button>
              </template>
              <template v-else-if="comp.status === 'installed'">
                <el-button type="warning" size="small" @click="handleReinstall(comp)">
                  {{ $t('components.btn.reinstall') }}
                </el-button>
                <el-button type="danger" size="small" @click="handleUninstall(comp)">
                  {{ $t('components.btn.uninstall') }}
                </el-button>
                <el-button size="small" @click="handleShowLog(comp)">
                  {{ $t('components.btn.logs') }}
                </el-button>
              </template>
            </template>
          </div>
        </div>
      </div>
    </VfListPanel>

    <!-- Install Config Dialog -->
    <el-dialog
      v-model="installDialogVisible"
      :title="$t('components.dialog.install.title', { name: installComponentType })"
      width="500px"
    >
      <el-form label-width="140px" v-if="installComponentType === 'lada' || installComponentType === 'video2x'">
        <el-text>{{ installComponentType === 'lada' ? 'Lada' : 'Video2X' }} 将直接拉取镜像运行，无需额外配置。</el-text>
      </el-form>

      <template #footer>
        <el-button @click="installDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmInstall">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Installation Log Dialog -->
    <el-dialog
      v-model="logDialogVisible"
      :title="`${$t('components.log.title')} - ${installComponentType}`"
      width="700px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <div class="log-terminal" ref="logTerminalRef">
        <div v-for="(line, i) in installLogs" :key="i" class="log-line">{{ line }}</div>
        <div v-if="installStatus === 'running'" class="log-line log-line--cursor">...</div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.component-manager {
  min-height: 100%;
}

.cm-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
  padding: 16px;
}

@media (max-width: 767px) {
  .cm-grid {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 12px;
  }
}

.cm-card {
  background: var(--vf-bg-elevated);
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  padding: 16px;
  transition: all 0.2s ease;
}

.cm-card:hover {
  border-color: var(--vf-border-active);
  box-shadow: 0 0 0 1px var(--vf-border-active);
}

.cm-card__header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.cm-card__icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--vf-border);
  border-radius: var(--vf-radius);
  flex-shrink: 0;
}

.cm-card__name {
  font-family: var(--vf-font-display);
  font-weight: 600;
  font-size: 15px;
  margin-bottom: 4px;
}

.cm-card__desc {
  font-size: 12px;
  color: var(--vf-text-muted);
  line-height: 1.4;
}

.cm-card__status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding: 8px 0;
  border-top: 1px solid var(--vf-border);
  border-bottom: 1px solid var(--vf-border);
}

.cm-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
}

.cm-badge__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.cm-badge--installed {
  color: var(--vf-green);
}
.cm-badge--missing {
  color: var(--vf-text-muted);
}
.cm-badge--installing {
  color: var(--vf-accent);
}
.cm-badge--error {
  color: var(--vf-red);
}

.cm-version {
  font-family: var(--vf-font-mono);
  font-size: 11px;
  color: var(--vf-text-muted);
}

.cm-card__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.log-terminal {
  background: #1a1a2e;
  color: #00ff88;
  font-family: 'Cascadia Code', 'Fira Code', monospace;
  font-size: 12px;
  padding: 12px;
  border-radius: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.log-line {
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line--cursor::after {
  content: '';
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}
</style>
