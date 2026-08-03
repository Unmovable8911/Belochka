import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, waitFor, act } from "@testing-library/react"
import {
  batchRunReducer,
  initialBatchRunState,
  selectionState,
  toggleGroupSelection,
  toggleAllSelection,
  useBatchRun,
} from "./useBatchRun"
import type { BatchRunState } from "./useBatchRun"
import type { BatchRunInfo, RunResultInfo } from "../types/batch"

// --- Mocks ---

vi.mock("@/api/client", () => ({
  getCurrentBatchRun: vi.fn(),
  createBatchRun: vi.fn(),
  cancelBatchRun: vi.fn(),
}))

import { getCurrentBatchRun, createBatchRun, cancelBatchRun } from "@/api/client"

const mockGetCurrent = getCurrentBatchRun as ReturnType<typeof vi.fn>
const mockCreateBatchRun = createBatchRun as ReturnType<typeof vi.fn>
const mockCancelBatchRun = cancelBatchRun as ReturnType<typeof vi.fn>

// MockWebSocket captures instances so tests can script frames.
class MockWebSocket {
  static instances: MockWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  url: string
  readyState = MockWebSocket.CONNECTING
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = 3
  }

  open() {
    this.readyState = 1
    this.onopen?.()
  }

  emit(data: string) {
    this.onmessage?.({ data })
  }

  closeFromServer() {
    this.readyState = 3
    this.onclose?.()
  }
}

function makeRun(overrides: Partial<BatchRunInfo> = {}): BatchRunInfo {
  return {
    id: "run-1",
    script: "echo hi",
    status: "running",
    created_at: "2026-08-03T12:00:00Z",
    ...overrides,
  }
}

function makeResult(overrides: Partial<RunResultInfo> = {}): RunResultInfo {
  return {
    server_id: "s1",
    status: "pending",
    output: "",
    truncated: false,
    ...overrides,
  }
}

// --- selectionState ---

describe("selectionState", () => {
  it("is unchecked when nothing is selected", () => {
    expect(selectionState(["a", "b"], new Set())).toBe("unchecked")
  })

  it("is checked when every member is selected", () => {
    expect(selectionState(["a", "b"], new Set(["a", "b"]))).toBe("checked")
  })

  it("is indeterminate when only some members are selected", () => {
    expect(selectionState(["a", "b"], new Set(["a"]))).toBe("indeterminate")
  })

  it("is unchecked for an empty group", () => {
    expect(selectionState([], new Set())).toBe("unchecked")
  })
})

// --- toggleGroupSelection ---

describe("toggleGroupSelection", () => {
  it("selects all members from an unchecked group", () => {
    const next = toggleGroupSelection(["a", "b"], new Set())
    expect([...next]).toEqual(["a", "b"])
  })

  it("selects all members from an indeterminate group", () => {
    const next = toggleGroupSelection(["a", "b"], new Set(["a"]))
    expect([...next]).toEqual(["a", "b"])
  })

  it("deselects all members from a fully selected group", () => {
    const next = toggleGroupSelection(["a", "b"], new Set(["a", "b"]))
    expect(next.size).toBe(0)
  })
})

// --- toggleAllSelection ---

describe("toggleAllSelection", () => {
  it("selects every server when none are selected", () => {
    const next = toggleAllSelection(["a", "b", "c"], new Set())
    expect([...next]).toEqual(["a", "b", "c"])
  })

  it("deselects every server when all are selected", () => {
    const next = toggleAllSelection(["a", "b", "c"], new Set(["a", "b", "c"]))
    expect(next.size).toBe(0)
  })

  it("selects every server when only some are selected", () => {
    const next = toggleAllSelection(["a", "b", "c"], new Set(["a"]))
    expect([...next]).toEqual(["a", "b", "c"])
  })
})

// --- Reducer ---

function apply(action: Parameters<typeof batchRunReducer>[1], state: BatchRunState = initialBatchRunState) {
  return batchRunReducer(state, action)
}

