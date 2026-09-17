import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import DownloadsView from '../DownloadsView.vue'
import type { Download, DownloadListRes } from '@/api/download'

const { listDownloads } = vi.hoisted(() => ({
  listDownloads: vi.fn<(page: number, pageSize: number) => Promise<DownloadListRes>>(),
}))

vi.mock('@/api/download', () => ({
  createDownload: vi.fn(),
  listDownloads,
  cancelDownload: vi.fn(),
  deleteDownload: vi.fn(),
}))

// Telegram 登录状态查询同样依赖 axios 实例，这里一并桩掉，
// 避免在 localStorage 桩就绪前触发 i18n 初始化
vi.mock('@/api/telegram', () => ({
  getTelegramStatus: vi.fn().mockResolvedValue({ authenticated: true }),
  startTelegramQRLogin: vi.fn(),
  getTelegramQRCode: vi.fn(),
  submitTelegram2FA: vi.fn(),
  logoutTelegram: vi.fn(),
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    setting: { video_dir: '/videos', output_dir: '/output' },
    loadSettings: vi.fn().mockResolvedValue(undefined),
  }),
}))

// jsdom 未实现 ResizeObserver，Element Plus 表格挂载时会用到
if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
}

// jsdom 环境下 localStorage 不可用，先构造 mock
function makeStorage(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    key: (i: number) => Array.from(store.keys())[i] ?? null,
    removeItem: (k: string) => void store.delete(k),
    setItem: (k: string, v: string) => void store.set(k, String(v)),
  } as Storage
}

function makeRow(overrides: Partial<Download> = {}): Download {
  return {
    id: 'dl-1',
    url: 'https://example.com/video',
    platform: 'yt-dlp',
    status: 'completed',
    progress: 100,
    overwrite: false,
    created_at: 1700000000,
    updated_at: 1700000000,
    ...overrides,
  }
}

function emptyRes(): DownloadListRes {
  return { list: [], total: 0, page: 1, page_size: 10 }
}

function oneRowRes(): DownloadListRes {
  return { list: [makeRow()], total: 1, page: 1, page_size: 10 }
}

let i18n: any

beforeEach(async () => {
  vi.stubGlobal('localStorage', makeStorage())
  vi.resetModules()
  i18n = (await import('@/i18n')).i18n
  listDownloads.mockReset()
})

function mountView() {
  return mount(DownloadsView, {
    global: {
      plugins: [ElementPlus, i18n],
      components: { ...ElementPlusIconsVue },
    },
  })
}

describe('DownloadsView 空数据布局', () => {
  it('后端返回空列表时展示初始搜索页，而不是已提交态', async () => {
    listDownloads.mockResolvedValue(emptyRes())

    const wrapper = mountView()
    await flushPromises()

    expect(listDownloads).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.search-landing').exists()).toBe(true)
    expect(wrapper.find('.top-search-bar').exists()).toBe(false)
    expect(wrapper.find('.history-panel').exists()).toBe(false)
  })

  it('后端返回下载记录时展示已提交态', async () => {
    listDownloads.mockResolvedValue(oneRowRes())

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.search-landing').exists()).toBe(false)
    expect(wrapper.find('.top-search-bar').exists()).toBe(true)
    expect(wrapper.find('.history-panel').exists()).toBe(true)
  })

  it('列表刷新后返回空数据时，页面实时切回初始搜索页', async () => {
    listDownloads.mockResolvedValueOnce(oneRowRes())

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('.history-panel').exists()).toBe(true)
    expect(wrapper.find('.search-landing').exists()).toBe(false)

    // 记录被清空后的一次列表刷新（排序变化会触发 loadDownloads）
    listDownloads.mockResolvedValue(emptyRes())
    const table = wrapper.findComponent({ name: 'ElTable' })
    expect(table.exists()).toBe(true)
    table.vm.$emit('sort-change', { prop: 'created_at', order: 'ascending' })
    await flushPromises()

    expect(listDownloads).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.search-landing').exists()).toBe(true)
    expect(wrapper.find('.history-panel').exists()).toBe(false)
  })
})
