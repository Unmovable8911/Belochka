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
  data: { serverId: string; name?: string; host?: string; group_id?: string }
}

export interface AddServerAction {
  type: "add_server"
  data: { id: string; name: string; host: string; group_id?: string }
}

export type MonitorAction =
  | SnapshotAction
  | WsConnectedAction
  | RemoveServerAction
  | UpdateServerAction
  | AddServerAction

// --- Reducer ---

export function monitorReducer(state: MonitorState, action: MonitorAction): MonitorState {
  switch (action.type) {
    case "snapshot":
      return {
        ...state,
        servers: action.data.servers,
        metrics: action.data.metrics,
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
            ? {
                ...s,
                ...(action.data.name !== undefined ? { name: action.data.name } : {}),
                ...(action.data.host !== undefined ? { host: action.data.host } : {}),
                ...("group_id" in action.data ? { group_id: action.data.group_id } : {}),
              }
            : s
        ),
      }
    }

    case "add_server": {
      // Optimistic: add the server before the next WebSocket snapshot arrives.
      // If the server is already in the list (race with WS), skip the duplicate.
      if (state.servers.some((s) => s.id === action.data.id)) {
        return state
      }
      return {
        ...state,
        servers: [
          ...state.servers,
          {
            id: action.data.id,
            name: action.data.name,
            host: action.data.host,
            status: "connecting",
            group_id: action.data.group_id,
          },
        ],
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
