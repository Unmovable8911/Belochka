import { useEffect, useMemo, useReducer, useRef, useCallback, type ReactNode } from "react"
import {
  MonitorContext,
  monitorReducer,
  initialMonitorState,
  type MonitorAction,
} from "../hooks/useMonitorState"
import { getReconnectDelay } from "../lib/reconnect"

function buildWsUrl(): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${protocol}//${window.location.host}/api/ws`
}

export function WebSocketProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(monitorReducer, initialMonitorState)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const intentionalCloseRef = useRef(false)
  const reconnectAttemptRef = useRef(0)

  const connect = useCallback(() => {
    const ws = new WebSocket(buildWsUrl())
    wsRef.current = ws
    intentionalCloseRef.current = false

    ws.onopen = () => {
      reconnectAttemptRef.current = 0
      dispatch({ type: "ws_connected", data: true })
    }

    ws.onmessage = (event: MessageEvent) => {
      try {
        const envelope = JSON.parse(event.data as string) as {
          type: string
          data: unknown
        }
        dispatch({ type: envelope.type, data: envelope.data } as MonitorAction)
      } catch {
        // Ignore malformed messages
      }
    }

    ws.onclose = (event: CloseEvent) => {
      dispatch({ type: "ws_connected", data: false })
      wsRef.current = null

      // Reconnect on unexpected close (not code 1000 = normal closure)
      if (event.code !== 1000 && !intentionalCloseRef.current) {
        const delay = getReconnectDelay(reconnectAttemptRef.current)
        reconnectAttemptRef.current += 1
        reconnectTimerRef.current = setTimeout(() => {
          connect()
        }, delay)
      }
    }

    ws.onerror = () => {
      // Error will be followed by onclose, so no action needed here
    }
  }, [])

  useEffect(() => {
    connect()

    return () => {
      intentionalCloseRef.current = true
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [connect])

  // dispatch is stable from useReducer, so this only creates a new object when state changes
  const contextValue = useMemo(() => ({ state, dispatch }), [state])

  return (
    <MonitorContext value={contextValue}>
      {children}
    </MonitorContext>
  )
}
