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
  group_id?: string
  created_at: string
  updated_at: string
}

export interface Group {
  id: string
  name: string
  member_count: number
}

// GroupNode is a Group with its member servers attached, ready for sidebar
// rendering. Groups are flat — no nesting.
export interface GroupNode {
  id: string
  name: string
  member_count: number
  servers: ServerInfo[]
}

export interface ServerFormData {
  name: string
  host: string
  port: number
  username: string
  authType: AuthType
  password: string
  keyPath: string
  group_id?: string
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

export interface MemoryMetrics {
  total: number
  used: number
  swapTotal: number
  swapUsed: number
}

export interface DiskPartition {
  filesystem: string
  mountPoint: string
  total: number
  used: number
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
  ppid: number
  user: string
  rss: number
  cpuPct: number
  memPct: number
  etime: string
  command: string
  command_name: string
  protected: boolean
}

export interface KillResult {
  pid: number
  signal: string
  success: boolean
}

export interface SystemInfo {
  hostname: string
  kernel: string
  uptimeSec: number
  osName: string
  coreCount: number
}

export interface ServerMetrics {
  aggregate?: CPUCore
  cores?: CPUCore[]
  memory?: MemoryMetrics
  disk?: DiskMetrics
  network?: NetworkMetrics
  system?: SystemInfo
  serverId?: string
  collectedAt?: string
  partial?: boolean
}

export interface ServerInfo {
  id: string
  name: string
  host: string
  status: string
  attempts?: number
  lastError?: string
  group_id?: string
}

export interface AppConfig {
  port: number
  data_dir: string
  language: string
  log_path: string
  log_retention_days: number
}

export interface PatchConfigResponse extends AppConfig {
  restart_required?: boolean
}

export interface AuthStatus {
  needs_setup: boolean
  authenticated: boolean
}