describe("batchRunReducer", () => {
  it("TOGGLE_SERVER adds and removes a server", () => {
    const added = apply({ type: "TOGGLE_SERVER", serverId: "s1" })
    expect(added.selected).toEqual(["s1"])
    const removed = apply({ type: "TOGGLE_SERVER", serverId: "s1" }, added)
    expect(removed.selected).toEqual([])
  })

  it("TOGGLE_GROUP selects a whole group", () => {
    const state = apply({ type: "TOGGLE_GROUP", memberIds: ["s1", "s2"] })
    expect(state.selected).toEqual(["s1", "s2"])
  })

  it("TOGGLE_ALL selects every server", () => {
    const state = apply({ type: "TOGGLE_ALL", serverIds: ["s1", "s2"] })
    expect(state.selected).toEqual(["s1", "s2"])
  })

  it("OPEN with a done run opens compose mode with the script prefilled", () => {
    const run = makeRun({ status: "done", script: "df -h" })
    const state = apply({ type: "OPEN", run, results: [makeResult({ status: "success" })] })
    expect(state.mode).toBe("compose")
    expect(state.script).toBe("df -h")
    expect(state.run).toBe(run)
    expect(state.results).toHaveLength(1)
    expect(state.selected).toEqual([])
  })

  it("OPEN with a running run jumps to the results view", () => {
    const run = makeRun()
    const state = apply({ type: "OPEN", run, results: [makeResult()] })
    expect(state.mode).toBe("results")
  })

  it("OPEN with no run keeps compose mode and an empty script", () => {
    const state = apply({ type: "OPEN", run: null, results: [] })
    expect(state.mode).toBe("compose")
    expect(state.script).toBe("")
    expect(state.run).toBeNull()
  })

  it("DISPATCH_SUCCESS switches to results and resets error", () => {
    const state = apply(
      { type: "DISPATCH_ERROR", code: "batch_run_in_progress", message: "busy" },
      { ...initialBatchRunState, dispatching: true }
    )
    const success = apply(
      { type: "DISPATCH_SUCCESS", run: makeRun(), results: [makeResult({ status: "running" })] },
      state
    )
    expect(success.mode).toBe("results")
    expect(success.dispatching).toBe(false)
    expect(success.dispatchError).toBeNull()
  })

  it("WS_EVENT output appends to the right server's output", () => {
    const state = apply({ type: "OPEN", run: makeRun(), results: [makeResult({ server_id: "s1" }), makeResult({ server_id: "s2" })] })
    const next = apply(
      { type: "WS_EVENT", event: { type: "output", server_id: "s2", data: "hi" } },
      state
    )
    expect(next.results[0].output).toBe("")
    expect(next.results[1].output).toBe("hi")
  })

  it("WS_EVENT status updates the result and preserves unspecified fields", () => {
    const state = apply({ type: "OPEN", run: makeRun(), results: [makeResult({ status: "running", output: "x" })] })
    const next = apply(
      {
        type: "WS_EVENT",
        event: {
          type: "status",
          server_id: "s1",
          status: "failed",
          exit_code: 3,
          error: "exit status 3",
          finished_at: "2026-08-03T12:01:00Z",
        },
      },
      state
    )
    expect(next.results[0].status).toBe("failed")
    expect(next.results[0].exit_code).toBe(3)
    expect(next.results[0].error).toBe("exit status 3")
    expect(next.results[0].finished_at).toBe("2026-08-03T12:01:00Z")
    expect(next.results[0].output).toBe("x")
  })

  it("SET_MODE switches views", () => {
    const state = apply({ type: "SET_MODE", mode: "results" })
    expect(state.mode).toBe("results")
  })
})

// --- Hook ---

describe("useBatchRun", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    MockWebSocket.instances = []
    vi.stubGlobal("WebSocket", MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("open() fetches the current run and a running run lands in results mode with the WS connected", async () => {
    mockGetCurrent.mockResolvedValue({ run: makeRun(), results: [makeResult({ status: "running" })] })
    const { result } = renderHook(() => useBatchRun())

    await act(async () => {
      await result.current.open()
    })

    expect(result.current.state.mode).toBe("results")
    expect(result.current.state.run?.id).toBe("run-1")

    // The WS connects to the running run.
    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    expect(MockWebSocket.instances[0].url).toContain("/api/ws/batch-runs/run-1")
  })

  it("runScript dispatches, switches to results, and streams WS events into state", async () => {
    mockGetCurrent.mockResolvedValue(null)
    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ status: "pending" })],
    })
    const { result } = renderHook(() => useBatchRun())

    await act(async () => {
      await result.current.open()
      await result.current.runScript("echo hi", ["s1"])
    })

    expect(result.current.state.mode).toBe("results")
    expect(result.current.state.run?.id).toBe("run-1")
    expect(mockCreateBatchRun).toHaveBeenCalledWith("echo hi", ["s1"])

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    const ws = MockWebSocket.instances[0]
    act(() => {
      ws.open()
      ws.emit(JSON.stringify({ type: "output", server_id: "s1", data: "hello" }))
      ws.emit(
        JSON.stringify({
          type: "status",
          server_id: "s1",
          status: "success",
          exit_code: 0,
          finished_at: "2026-08-03T12:01:00Z",
        })
      )
    })
    expect(result.current.state.results[0].output).toBe("hello")
    expect(result.current.state.results[0].status).toBe("success")
    expect(result.current.state.results[0].exit_code).toBe(0)
  })

  it("runScript surfaces a dispatch conflict as dispatchError", async () => {
    mockGetCurrent.mockResolvedValue(null)
    const err = new Error("Another batch run is already in progress") as Error & { code: string }
    err.code = "batch_run_in_progress"
    mockCreateBatchRun.mockRejectedValue(err)

    const { result } = renderHook(() => useBatchRun())
    await act(async () => {
      await result.current.open()
      await result.current.runScript("echo hi", ["s1"])
    })

    expect(result.current.state.dispatchError?.code).toBe("batch_run_in_progress")
    expect(result.current.state.mode).toBe("compose")
  })

  it("sendInput writes an input frame to the socket", async () => {
    mockGetCurrent.mockResolvedValue({ run: makeRun(), results: [makeResult({ status: "running" })] })
    const { result } = renderHook(() => useBatchRun())

    await act(async () => {
      await result.current.open()
    })
    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    act(() => {
      MockWebSocket.instances[0].open()
    })

    act(() => {
      result.current.sendInput("s1", "y\n")
    })
    expect(MockWebSocket.instances[0].sent).toEqual([
      JSON.stringify({ type: "input", server_id: "s1", data: "y\n" }),
    ])
  })

  it("cancel() calls the cancel endpoint", async () => {
    mockGetCurrent.mockResolvedValue(null)
    const { result } = renderHook(() => useBatchRun())
    await act(async () => {
      await result.current.cancel()
    })
    expect(mockCancelBatchRun).toHaveBeenCalled()
  })
})
