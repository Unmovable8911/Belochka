export type AuthType = "password" | "key"

export interface Server {
  id: string
  name: string
  host: string
  port: number
  auth_type: AuthType
  username: string
  key_path?: string
  host_key_fingerprint?: string
  created_at: string
  updated_at: string
}

export interface ServerFormData {
  name: string
  host: string
  port: number
  username: string
  authType: AuthType
  password: string
  keyPath: string
}

export interface TestResult {
  fingerprint: string
}

export interface CronEntry {
  minute: string
  hour: string
  dayOfMonth: string
  month: string
  dayOfWeek: string
  command: string
  enabled: boolean
  raw: string
}

export interface CronResult {
  entries: CronEntry[]
  passthroughs: string[]
}

export interface CronRunResult {
  exitCode: number
  output: string
}

// --- WebSocket metric domain types (mirrors Go model types for the frontend) ---

export interface CPUCore {
  name?: string
  usagePercent: number
}

export interface CPUMetrics {
  aggregate: CPUCore
  cores: CPUCore[]
}

export interface MemoryMetrics {
  total: number
  used: number
  available: number
  swapTotal: number
  swapUsed: number
}

export interface DiskPartition {
  filesystem: string
  mountPoint: string
  total: number
  used: number
  available: number
}

export interface DiskMetrics {
  partitions: DiskPartition[]
}

export interface NetworkInterface {
  name: string
  rxBytesPerSec: number
  txBytesPerSec: number
}

export interface NetworkMetrics {
  interfaces: NetworkInterface[]
}

export interface Process {
  pid: number
  user: string
  cpuPct: number
  memPct: number
  command: string
}

export interface ProcessMetrics {
  processes: Process[]
}

export interface SystemInfo {
  hostname: string
  kernel: string
  uptimeSec: number
  osName: string
  coreCount: number
}

export interface ServerMetrics {
  cpu: CPUMetrics
  memory: MemoryMetrics
  disk: DiskMetrics
  network: NetworkMetrics
  process: ProcessMetrics
  system: SystemInfo
}

export interface ServerInfo {
  id: string
  name: string
  host: string
  status: string
  attempts?: number
  lastError?: string
}
