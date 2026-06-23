import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, cleanup } from "@testing-library/react"
import { WebSocketProvider } from "../components/WebSocketProvider"
import { StaleDataOverlay } from "../components/StaleDataOverlay"

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

describe("StaleDataOverlay", () => {
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

  it("renders children at full opacity when connected", () => {
    render(
      <WebSocketProvider>
        <StaleDataOverlay>
          <span data-testid="child">Hello</span>
        </StaleDataOverlay>
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    const wrapper = screen.getByTestId("stale-data-overlay")
    expect(wrapper).not.toHaveClass("opacity-50")
    expect(screen.getByTestId("child").textContent).toBe("Hello")
  })

  it("applies reduced opacity when disconnected", () => {
    render(
      <WebSocketProvider>
        <StaleDataOverlay>
          <span data-testid="child">Hello</span>
        </StaleDataOverlay>
      </WebSocketProvider>
    )

    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    act(() => {
      mockWs.onclose?.({ code: 1006 } as CloseEvent)
    })

    const wrapper = screen.getByTestId("stale-data-overlay")
    expect(wrapper).toHaveClass("opacity-50")
  })

  it("restores full opacity on reconnection", () => {
    render(
      <WebSocketProvider>
        <StaleDataOverlay>
          <span data-testid="child">Hello</span>
        </StaleDataOverlay>
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

    const wrapper = screen.getByTestId("stale-data-overlay")
    expect(wrapper).toHaveClass("opacity-50")

    // Reconnect
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    act(() => {
      mockWs.onopen?.(new Event("open"))
    })

    expect(wrapper).not.toHaveClass("opacity-50")
  })
})
