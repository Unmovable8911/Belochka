import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  monitorReducer,
  type MonitorState,
  type MonitorAction,
  type ServerMetrics,
  type ServerInfo,
} from "../hooks/useMonitorState"
import Dashboard from "../pages/Dashboard"
import type { Dispatch } from "react"

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

function makeMetrics(overrides: Partial<ServerMetrics> = {}): ServerMetrics {
  return {
    cpu: overrides.cpu ?? { aggregate: { usagePercent: 45.2 }, cores: [] },
    memory: overrides.memory ?? {
      total: 8 * 1024 * 1024 * 1024,
      used: 4 * 1024 * 1024 * 1024,
      available: 4 * 1024 * 1024 * 1024,
      swapTotal: 0,
      swapUsed: 0,
    },
    disk: overrides.disk ?? {
      partitions: [
        {
          filesystem: "/dev/sda1",
          mountPoint: "/",
          total: 100 * 1024 * 1024 * 1024,
          used: 60 * 1024 * 1024 * 1024,
          available: 40 * 1024 * 1024 * 1024,
        },
      ],
    },
    network: overrides.network ?? {
      interfaces: [
        { name: "eth0", rxBytesPerSec: 1500000, txBytesPerSec: 500000 },
      ],
    },
    process: overrides.process ?? { processes: [] },
    system: overrides.system ?? {
      hostname: "web-01",
      kernel: "5.15.0",
      uptimeSec: 86400,
      osName: "Ubuntu 22.04",
      coreCount: 4,
    },
  }
}

function renderDashboard(state: MonitorState = initialMonitorState) {
  const dispatch: Dispatch<MonitorAction> = vi.fn()
  return render(
    <MonitorContext value={{ state, dispatch }}>
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>
    </MonitorContext>
  )
}

describe("Dashboard Disconnected State Display", () => {
  afterEach(() => {
    cleanup()
  })

  // --- Slice 1: Reducer stores attempts + lastError from status action ---

  it("stores attempts and lastError from status action on ServerInfo", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [
        { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected" },
      ],
    }

    const action: MonitorAction = {
      type: "status",
      data: {
        serverId: "srv-1",
        status: "reconnecting",
        attempts: 3,
        lastError: "connection refused",
      },
    }

    const state = monitorReducer(stateWithServers, action)

    expect(state.servers[0].status).toBe("reconnecting")
    expect(state.servers[0].attempts).toBe(3)
    expect(state.servers[0].lastError).toBe("connection refused")
  })

  // --- Slice 2: Reconnecting card shows attempt count ---

  it("shows reconnecting status with attempt count instead of metrics", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({ id: "srv-1", name: "Web Server", status: "reconnecting", attempts: 3 }),
      ],
      metrics: {
        "srv-1": makeMetrics(),
      },
    }
    renderDashboard(state)

    // Should show reconnecting message with attempt count
    expect(screen.getByText(/Reconnecting \(3\/∞\)/)).toBeInTheDocument()
    // Should NOT show metric bars
    expect(screen.queryByRole("progressbar")).toBeNull()
  })

  // --- Slice 3: Auth failure shows clear message ---

  it("shows auth failure message for failed status with auth error", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({
          id: "srv-1",
          name: "Web Server",
          status: "failed",
          lastError: "authentication failed: password rejected",
        }),
      ],
    }
    renderDashboard(state)

    expect(screen.getByText(/Auth failed/)).toBeInTheDocument()
    expect(screen.getByText(/check configuration/i)).toBeInTheDocument()
  })

  // --- Slice 4: Host key mismatch shows specific error ---

  it("shows host key mismatch message", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({
          id: "srv-1",
          name: "Web Server",
          status: "failed",
          lastError: "host key mismatch: expected SHA256:abc, got SHA256:xyz",
        }),
      ],
    }
    renderDashboard(state)

    expect(screen.getByText(/Host key mismatch/)).toBeInTheDocument()
    expect(screen.getByText(/check configuration/i)).toBeInTheDocument()
  })

  // --- Slice 5: Connecting state shows loading indicator ---

  it("shows loading indicator for connecting state", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({
          id: "srv-1",
          name: "Web Server",
          status: "reconnecting",
          attempts: 0,
        }),
      ],
    }
    renderDashboard(state)

    expect(screen.getByText(/Connecting/)).toBeInTheDocument()
  })

  // --- Slice 6: Disconnected card remains clickable ---

  it("disconnected card remains clickable and links to detail page", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({
          id: "srv-1",
          name: "Web Server",
          status: "reconnecting",
          attempts: 5,
        }),
      ],
    }
    renderDashboard(state)

    const link = screen.getByRole("link", { name: /web server/i })
    expect(link).toHaveAttribute("href", "/server/srv-1")
  })

  // --- Slice 7: Disconnected card retains same size as connected cards ---

  it("disconnected card has the same structural elements as connected card (grid stability)", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({
          id: "srv-1",
          name: "Connected Server",
          status: "connected",
        }),
        makeServer({
          id: "srv-2",
          name: "Disconnected Server",
          status: "reconnecting",
          attempts: 2,
        }),
      ],
      metrics: {
        "srv-1": makeMetrics(),
      },
    }
    renderDashboard(state)

    // Both cards are rendered in the grid
    expect(screen.getByText("Connected Server")).toBeInTheDocument()
    expect(screen.getByText("Disconnected Server")).toBeInTheDocument()

    // Both cards have a CardContent area (disconnected card has min-height for stability)
    const grid = screen.getByTestId("server-grid")
    const cards = grid.querySelectorAll("[data-testid='server-card']")
    expect(cards).toHaveLength(2)
  })

  // --- Slice 8: Connected card with metrics still works normally ---

  it("connected card still shows metrics normally", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({ id: "srv-1", name: "Web Server", status: "connected" }),
      ],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 45.2 }, cores: [] },
        }),
      },
    }
    renderDashboard(state)

    expect(screen.getByText("CPU")).toBeInTheDocument()
    expect(screen.getByText("45.2%")).toBeInTheDocument()
    expect(screen.getByRole("progressbar", { name: /cpu/i })).toBeInTheDocument()
  })
})
