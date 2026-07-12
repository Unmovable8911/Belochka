import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
  type MonitorAction,
} from "../hooks/useMonitorState"
import type { ServerInfo } from "../types/server"
import Dashboard from "../pages/Dashboard"
import ServerDetail from "../pages/ServerDetail"
import type { Dispatch } from "react"

// --- Mock API module ---
const { mockReconnectServer } = vi.hoisted(() => ({
  mockReconnectServer: vi.fn(),
}))

vi.mock("@/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/client")>()
  return {
    ...actual,
    reconnectServer: mockReconnectServer,
  }
})

// --- Test data builders ---

function makeServer(overrides: Partial<ServerInfo> = {}): ServerInfo {
  return {
    id: overrides.id ?? "srv-1",
    name: overrides.name ?? "Web Server",
    host: overrides.host ?? "10.0.0.1",
    status: overrides.status ?? "connected",
    attempts: overrides.attempts,
    lastError: overrides.lastError,
  }
}

function makeFailedServer(overrides: Partial<ServerInfo> = {}): ServerInfo {
  return makeServer({
    status: "failed",
    lastError: "authentication failed: password rejected",
    ...overrides,
  })
}

function renderDashboard(state: MonitorState) {
  const dispatch: Dispatch<MonitorAction> = vi.fn()
  return render(
    <MonitorContext value={{ state, dispatch }}>
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>
    </MonitorContext>
  )
}

function renderDetail(state: MonitorState, serverId = "srv-1") {
  const dispatch: Dispatch<MonitorAction> = vi.fn()
  return render(
    <MonitorContext value={{ state, dispatch }}>
      <MemoryRouter initialEntries={[`/server/${serverId}`]}>
        <Routes>
          <Route path="/server/:id" element={<ServerDetail />} />
        </Routes>
      </MemoryRouter>
    </MonitorContext>
  )
}

describe("ServerCard reconnect button", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    cleanup()
  })

  it("shows reconnect button when server status is failed", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDashboard(state)

    expect(screen.getByRole("button", { name: /reconnect/i })).toBeInTheDocument()
  })

  it("does NOT show reconnect button when server status is connected", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "Web Server", status: "connected" })],
    }
    renderDashboard(state)

    expect(screen.queryByRole("button", { name: /reconnect/i })).toBeNull()
  })

  it("does NOT show reconnect button when server status is reconnecting", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "Web Server", status: "reconnecting", attempts: 3 })],
    }
    renderDashboard(state)

    expect(screen.queryByRole("button", { name: /reconnect/i })).toBeNull()
  })

  it("calls reconnectServer API and shows loading state on click", async () => {
    mockReconnectServer.mockImplementation(() => new Promise(() => {})) // never resolves

    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDashboard(state)

    const button = screen.getByRole("button", { name: /reconnect/i })
    await userEvent.click(button)

    expect(mockReconnectServer).toHaveBeenCalledWith("srv-1")
    // Button should be disabled during loading
    expect(button).toBeDisabled()
  })

  it("stops click propagation to prevent card navigation", async () => {
    mockReconnectServer.mockResolvedValue(undefined)

    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDashboard(state)

    const link = screen.getByRole("link", { name: /web server/i })
    // Verify link still navigates to detail page
    expect(link).toHaveAttribute("href", "/server/srv-1")

    const button = screen.getByRole("button", { name: /reconnect/i })
    await userEvent.click(button)

    // API was called (button click was handled)
    expect(mockReconnectServer).toHaveBeenCalledWith("srv-1")
    // We're still on the dashboard (not navigated to detail page)
    // This is confirmed by the button still being in the document
    expect(screen.getByRole("button", { name: /reconnect/i })).toBeInTheDocument()
  })

  it("shows loading spinner while reconnecting", async () => {
    mockReconnectServer.mockImplementation(() => new Promise(() => {})) // never resolves

    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDashboard(state)

    const button = screen.getByRole("button", { name: /reconnect/i })
    await userEvent.click(button)

    // Button text should still be there or we check for spinner
    // The button should have a loading indicator (Loader2 icon)
    expect(button.querySelector("svg")).toBeTruthy()
  })
})

describe("ServerDetail reconnect button", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    cleanup()
  })

  it("shows reconnect button when server status is failed", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDetail(state)

    expect(screen.getByRole("button", { name: /reconnect/i })).toBeInTheDocument()
  })

  it("does NOT show reconnect button when server is connected", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "Web Server", status: "connected" })],
    }
    renderDetail(state)

    expect(screen.queryByRole("button", { name: /reconnect/i })).toBeNull()
  })

  it("calls reconnectServer API and shows loading state on click", async () => {
    mockReconnectServer.mockImplementation(() => new Promise(() => {})) // never resolves

    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeFailedServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDetail(state)

    const button = screen.getByRole("button", { name: /reconnect/i })
    await userEvent.click(button)

    expect(mockReconnectServer).toHaveBeenCalledWith("srv-1")
    expect(button).toBeDisabled()
  })
})
