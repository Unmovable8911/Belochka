import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, cleanup } from "@testing-library/react"
import { WebSocketProvider } from "../components/WebSocketProvider"
import { useMonitorState } from "../hooks/useMonitorState"
import type { ServerMetrics } from "../hooks/useMonitorState"

// --- Mock WebSocket ---

interface MockWebSocket {
  url: string
  onopen: ((ev: Event) => void) | null
  onclose: ((ev: CloseEvent) => void) | null
  onmessage: ((ev: MessageEvent) => void) | null
  onerror: ((ev: Event) => void) | null
  close: ReturnType<typeof vi.fn>
  readyState: number
}

let mockWs: MockWebSocket
let wsInstances: MockWebSocket[]

function createMockWebSocketClass() {
  wsInstances = []
  return class MockWebSocketImpl {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSING = 2
    static readonly CLOSED = 3

    url: string
    onopen: ((ev: Event) => void) | null = null
    onclose: ((ev: CloseEvent) => void) | null = null
    onmessage: ((ev: MessageEvent) => void) | null = null
    onerror: ((ev: Event) => void) | null = null
    close = vi.fn()
    readyState = 0

    constructor(url: string) {
      this.url = url
      mockWs = this as unknown as MockWebSocket
      wsInstances.push(this as unknown as MockWebSocket)
    }
  }
}

function makeMetrics(overrides: Partial<ServerMetrics> = {}): ServerMetrics {
  return {
    cpu: { aggregate: { usagePercent: 0 }, cores: [] },
    memory: { total: 0, used: 0, available: 0, swapTotal: 0, swapUsed: 0 },
    disk: { partitions: [] },
    network: { interfaces: [] },
    process: { processes: [] },
    system: { hostname: "", kernel: "", uptimeSec: 0, osName: "", coreCount: 0 },
    ...overrides,
  }
}

function sendMessage(ws: MockWebSocket, data: unknown) {
  ws.onmessage?.({
    data: JSON.stringify(data),
  } as MessageEvent)
}

// Consumer component that displays state
function StateDisplay() {
  const { state } = useMonitorState()
  return (
    <div>
      <span data-testid="ws-connected">{String(state.wsConnected)}</span>
      <span data-testid="server-count">{state.servers.length}</span>
      {state.servers.map((s) => (
        <span key={s.id} data-testid={`server-${s.id}-status`}>
          {s.status}
        </span>
      ))}
      {Object.entries(state.metrics).map(([id, m]) => (
        <span key={id} data-testid={`metrics-${id}-cpu`}>
          {m.cpu.aggregate.usagePercent}
        </span>
      ))}
    </div>
  )
}

describe("WebSocketProvider", () => {
  let OriginalWebSocket: typeof WebSocket

  beforeEach(() => {
    OriginalWebSocket = globalThis.WebSocket
    vi.useFakeTimers()
    globalThis.WebSocket = createMockWebSocketClass() as unknown as typeof WebSocket
  })

  afterEach(() => {
    cleanup()
    globalThis.WebSocket = OriginalWebSocket
    vi.useRealTimers()
  })

  it("connects to /api/ws on mount and exposes state via useMonitorState", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    expect(mockWs.url).toContain("/api/ws")
    expect(screen.getByTestId("server-count").textContent).toBe("0")
    expect(screen.getByTestId("ws-connected").textContent).toBe("false")
  })

  it("sets wsConnected to true when WebSocket opens", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    expect(screen.getByTestId("ws-connected").textContent).toBe("true")
  })

  it("dispatches snapshot messages to initialize full state", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [
            { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
          ],
          metrics: {
            "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 42.5 }, cores: [] } }),
          },
        },
      })
    })

    expect(screen.getByTestId("server-count").textContent).toBe("1")
    expect(screen.getByTestId("metrics-srv-1-cpu").textContent).toBe("42.5")
  })

  it("dispatches metrics messages to update server metrics", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    // First send snapshot to establish state
    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: {
            "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 10 }, cores: [] } }),
          },
        },
      })
    })

    // Then send metrics update
    act(() => {
      sendMessage(mockWs, {
        type: "metrics",
        data: {
          serverId: "srv-1",
          metrics: makeMetrics({ cpu: { aggregate: { usagePercent: 88.3 }, cores: [] } }),
        },
      })
    })

    expect(screen.getByTestId("metrics-srv-1-cpu").textContent).toBe("88.3")
  })

  it("dispatches status messages to update server connection state", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: {},
        },
      })
    })

    act(() => {
      sendMessage(mockWs, {
        type: "status",
        data: { serverId: "srv-1", status: "reconnecting" },
      })
    })

    expect(screen.getByTestId("server-srv-1-status").textContent).toBe("reconnecting")
  })

  it("sets wsConnected to false on close and attempts reconnection", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })
    expect(screen.getByTestId("ws-connected").textContent).toBe("true")

    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })
    expect(screen.getByTestId("ws-connected").textContent).toBe("false")

    // Should create a new WebSocket after retry delay
    const countBefore = wsInstances.length
    act(() => {
      vi.advanceTimersByTime(3000)
    })
    expect(wsInstances.length).toBeGreaterThan(countBefore)
  })

  it("does not reconnect on normal close (code 1000)", () => {
    render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
      mockWs.onclose?.({ code: 1000 } as CloseEvent)
    })

    const countAfterClose = wsInstances.length
    act(() => {
      vi.advanceTimersByTime(10000)
    })
    expect(wsInstances.length).toBe(countAfterClose)
  })

  it("closes WebSocket on unmount", () => {
    const { unmount } = render(
      <WebSocketProvider>
        <StateDisplay />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    unmount()
    expect(mockWs.close).toHaveBeenCalled()
  })

  it("does not re-render children when state has not changed", () => {
    let renderCount = 0

    function RenderCounter() {
      renderCount++
      const { state } = useMonitorState()
      return <span data-testid="render-count">{state.servers.length}</span>
    }

    render(
      <WebSocketProvider>
        <RenderCounter />
      </WebSocketProvider>
    )

    const initialCount = renderCount

    // Open the WebSocket - this changes wsConnected, so it will re-render
    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    const afterOpenCount = renderCount
    expect(afterOpenCount).toBeGreaterThan(initialCount)

    // Send a snapshot - this changes state, should re-render
    act(() => {
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: {},
        },
      })
    })

    expect(renderCount).toBeGreaterThan(afterOpenCount)
    expect(screen.getByTestId("render-count").textContent).toBe("1")
  })

  it("throws when useMonitorState is used outside provider", () => {
    // Suppress console.error from React error boundary
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})
    expect(() => render(<StateDisplay />)).toThrow(
      "useMonitorState must be used within a WebSocketProvider"
    )
    spy.mockRestore()
  })
})
