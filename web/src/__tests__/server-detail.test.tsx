import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
  type MonitorAction,
  type ServerMetrics,
} from "../hooks/useMonitorState"
import ServerDetail from "../pages/ServerDetail"
import type { Dispatch } from "react"

// --- Test data builders ---

function makeServer(overrides: Partial<{ id: string; name: string; host: string; status: string }> = {}) {
  return {
    id: overrides.id ?? "srv-1",
    name: overrides.name ?? "Web Server",
    host: overrides.host ?? "10.0.0.1",
    status: overrides.status ?? "connected",
  }
}

function makeMetrics(overrides: Partial<ServerMetrics> = {}): ServerMetrics {
  return {
    cpu: overrides.cpu ?? {
      aggregate: { usagePercent: 45.2 },
      cores: [
        { name: "cpu0", usagePercent: 50.0 },
        { name: "cpu1", usagePercent: 40.0 },
      ],
    },
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
      kernel: "5.15.0-generic",
      uptimeSec: 90061,
      osName: "Ubuntu 22.04",
      coreCount: 4,
    },
  }
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

describe("ServerDetail", () => {
  afterEach(() => {
    cleanup()
  })

  it("displays hostname, kernel, uptime, OS, and core count", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "Web Server" })],
      metrics: { "srv-1": makeMetrics() },
    }
    renderDetail(state)

    expect(screen.getByText("web-01")).toBeInTheDocument()
    expect(screen.getByText("5.15.0-generic")).toBeInTheDocument()
    expect(screen.getByText("1d 1h 1m")).toBeInTheDocument()
    expect(screen.getByText("Ubuntu 22.04")).toBeInTheDocument()
    expect(screen.getByText("4 cores")).toBeInTheDocument()
  })

  it("shows CPU ring gauge with overall percentage and conic-gradient", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: {
            aggregate: { usagePercent: 72.5 },
            cores: [],
          },
        }),
      },
    }
    renderDetail(state)

    const ring = screen.getByTestId("cpu-ring-gauge")
    expect(ring).toBeInTheDocument()
    // conic-gradient should use the percentage
    expect(ring.style.background).toContain("conic-gradient")
    expect(ring.style.background).toContain("72.5%")
    // The label should show the percentage
    expect(screen.getByText("72.5%")).toBeInTheDocument()
  })

  it("applies correct color to ring gauge based on threshold", () => {
    // Green (< 60%)
    const greenState: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 25.0 }, cores: [] },
        }),
      },
    }
    const { unmount: unmount1 } = renderDetail(greenState)
    const greenRing = screen.getByTestId("cpu-ring-gauge")
    expect(greenRing.dataset.color).toBe("green")
    unmount1()

    // Yellow (60-80%)
    const yellowState: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 65.0 }, cores: [] },
        }),
      },
    }
    const { unmount: unmount2 } = renderDetail(yellowState)
    const yellowRing = screen.getByTestId("cpu-ring-gauge")
    expect(yellowRing.dataset.color).toBe("yellow")
    unmount2()

    // Red (>= 80%)
    const redState: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 90.0 }, cores: [] },
        }),
      },
    }
    renderDetail(redState)
    const redRing = screen.getByTestId("cpu-ring-gauge")
    expect(redRing.dataset.color).toBe("red")
  })

  it("renders per-core progress bars with labels and percentages", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: {
            aggregate: { usagePercent: 45.0 },
            cores: [
              { name: "cpu0", usagePercent: 30.0 },
              { name: "cpu1", usagePercent: 70.0 },
              { name: "cpu2", usagePercent: 85.0 },
              { name: "cpu3", usagePercent: 10.0 },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    // Each core should have a labeled progress bar
    expect(screen.getByText("Core 0")).toBeInTheDocument()
    expect(screen.getByText("Core 1")).toBeInTheDocument()
    expect(screen.getByText("Core 2")).toBeInTheDocument()
    expect(screen.getByText("Core 3")).toBeInTheDocument()

    // Percentage values displayed
    expect(screen.getByText("30.0%")).toBeInTheDocument()
    expect(screen.getByText("70.0%")).toBeInTheDocument()
    expect(screen.getByText("85.0%")).toBeInTheDocument()
    expect(screen.getByText("10.0%")).toBeInTheDocument()

    // Progress bars with correct aria-labels
    const bars = screen.getAllByRole("progressbar")
    expect(bars).toHaveLength(4)

    // Check color coding on core bars
    const core0Bar = screen.getByRole("progressbar", { name: /core 0/i })
    expect(core0Bar.dataset.color).toBe("green") // 30%

    const core1Bar = screen.getByRole("progressbar", { name: /core 1/i })
    expect(core1Bar.dataset.color).toBe("yellow") // 70%

    const core2Bar = screen.getByRole("progressbar", { name: /core 2/i })
    expect(core2Bar.dataset.color).toBe("red") // 85%

    const core3Bar = screen.getByRole("progressbar", { name: /core 3/i })
    expect(core3Bar.dataset.color).toBe("green") // 10%
  })

  it("provides a back link to the dashboard", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: { "srv-1": makeMetrics() },
    }
    renderDetail(state)

    const backLink = screen.getByRole("link", { name: /back to dashboard/i })
    expect(backLink).toBeInTheDocument()
    expect(backLink).toHaveAttribute("href", "/")
  })

  it("shows loading state when server exists but has no metrics", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "New Server" })],
      // no metrics
    }
    renderDetail(state)

    // Server name should show
    expect(screen.getByText("New Server")).toBeInTheDocument()
    // No ring gauge or progress bars
    expect(screen.queryByTestId("cpu-ring-gauge")).toBeNull()
    expect(screen.queryByTestId("system-info-bar")).toBeNull()
    expect(screen.queryByRole("progressbar")).toBeNull()
  })

  it("shows not-found state when server ID does not exist", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
    }
    renderDetail(state, "nonexistent-id")

    expect(screen.getByText("Server not found")).toBeInTheDocument()
    // Still has back link
    const backLink = screen.getByRole("link", { name: /back to dashboard/i })
    expect(backLink).toHaveAttribute("href", "/")
  })
})
