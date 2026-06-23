import { describe, it, expect } from 'vitest'
import { formatBytes, formatNetworkSpeed, formatPercent, getUsageColor } from '@/lib/format'

describe('formatBytes', () => {
  it('formats bytes below 1 KiB as bytes', () => {
    expect(formatBytes(500)).toBe('500 B')
  })

  it('formats values in KiB range', () => {
    expect(formatBytes(1024)).toBe('1.0 KiB')
    expect(formatBytes(1536)).toBe('1.5 KiB')
  })

  it('formats values in MiB range', () => {
    expect(formatBytes(1048576)).toBe('1.0 MiB')
    expect(formatBytes(1572864)).toBe('1.5 MiB')
  })

  it('formats values in GiB range', () => {
    expect(formatBytes(1073741824)).toBe('1.0 GiB')
    expect(formatBytes(1610612736)).toBe('1.5 GiB')
  })

  it('formats values in TiB range', () => {
    expect(formatBytes(1099511627776)).toBe('1.0 TiB')
  })

  it('handles zero', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('handles negative values', () => {
    expect(formatBytes(-100)).toBe('0 B')
  })

  it('handles NaN', () => {
    expect(formatBytes(NaN)).toBe('0 B')
  })

  it('handles undefined/null by treating as invalid', () => {
    expect(formatBytes(undefined as unknown as number)).toBe('0 B')
    expect(formatBytes(null as unknown as number)).toBe('0 B')
  })

  it('handles Infinity', () => {
    expect(formatBytes(Infinity)).toBe('0 B')
  })
})

describe('formatNetworkSpeed', () => {
  it('formats low speeds as B/s', () => {
    expect(formatNetworkSpeed(500)).toBe('500 B/s')
  })

  it('formats speeds in KB/s range', () => {
    expect(formatNetworkSpeed(1000)).toBe('1.0 KB/s')
    expect(formatNetworkSpeed(1500)).toBe('1.5 KB/s')
    expect(formatNetworkSpeed(999999)).toBe('1000.0 KB/s')
  })

  it('formats speeds in MB/s range', () => {
    expect(formatNetworkSpeed(1000000)).toBe('1.0 MB/s')
    expect(formatNetworkSpeed(1500000)).toBe('1.5 MB/s')
  })

  it('formats speeds in GB/s range', () => {
    expect(formatNetworkSpeed(1000000000)).toBe('1.0 GB/s')
  })

  it('handles zero', () => {
    expect(formatNetworkSpeed(0)).toBe('0 B/s')
  })

  it('handles edge cases', () => {
    expect(formatNetworkSpeed(-1)).toBe('0 B/s')
    expect(formatNetworkSpeed(NaN)).toBe('0 B/s')
    expect(formatNetworkSpeed(undefined as unknown as number)).toBe('0 B/s')
  })
})

describe('formatPercent', () => {
  it('formats percentage with 1 decimal place', () => {
    expect(formatPercent(45.67)).toBe('45.7%')
    expect(formatPercent(100)).toBe('100.0%')
    expect(formatPercent(0)).toBe('0.0%')
  })

  it('formats fractional percentages', () => {
    expect(formatPercent(59.9)).toBe('59.9%')
    expect(formatPercent(60)).toBe('60.0%')
    expect(formatPercent(79.9)).toBe('79.9%')
    expect(formatPercent(80.1)).toBe('80.1%')
  })

  it('handles edge cases', () => {
    expect(formatPercent(-5)).toBe('0.0%')
    expect(formatPercent(NaN)).toBe('0.0%')
    expect(formatPercent(undefined as unknown as number)).toBe('0.0%')
    expect(formatPercent(Infinity)).toBe('0.0%')
  })
})

describe('getUsageColor', () => {
  it('returns green for 0-60% (exclusive of 60)', () => {
    expect(getUsageColor(0)).toBe('green')
    expect(getUsageColor(30)).toBe('green')
    expect(getUsageColor(59.9)).toBe('green')
  })

  it('returns yellow at exactly 60%', () => {
    expect(getUsageColor(60)).toBe('yellow')
  })

  it('returns yellow for 60-80% (exclusive of 80)', () => {
    expect(getUsageColor(60.1)).toBe('yellow')
    expect(getUsageColor(70)).toBe('yellow')
    expect(getUsageColor(79.9)).toBe('yellow')
  })

  it('returns red at exactly 80%', () => {
    expect(getUsageColor(80)).toBe('red')
  })

  it('returns red for 80-100%', () => {
    expect(getUsageColor(80.1)).toBe('red')
    expect(getUsageColor(90)).toBe('red')
    expect(getUsageColor(100)).toBe('red')
  })

  it('handles edge cases', () => {
    expect(getUsageColor(-1)).toBe('green')
    expect(getUsageColor(NaN)).toBe('green')
    expect(getUsageColor(undefined as unknown as number)).toBe('green')
    expect(getUsageColor(150)).toBe('red')
  })
})
