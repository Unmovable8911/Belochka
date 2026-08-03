import { describe, it, expect } from "vitest"
import { processReducer, initialProcessState } from "./useProcesses"
import type { Process } from "@/types/server"

function makeProcess(overrides: Partial<Process> = {}): Process {
  return {
    pid: overrides.pid ?? 1,
    ppid: overrides.ppid ?? 0,
    user: overrides.user ?? "root",
    rss: overrides.rss ?? 10000,
    cpuPct: overrides.cpuPct ?? 0.5,
    memPct: overrides.memPct ?? 1.0,
    etime: overrides.etime ?? "01:00:00",
    command: overrides.command ?? "systemd",
    command_name: overrides.command_name ?? "systemd",
    protected: overrides.protected ?? false,
  }
}

describe("processReducer", () => {
  it("FETCH_START sets loading and clears error", () => {
    const prev = { ...initialProcessState, error: "previous error", fetched: true }
    const next = processReducer(prev, { type: "FETCH_START" })
    expect(next.loading).toBe(true)
    expect(next.error).toBeNull()
    expect(next.fetched).toBe(true)
  })

  it("FETCH_SUCCESS stores data and clears loading", () => {
    const data = [makeProcess({ pid: 1 }), makeProcess({ pid: 2 })]
    const prev = { ...initialProcessState, loading: true }
    const next = processReducer(prev, { type: "FETCH_SUCCESS", data })
    expect(next.loading).toBe(false)
    expect(next.error).toBeNull()
    expect(next.processes).toEqual(data)
    expect(next.fetched).toBe(true)
  })

  it("FETCH_SUCCESS replaces existing processes", () => {
    const prev = { ...initialProcessState, processes: [makeProcess({ pid: 1 })], fetched: true }
    const data = [makeProcess({ pid: 2 }), makeProcess({ pid: 3 })]
    const next = processReducer(prev, { type: "FETCH_SUCCESS", data })
    expect(next.processes).toEqual(data)
  })

  it("FETCH_ERROR sets error and clears loading", () => {
    const prev = { ...initialProcessState, loading: true }
    const next = processReducer(prev, { type: "FETCH_ERROR", error: "SSH failed" })
    expect(next.loading).toBe(false)
    expect(next.error).toBe("SSH failed")
  })

  it("SET_AUTO_REFRESH toggles autoRefresh", () => {
    const prev = { ...initialProcessState, autoRefresh: true }
    let next = processReducer(prev, { type: "SET_AUTO_REFRESH", enabled: false })
    expect(next.autoRefresh).toBe(false)
    next = processReducer(next, { type: "SET_AUTO_REFRESH", enabled: true })
    expect(next.autoRefresh).toBe(true)
  })

  it("initial state has autoRefresh true", () => {
    expect(initialProcessState.autoRefresh).toBe(true)
    expect(initialProcessState.processes).toEqual([])
    expect(initialProcessState.loading).toBe(false)
    expect(initialProcessState.error).toBeNull()
    expect(initialProcessState.fetched).toBe(false)
  })

  it("returns same state for unknown action", () => {
    const prev = initialProcessState
    // @ts-expect-error testing unknown action
    const next = processReducer(prev, { type: "UNKNOWN" })
    expect(next).toBe(prev)
  })
})
