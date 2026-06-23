import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, cleanup } from "@testing-library/react"
import { WebSocketProvider } from "../components/WebSocketProvider"
import { ConnectionBanner } from "../components/ConnectionBanner"
import { StaleDataOverlay } from "../components/StaleDataOverlay"
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

/** Mimics the App layout with banner + overlay */
function AppLayout() {
  return (
    <WebSocketProvider>
      <ConnectionBanner />
      <StaleDataOverlay>
        <span data-testid="content">Dashboard content</span>
      </StaleDataOverlay>
    </WebSocketProvider>
  )
}

describe("Disconnection UX integration", () => {
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

  it("shows banner and dims content on disconnect, restores both on reconnect", () => {
    render(<AppLayout />)

    // Connect and load data
    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: { "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 42 }, cores: [] } }) },
        },
      })
    })

    // No banner, no dimming when connected
    expect(screen.queryByRole("alert")).toBeNull()
    expect(screen.getByTestId("stale-data-overlay")).not.toHaveClass("opacity-50")

    // Disconnect
    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })

    // Banner visible and content dimmed
    expect(screen.getByRole("alert")).toBeInTheDocument()
    expect(screen.getByRole("alert").textContent).toContain("Connection lost")
    expect(screen.getByTestId("stale-data-overlay")).toHaveClass("opacity-50")
    // Content is still present (retained)
    expect(screen.getByTestId("content").textContent).toBe("Dashboard content")

    // Wait for reconnect attempt (1s backoff)
    act(() => {
      vi.advanceTimersByTime(1000)
    })

    // Successful reconnection
    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    // Banner gone, content restored
    expect(screen.queryByRole("alert")).toBeNull()
    expect(screen.getByTestId("stale-data-overlay")).not.toHaveClass("opacity-50")
  })

  it("retains metric data during disconnection", () => {
    function MetricsDisplay() {
      const { state } = useMonitorState()
      return (
        <div>
          {Object.entries(state.metrics).map(([id, m]: [string, ServerMetrics]) => (
            <span key={id} data-testid={`cpu-${id}`}>
              {m.cpu.aggregate.usagePercent}
            </span>
          ))}
        </div>
      )
    }

    render(
      <WebSocketProvider>
        <MetricsDisplay />
      </WebSocketProvider>
    )

    // Connect and receive metrics
    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: { "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 55 }, cores: [] } }) },
        },
      })
    })

    expect(screen.getByTestId("cpu-srv-1").textContent).toBe("55")

    // Disconnect
    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })

    // Metrics are still available
    expect(screen.getByTestId("cpu-srv-1").textContent).toBe("55")
  })

  it("does not trigger reconnection on intentional close (unmount)", () => {
    const { unmount } = render(<AppLayout />)

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    const countBefore = wsInstances.length
    unmount()

    // Advance time well beyond any backoff delay
    act(() => {
      vi.advanceTimersByTime(60000)
    })

    // No new WebSocket instances created after unmount
    expect(wsInstances.length).toBe(countBefore)
  })

  it("receives fresh snapshot on reconnection that replaces stale state", () => {
    function MetricsDisplay() {
      const { state } = useMonitorState()
      return (
        <div>
          <span data-testid="server-count">{state.servers.length}</span>
          {Object.entries(state.metrics).map(([id, m]: [string, ServerMetrics]) => (
            <span key={id} data-testid={`cpu-${id}`}>
              {m.cpu.aggregate.usagePercent}
            </span>
          ))}
        </div>
      )
    }

    render(
      <WebSocketProvider>
        <MetricsDisplay />
      </WebSocketProvider>
    )

    // Initial snapshot
    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [{ id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" }],
          metrics: { "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 30 }, cores: [] } }) },
        },
      })
    })

    expect(screen.getByTestId("cpu-srv-1").textContent).toBe("30")

    // Disconnect
    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })

    // Reconnect
    act(() => {
      vi.advanceTimersByTime(1000)
    })

    // Server sends fresh snapshot with updated data
    act(() => {
      mockWs.onopen?.(new Event("open"))
      sendMessage(mockWs, {
        type: "snapshot",
        data: {
          servers: [
            { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
            { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "connected" },
          ],
          metrics: {
            "srv-1": makeMetrics({ cpu: { aggregate: { usagePercent: 65 }, cores: [] } }),
            "srv-2": makeMetrics({ cpu: { aggregate: { usagePercent: 20 }, cores: [] } }),
          },
        },
      })
    })

    // State is replaced with fresh snapshot
    expect(screen.getByTestId("server-count").textContent).toBe("2")
    expect(screen.getByTestId("cpu-srv-1").textContent).toBe("65")
    expect(screen.getByTestId("cpu-srv-2").textContent).toBe("20")
  })
})
