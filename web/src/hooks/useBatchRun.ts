import { useCallback, useEffect, useReducer, useRef } from "react"
import * as api from "@/api/client"
import type { BatchRunInfo, BatchRunSnapshot, BatchServerEvent, RunResultInfo } from "@/types/batch"

// --- State ---

export type BatchMode = "compose" | "results"

export interface BatchRunState {
  mode: BatchMode
  run: BatchRunInfo | null
  results: RunResultInfo[]
  script: string
  selected: string[]
  wsConnected: boolean
  dispatching: boolean
  // dispatchError carries the API error code plus its message; the dialog
  // renders the code's i18n string when one exists.
  dispatchError: { code: string; message: string } | null
}

export const initialBatchRunState: BatchRunState = {
  mode: "compose",
  run: null,
  results: [],
  script: "",
  selected: [],
  wsConnected: false,
  dispatching: false,
  dispatchError: null,
}

// --- Actions ---

export type BatchRunAction =
  | { type: "OPEN"; run: BatchRunInfo | null; results: RunResultInfo[] }
  | { type: "SET_SCRIPT"; script: string }
  | { type: "TOGGLE_SERVER"; serverId: string }
  | { type: "TOGGLE_GROUP"; memberIds: string[] }
  | { type: "TOGGLE_ALL"; serverIds: string[] }
  | { type: "DISPATCH_START" }
  | { type: "DISPATCH_SUCCESS"; run: BatchRunInfo; results: RunResultInfo[] }
  | { type: "DISPATCH_ERROR"; code: string; message: string }
  | { type: "WS_CONNECTED"; connected: boolean }
  | { type: "WS_EVENT"; event: BatchServerEvent }
  | { type: "SYNC"; run: BatchRunInfo | null; results: RunResultInfo[] }
  | { type: "SET_MODE"; mode: BatchMode }

// --- Selection helpers ---

export type SelectionState = "checked" | "indeterminate" | "unchecked"

// selectionState computes the tri-state of a checkbox over memberIds given the
// selected set. An empty group is never selected.
export function selectionState(memberIds: string[], selected: Set<string>): SelectionState {
  if (memberIds.length === 0) return "unchecked"
  let count = 0
  for (const id of memberIds) {
    if (selected.has(id)) count++
  }
  if (count === 0) return "unchecked"
  if (count === memberIds.length) return "checked"
  return "indeterminate"
}

// toggleGroupSelection flips a Group checkbox: checking an unchecked or
// partially-selected Group selects all its members; unchecking a fully-selected
// Group deselects them all.
export function toggleGroupSelection(memberIds: string[], selected: Set<string>): Set<string> {
  const next = new Set(selected)
  if (selectionState(memberIds, selected) === "checked") {
    for (const id of memberIds) next.delete(id)
  } else {
    for (const id of memberIds) next.add(id)
  }
  return next
}

// toggleAllSelection flips the global select-all checkbox over every server.
export function toggleAllSelection(serverIds: string[], selected: Set<string>): Set<string> {
  const next = new Set(selected)
  const allSelected = serverIds.length > 0 && serverIds.every((id) => selected.has(id))
  if (allSelected) {
    for (const id of serverIds) next.delete(id)
  } else {
    for (const id of serverIds) next.add(id)
  }
  return next
}

// --- Reducer ---

export function batchRunReducer(state: BatchRunState, action: BatchRunAction): BatchRunState {
  switch (action.type) {
    case "OPEN": {
      const running = action.run?.status === "running"
      return {
        ...initialBatchRunState,
        // The script is prefilled from the last run so tweaking and re-running
        // a fix is cheap.
        script: action.run?.script ?? "",
        run: action.run,
        results: action.results,
        // A run in progress jumps straight to the live results view.
        mode: running ? "results" : "compose",
      }
    }

    case "SET_SCRIPT":
      return { ...state, script: action.script }

    case "TOGGLE_SERVER": {
      const next = new Set(state.selected)
      if (next.has(action.serverId)) {
        next.delete(action.serverId)
      } else {
        next.add(action.serverId)
      }
      return { ...state, selected: [...next] }
    }

    case "TOGGLE_GROUP": {
      const next = toggleGroupSelection(action.memberIds, new Set(state.selected))
      return { ...state, selected: [...next] }
    }

    case "TOGGLE_ALL": {
      const next = toggleAllSelection(action.serverIds, new Set(state.selected))
      return { ...state, selected: [...next] }
    }

    case "DISPATCH_START":
      return { ...state, dispatching: true, dispatchError: null }

    case "DISPATCH_SUCCESS":
      return {
        ...state,
        mode: "results",
        run: action.run,
        results: action.results,
        dispatching: false,
        dispatchError: null,
        wsConnected: false,
      }

    case "DISPATCH_ERROR":
      return { ...state, dispatching: false, dispatchError: { code: action.code, message: action.message } }

    case "WS_CONNECTED":
      return { ...state, wsConnected: action.connected }

    case "WS_EVENT": {
      const ev = action.event
      if (ev.type === "output") {
        return {
          ...state,
          results: state.results.map((r) =>
            r.server_id === ev.server_id ? { ...r, output: r.output + ev.data } : r
          ),
        }
      }
      return {
        ...state,
        results: state.results.map((r) =>
          r.server_id === ev.server_id
            ? {
                ...r,
                status: ev.status,
                exit_code: ev.exit_code ?? r.exit_code,
                truncated: ev.truncated ?? r.truncated,
                error: ev.error ?? r.error,
                finished_at: ev.finished_at ?? r.finished_at,
              }
            : r
        ),
      }
    }

    case "SYNC":
      return { ...state, run: action.run, results: action.results }

    case "SET_MODE":
      return { ...state, mode: action.mode }

    default:
      return state
  }
}

