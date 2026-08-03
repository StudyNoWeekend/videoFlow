import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  getComponents,
  installComponent,
  uninstallComponent,
  type ComponentInfo,
  type ComponentInstallReq,
  type ComponentUninstallReq,
} from '@/api/component'

export const useComponentStore = defineStore('component', () => {
  const components = ref<ComponentInfo[]>([])
  const loading = ref(false)

  // 按类型分组
  const componentMap = computed(() => {
    const map: Record<string, ComponentInfo> = {}
    for (const c of components.value) {
      map[c.type] = c
    }
    return map
  })

  async function loadComponents(): Promise<void> {
    loading.value = true
    try {
      components.value = await getComponents()
    } finally {
      loading.value = false
    }
  }

  async function install(data: ComponentInstallReq): Promise<string> {
    const res = await installComponent(data)
    return res.session_id
  }

  async function uninstall(data: ComponentUninstallReq): Promise<string> {
    const res = await uninstallComponent(data)
    return res.session_id
  }

  /**
   * 获取某个组件的状态
   */
  function getStatus(type: string): ComponentInfo | undefined {
    return componentMap.value[type]
  }

  return {
    components,
    loading,
    loadComponents,
    install,
    uninstall,
    getStatus,
  }
})
