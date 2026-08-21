import { describe, it, expect, beforeEach, vi } from 'vitest'

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

let i18n: any
let elementPlusLocales: any

beforeEach(async () => {
  vi.stubGlobal('localStorage', makeStorage())
  vi.resetModules()
  const mod = await import('@/i18n')
  i18n = mod.i18n
  elementPlusLocales = mod.elementPlusLocales
})

describe('i18n 语言切换', () => {
  it('默认语言为中文', () => {
    expect(i18n.global.locale.value).toBe('zh')
  })

  it('从 localStorage 读取保存的语言', async () => {
    localStorage.setItem('videoflow-lang', 'ja')
    vi.resetModules()
    const mod = await import('@/i18n')
    expect(mod.i18n.global.locale.value).toBe('ja')
  })

  it('四个语言均可切换且文案正确', () => {
    const t = i18n.global.t
    const cases: Array<[string, string]> = [
      ['zh-TW', '影片資源'],
      ['en', 'Videos'],
      ['ja', '動画リソース'],
      ['zh', '视频资源'],
    ]
    for (const [lang, expected] of cases) {
      i18n.global.locale.value = lang
      expect(t('nav.videos')).toBe(expected)
      expect(elementPlusLocales[lang]).toBeTruthy()
    }
  })

  it('v0.3.1 新增 key 在四个语言中均不再返回原始 key', () => {
    const keys = [
      'settings.label.output_dir',
      'videos.batch_delete',
      'videos.batch_delete.confirm',
      'videos.batch_delete.delete_files_label',
      'videos.batch_delete.success',
      'videos.batch_delete.success_with_files',
      'videos.component_missing',
      'tasks.batch_delete',
      'tasks.batch_delete.confirm',
      'tasks.batch_delete.delete_files_label',
      'tasks.batch_delete.success',
      'tasks.delete.delete_files_label',
    ]
    for (const lang of ['zh', 'zh-TW', 'en', 'ja']) {
      i18n.global.locale.value = lang
      for (const key of keys) {
        const value = i18n.global.t(key)
        expect(value, `${lang} 的 ${key}`).not.toBe(key)
        expect(value).toBeTruthy()
      }
    }
  })
})
