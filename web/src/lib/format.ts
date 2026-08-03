const BYTES_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB'] as const

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B'
  if (bytes < 1024) return `${Math.round(bytes)} B`

  let unitIndex = 0
  let value = bytes
  while (value >= 1024 && unitIndex < BYTES_UNITS.length - 1) {
    value /= 1024
    unitIndex++
  }

  return `${value.toFixed(1)} ${BYTES_UNITS[unitIndex]}`
}

const SPEED_UNITS = ['B/s', 'KB/s', 'MB/s', 'GB/s'] as const

export function formatNetworkSpeed(bytesPerSec: number): string {
  if (!Number.isFinite(bytesPerSec) || bytesPerSec < 0) return '0 B/s'
  if (bytesPerSec < 1000) return `${Math.round(bytesPerSec)} B/s`

  let unitIndex = 0
  let value = bytesPerSec
  while (value >= 1000 && unitIndex < SPEED_UNITS.length - 1) {
    value /= 1000
    unitIndex++
  }

  return `${value.toFixed(1)} ${SPEED_UNITS[unitIndex]}`
}

export function formatPercent(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '0.0%'
  return `${value.toFixed(1)}%`
}

export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0m'
  const totalMinutes = Math.floor(seconds / 60)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60

  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

export function formatRunDuration(startedAt?: string, finishedAt?: string): string {
  if (!startedAt || !finishedAt) return ''
  const start = new Date(startedAt).getTime()
  const end = new Date(finishedAt).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return ''

  const totalSeconds = (end - start) / 1000
  if (totalSeconds < 60) return `${Math.round(totalSeconds)}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = Math.round(totalSeconds % 60)
  if (minutes < 60) return `${minutes}m ${seconds}s`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ${minutes % 60}m`
}

type UsageColor = 'green' | 'yellow' | 'red'

export function getUsageColor(percent: number): UsageColor {
  if (!Number.isFinite(percent) || percent < 60) return 'green'
  if (percent < 80) return 'yellow'
  return 'red'
}

export const USAGE_COLOR_HEX: Record<UsageColor, string> = {
  green: "#22c55e",
  yellow: "#eab308",
  red: "#ef4444",
}
