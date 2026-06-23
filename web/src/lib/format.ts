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

export type UsageColor = 'green' | 'yellow' | 'red'

export function getUsageColor(percent: number): UsageColor {
  if (!Number.isFinite(percent) || percent < 60) return 'green'
  if (percent < 80) return 'yellow'
  return 'red'
}
