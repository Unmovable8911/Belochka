import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, cleanup } from "@testing-library/react"
import { WebSocketProvider } from "../components/WebSocketProvider"
import { ConnectionBanner } from "../components/ConnectionBanner"

// --- Mock WebSocket (same pattern as websocket-provider tests) ---

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

function createMockWebSocketClass() {
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
    }
  }
}

function sendMessage(ws: MockWebSocket, data: unknown) {
  ws.onmessage?.({
    data: JSON.stringify(data),
  } as MessageEvent)
}

describe("ConnectionBanner", () => {
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

  it("does not show banner when connected", () => {
    render(
      <WebSocketProvider>
        <ConnectionBanner />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    expect(screen.queryByRole("alert")).toBeNull()
  })

  it("shows banner with reconnecting message when connection is lost", () => {
    render(
      <WebSocketProvider>
        <ConnectionBanner />
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })

    const alert = screen.getByRole("alert")
    expect(alert).toBeInTheDocument()
    expect(alert.textContent).toContain("Connection lost")
    expect(alert.textContent).toContain("reconnecting")
  })

  it("hides banner when connection is restored", () => {
    render(
      <WebSocketProvider>
        <ConnectionBanner />
      </WebSocketProvider>
    )

    // Connect
    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    // Disconnect
    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })
    expect(screen.getByRole("alert")).toBeInTheDocument()

    // Reconnect
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    expect(screen.queryByRole("alert")).toBeNull()
  })
})