// --- Hook ---

const WS_RETRY_DELAY = 1000

export function useBatchRun() {
  const [state, dispatch] = useReducer(batchRunReducer, initialBatchRunState)
  const wsRef = useRef<WebSocket | null>(null)
  const runRef = useRef<BatchRunInfo | null>(null)
  const retryTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    runRef.current = state.run
  }, [state.run])

  // resyncRef is assigned by the effect below; connect's onclose closure
  // reads it at fire time, which is always after mount.
  const resyncRef = useRef<(() => Promise<void>) | null>(null)

  const connect = useCallback((run: BatchRunInfo) => {
    if (wsRef.current) return
    const protocol = location.protocol === "https:" ? "wss:" : "ws:"
    const ws = new WebSocket(`${protocol}//${location.host}/api/ws/batch-runs/${run.id}`)
    wsRef.current = ws

    ws.onopen = () => dispatch({ type: "WS_CONNECTED", connected: true })
    ws.onmessage = (e) => {
      try {
        dispatch({ type: "WS_EVENT", event: JSON.parse(e.data) as BatchServerEvent })
      } catch {
        // Ignore malformed frames.
      }
    }
    ws.onclose = () => {
      wsRef.current = null
      dispatch({ type: "WS_CONNECTED", connected: false })
      // A drop mid-run (e.g. after a page refresh) resyncs over REST and
      // reconnects while the run is still running.
      if (runRef.current?.status === "running" && !retryTimer.current) {
        retryTimer.current = setTimeout(() => {
          retryTimer.current = null
          void resyncRef.current?.()
        }, WS_RETRY_DELAY)
      }
    }
  }, [])

  const resync = useCallback(async () => {
    const run = runRef.current
    if (!run || run.status !== "running") return
    try {
      const snapshot = await api.getCurrentBatchRun()
      if (!snapshot) return
      dispatch({ type: "SYNC", run: snapshot.run, results: snapshot.results })
      if (snapshot.run.status === "running") {
        connect(snapshot.run)
      }
    } catch {
      // Transient failure; the retry timer will fire again.
    }
  }, [connect])

  useEffect(() => {
    resyncRef.current = resync
  }, [resync])

  // Open the dialog: fetch the current run, prefill the script, and jump to
  // the live results view when a run is in progress.
  const open = useCallback(async () => {
    let snapshot: BatchRunSnapshot | null = null
    try {
      snapshot = await api.getCurrentBatchRun()
    } catch {
      snapshot = null
    }
    dispatch({ type: "OPEN", run: snapshot?.run ?? null, results: snapshot?.results ?? [] })
  }, [])

  // Connect the WS when the results view shows a running run; close it when
  // the view leaves that state.
  useEffect(() => {
    const run = state.run
    if (state.mode === "results" && run?.status === "running") {
      if (!wsRef.current) connect(run)
    } else if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [state.mode, state.run, connect])

  // Close the socket on unmount (the dialog closes).
  useEffect(() => {
    return () => {
      if (retryTimer.current) clearTimeout(retryTimer.current)
      wsRef.current?.close()
    }
  }, [])

  const setScript = useCallback((script: string) => {
    dispatch({ type: "SET_SCRIPT", script })
  }, [])

  const toggleServer = useCallback((serverId: string) => {
    dispatch({ type: "TOGGLE_SERVER", serverId })
  }, [])

  const toggleGroup = useCallback((memberIds: string[]) => {
    dispatch({ type: "TOGGLE_GROUP", memberIds })
  }, [])

  const toggleAll = useCallback((serverIds: string[]) => {
    dispatch({ type: "TOGGLE_ALL", serverIds })
  }, [])

  const setMode = useCallback((mode: BatchMode) => {
    dispatch({ type: "SET_MODE", mode })
  }, [])

  // runScript dispatches a new Batch Run and switches to the live results
  // view. A conflict (another run in progress) surfaces as dispatchError.
  const runScript = useCallback(async (script: string, serverIds: string[]) => {
    if (serverIds.length === 0 || script.trim() === "") return
    dispatch({ type: "DISPATCH_START" })
    try {
      const snapshot = await api.createBatchRun(script, serverIds)
      dispatch({ type: "DISPATCH_SUCCESS", run: snapshot.run, results: snapshot.results })
    } catch (err) {
      const apiErr = err as { code?: string }
      dispatch({
        type: "DISPATCH_ERROR",
        code: apiErr.code ?? "unknown",
        message: err instanceof Error ? err.message : "Unknown error",
      })
    }
  }, [])

  const cancel = useCallback(async () => {
    try {
      await api.cancelBatchRun()
    } catch {
      // The WS status events carry the cancellation results; a failure to
      // reach the endpoint is surfaced by the run's own statuses.
    }
  }, [])

  // sendInput writes interactive stdin data to a running Server's process.
  const sendInput = useCallback((serverId: string, data: string) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: "input", server_id: serverId, data }))
  }, [])

  return {
    state,
    open,
    setScript,
    toggleServer,
    toggleGroup,
    toggleAll,
    setMode,
    runScript,
    cancel,
    sendInput,
  }
}
