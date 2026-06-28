import { createContext, useContext, type Dispatch } from "react"
import type { ServerInfo, ServerMetrics } from "@/types/server"

// --- State ---

export interface MonitorState {
  servers: ServerInfo[]
  metrics: Record<string, ServerMetrics>
  wsConnected: boolean
}

export const initialMonitorState: MonitorState = {
  servers: [],
  metrics: {},
  wsConnected: false,
}

// --- Actions ---

export interface SnapshotAction {
  type: "snapshot"
  data: {
    servers: ServerInfo[]
    metrics: Record<string, ServerMetrics>
  }
}

export interface MetricsAction {
  type: "metrics"
  data: {
    serverId: string
    metrics: ServerMetrics
  }
}

export interface StatusAction {
  type: "status"
  data: {
    serverId: string
    status: string
    attempts?: number
    lastError?: string
  }
}

export interface WsConnectedAction {
  type: "ws_connected"
  data: boolean
}

export interface RemoveServerAction {
  type: "remove_server"
  data: { serverId: string }
}

export interface UpdateServerAction {
  type: "update_server"
  data: { serverId: string; name: string; host: string }
}

export type MonitorAction =
  | SnapshotAction
  | MetricsAction
  | StatusAction
  | WsConnectedAction
  | RemoveServerAction
  | UpdateServerAction

// --- Reducer ---

export function monitorReducer(state: MonitorState, action: MonitorAction): MonitorState {
  switch (action.type) {
    case "snapshot":
      return {
        ...state,
        servers: action.data.servers,
        metrics: action.data.metrics,
      }

    case "metrics":
      return {
        ...state,
        metrics: {
          ...state.metrics,
          [action.data.serverId]: action.data.metrics,
        },
      }

    case "status": {
      const updatedServers = state.servers.map((s) =>
        s.id === action.data.serverId
          ? {
              ...s,
              status: action.data.status,
              attempts: action.data.attempts,
              lastError: action.data.lastError,
            }
          : s
      )
      return {
        ...state,
        servers: updatedServers,
      }
    }

    case "ws_connected":
      return {
        ...state,
        wsConnected: action.data,
      }

    case "remove_server": {
      const { [action.data.serverId]: _, ...remainingMetrics } = state.metrics
      return {
        ...state,
        servers: state.servers.filter((s) => s.id !== action.data.serverId),
        metrics: remainingMetrics,
      }
    }

    case "update_server": {
      return {
        ...state,
        servers: state.servers.map((s) =>
          s.id === action.data.serverId
            ? { ...s, name: action.data.name, host: action.data.host }
            : s
        ),
      }
    }

    default:
      return state
  }
}

// --- Context ---

export interface MonitorContextValue {
  state: MonitorState
  dispatch: Dispatch<MonitorAction>
}

export const MonitorContext = createContext<MonitorContextValue | null>(null)

export function useMonitorState(): MonitorContextValue {
  const ctx = useContext(MonitorContext)
  if (!ctx) {
    throw new Error("useMonitorState must be used within a WebSocketProvider")
  }
  return ctx
}
