import { useReducer, useEffect, useCallback, useRef } from "react"
import { getProcesses } from "@/api/client"
import type { Process } from "@/types/server"

// --- State ---

export interface ProcessState {
  processes: Process[]
  loading: boolean
  error: string | null
  autoRefresh: boolean
  fetched: boolean
}

export const initialProcessState: ProcessState = {
  processes: [],
  loading: false,
  error: null,
  autoRefresh: true,
  fetched: false,
}

// --- Actions ---

export type ProcessAction =
  | { type: "FETCH_START" }
  | { type: "FETCH_SUCCESS"; data: Process[] }
  | { type: "FETCH_ERROR"; error: string }
  | { type: "SET_AUTO_REFRESH"; enabled: boolean }

// --- Reducer ---

export function processReducer(state: ProcessState, action: ProcessAction): ProcessState {
  switch (action.type) {
    case "FETCH_START":
      return { ...state, loading: true, error: null }
    case "FETCH_SUCCESS":
      return { ...state, loading: false, error: null, processes: action.data, fetched: true }
    case "FETCH_ERROR":
      return { ...state, loading: false, error: action.error }
    case "SET_AUTO_REFRESH":
      return { ...state, autoRefresh: action.enabled }
    default:
      return state
  }
}

// --- Hook ---

export function useProcesses(serverId: string) {
  const [state, dispatch] = useReducer(processReducer, initialProcessState)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchProcesses = useCallback(async () => {
    dispatch({ type: "FETCH_START" })
    try {
      const data = await getProcesses(serverId)
      dispatch({ type: "FETCH_SUCCESS", data })
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Unknown error"
      dispatch({ type: "FETCH_ERROR", error: msg })
    }
  }, [serverId])

  const setAutoRefresh = useCallback((enabled: boolean) => {
    dispatch({ type: "SET_AUTO_REFRESH", enabled })
  }, [])

  // Auto-refresh every 3s when enabled
  useEffect(() => {
    if (state.autoRefresh) {
      intervalRef.current = setInterval(fetchProcesses, 3000)
    }
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [state.autoRefresh, fetchProcesses])

  // Initial fetch
  useEffect(() => {
    fetchProcesses()
  }, [fetchProcesses])

  return {
    state,
    fetchProcesses,
    setAutoRefresh,
  }
}
