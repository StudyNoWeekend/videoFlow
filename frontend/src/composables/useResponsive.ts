import { ref, onMounted, onUnmounted, computed } from 'vue'

/** 断点定义 (px) */
export const BREAKPOINTS = {
  mobile: 480,
  tablet: 768,
  desktop: 1024,
  wide: 1440,
} as const

export type BreakpointName = keyof typeof BREAKPOINTS

/**
 * 响应式工具 composable。
 * 监听 window resize，提供 `isMobile`、`isTablet`、`isDesktop` 等响应式状态，
 * 以及当前断点名 `breakpoint`。
 *
 * 用法：
 *   const { isMobile, breakpoint } = useResponsive()
 */
export function useResponsive() {
  const windowWidth = ref(window.innerWidth)

  function onResize() {
    windowWidth.value = window.innerWidth
  }

  onMounted(() => window.addEventListener('resize', onResize))
  onUnmounted(() => window.removeEventListener('resize', onResize))

  /** 当前断点名 */
  const breakpoint = computed<BreakpointName>(() => {
    const w = windowWidth.value
    if (w <= BREAKPOINTS.mobile) return 'mobile'
    if (w <= BREAKPOINTS.tablet) return 'tablet'
    if (w <= BREAKPOINTS.desktop) return 'desktop'
    return 'wide'
  })

  const isMobile = computed(() => breakpoint.value === 'mobile')
  const isTablet = computed(() => breakpoint.value === 'tablet')
  const isDesktop = computed(() => breakpoint.value === 'desktop')
  const isWide = computed(() => breakpoint.value === 'wide')

  /** 小于等于 tablet 宽度（<= 768px） */
  const isMobileOrTablet = computed(() => breakpoint.value === 'mobile' || breakpoint.value === 'tablet')

  /** 小于等于 mobile 宽度（<= 480px） */
  const isMobileOnly = computed(() => breakpoint.value === 'mobile')

  return {
    windowWidth,
    breakpoint,
    isMobile,
    isTablet,
    isDesktop,
    isWide,
    isMobileOrTablet,
    isMobileOnly,
  }
}
