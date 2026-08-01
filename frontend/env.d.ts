/// <reference types="vite/client" />

declare module 'element-plus/dist/locale/zh-cn.mjs' {
  import type { Language } from 'element-plus/es/locale'
  const content: Language
  export default content
}

interface ImportMetaEnv {
  /** 后端 API 基础地址（开发态留空走 Vite 代理，生产态可显式指定） */
  readonly VITE_API_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
