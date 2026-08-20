import { createI18n } from 'vue-i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import zhTw from 'element-plus/es/locale/lang/zh-tw'
import enEp from 'element-plus/es/locale/lang/en'
import jaEp from 'element-plus/es/locale/lang/ja'

import zh from './locales/zh.json'
import zhTW from './locales/zh-TW.json'
import en from './locales/en.json'
import ja from './locales/ja.json'

const savedLang = localStorage.getItem('videoflow-lang') || 'zh'

export const i18n = createI18n({
  legacy: false,
  locale: savedLang,
  fallbackLocale: 'zh',
  messages: { zh, 'zh-TW': zhTW, en, ja },
})

// Element Plus 语言映射，供 App.vue 中的 el-config-provider 使用
export const elementPlusLocales: Record<string, typeof zhCn> = {
  zh: zhCn,
  'zh-TW': zhTw,
  en: enEp,
  ja: jaEp,
}
