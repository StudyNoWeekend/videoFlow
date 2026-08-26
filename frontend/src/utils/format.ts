/**
 * 格式化文件大小
 * @param size 字节数
 */
export function formatFileSize(size?: number): string {
  if (size === undefined || size === null || size < 0) {
    return '-'
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let index = 0
  let value = size
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value.toFixed(2)} ${units[index]}`
}

/**
 * 格式化下载速度（字节/秒 -> MB/s）
 * @param speed 字节/秒
 */
export function formatSpeed(speed?: number): string {
  if (speed === undefined || speed === null || speed <= 0) {
    return '-'
  }
  if (speed >= 1024 * 1024) {
    return `${(speed / (1024 * 1024)).toFixed(1)} MB/s`
  }
  if (speed >= 1024) {
    return `${(speed / 1024).toFixed(1)} KB/s`
  }
  return `${speed} B/s`
}

/**
 * 格式化时长（秒）
 * @param seconds 秒数
 */
export function formatDuration(seconds?: number): string {
  if (seconds === undefined || seconds === null || seconds <= 0) {
    return '-'
  }
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const parts = []
  if (h > 0) {
    parts.push(`${h}小时`)
  }
  if (m > 0 || h > 0) {
    parts.push(`${m}分钟`)
  }
  parts.push(`${s}秒`)
  return parts.join('')
}

/**
 * 格式化时间戳
 * @param timestamp 秒级时间戳
 */
export function formatTime(timestamp?: number): string {
  if (timestamp === undefined || timestamp === null) {
    return '-'
  }
  const date = new Date(timestamp * 1000)
  return date.toLocaleString('zh-CN')
}

/**
 * 格式化字幕时间（秒 -> HH:MM:SS,mmm）
 * @param seconds 秒数
 */
export function formatSrtTime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const ms = Math.floor((seconds % 1) * 1000)
  return `${pad(h)}:${pad(m)}:${pad(s)},${pad(ms, 3)}`
}

function pad(num: number, len = 2): string {
  return num.toString().padStart(len, '0')
}
