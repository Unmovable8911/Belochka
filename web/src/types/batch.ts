// --- Batch Command domain types (mirrors the Go batchrun model) ---

type BatchRunStatus = "running" | "done"

export type RunResultStatus = "pending" | "running" | "success" | "failed"

// BatchRun is a single dispatch of one script to a set of Servers. Only the
// most recent run is retained — a new dispatch replaces the previous one.
export interface BatchRunInfo {
  id: string
  script: string
  status: BatchRunStatus
  created_at: string
}

// RunResultInfo is one Server's outcome within a Batch Run. The output is a
// single merged stream (stdout+stderr are not separable over a PTY), capped
// server-side and marked truncated when the cap was hit.
export interface RunResultInfo {
  server_id: string
  status: RunResultStatus
  exit_code?: number
  output: string
  truncated: boolean
  error?: string
  started_at?: string
  finished_at?: string
}

// BatchRunSnapshot is the REST payload for the current run.
export interface BatchRunSnapshot {
  run: BatchRunInfo
  results: RunResultInfo[]
}

// WS server→client frame types.
interface BatchOutputEvent {
  type: "output"
  server_id: string
  data: string
}

interface BatchStatusEvent {
  type: "status"
  server_id: string
  status: RunResultStatus
  exit_code?: number
  truncated?: boolean
  error?: string
  finished_at?: string
}

export type BatchServerEvent = BatchOutputEvent | BatchStatusEvent
