import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
  type MonitorAction,
} from "../hooks/useMonitorState"
import type { ServerMetrics } from "../types/server"
import Dashboard from "../pages/Dashboard"
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
      cores: [],
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

describe("Dashboard Server Cards", () => {
  afterEach(() => {
    cleanup()
  })

  // --- Slice 1: Card shows server name, host, status ---

  it("renders a card for each server with name and host", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({ id: "srv-1", name: "Web Server", host: "10.0.0.1" }),
        makeServer({ id: "srv-2", name: "DB Server", host: "10.0.0.2" }),
      ],
    }
    renderDashboard(state)

    expect(screen.getByText("Web Server")).toBeInTheDocument()
    expect(screen.getByText("10.0.0.1")).toBeInTheDocument()
    expect(screen.getByText("DB Server")).toBeInTheDocument()
    expect(screen.getByText("10.0.0.2")).toBeInTheDocument()
  })

  it("shows status indicator for each server", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({ id: "srv-1", name: "Web Server", status: "connected" }),
        makeServer({ id: "srv-2", name: "DB Server", status: "error" }),
      ],
    }
    renderDashboard(state)

    expect(screen.getByText("connected")).toBeInTheDocument()
    expect(screen.getByText("error")).toBeInTheDocument()
  })

  // --- Slice 2: CPU usage bar ---

  it("shows CPU usage with percentage and progress bar", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 72.5 }, cores: [] },
        }),
      },
    }
    renderDashboard(state)

    expect(screen.getByText("CPU")).toBeInTheDocument()
    expect(screen.getByText("72.5%")).toBeInTheDocument()
    // Progress bar should exist with correct aria-label
    const cpuBar = screen.getByRole("progressbar", { name: /cpu/i })
    expect(cpuBar).toBeInTheDocument()
  })

  // --- Slice 3: Memory usage bar ---

  it("shows memory usage with percentage and progress bar", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          memory: {
            total: 16 * 1024 * 1024 * 1024,
            used: 12 * 1024 * 1024 * 1024,
            available: 4 * 1024 * 1024 * 1024,
            swapTotal: 0,
            swapUsed: 0,
          },
        }),
      },
    }
    renderDashboard(state)

    expect(screen.getByText("Memory")).toBeInTheDocument()
    // 12/16 = 75%
    expect(screen.getByText("75.0%")).toBeInTheDocument()
    const memBar = screen.getByRole("progressbar", { name: /memory/i })
    expect(memBar).toBeInTheDocument()
  })

  // --- Slice 4: Disk — highest-usage partition ---

  it("shows the highest-usage disk partition with mount point", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          disk: {
            partitions: [
              {
                filesystem: "/dev/sda1",
                mountPoint: "/",
                total: 100 * 1024 * 1024 * 1024,
                used: 30 * 1024 * 1024 * 1024,
                available: 70 * 1024 * 1024 * 1024,
              },
              {
                filesystem: "/dev/sdb1",
                mountPoint: "/data",
                total: 200 * 1024 * 1024 * 1024,
                used: 180 * 1024 * 1024 * 1024,
                available: 20 * 1024 * 1024 * 1024,
              },
            ],
          },
        }),
      },
    }
    renderDashboard(state)

    // /data has 90% usage (highest), should be displayed
    expect(screen.getByText(/\/data/)).toBeInTheDocument()
    expect(screen.getByText("90.0%")).toBeInTheDocument()
    const diskBar = screen.getByRole("progressbar", { name: /disk/i })
    expect(diskBar).toBeInTheDocument()
  })

  // --- Slice 5: Network — aggregated physical interfaces only ---

  it("shows aggregated network throughput from physical interfaces only", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          network: {
            interfaces: [
              { name: "eth0", rxBytesPerSec: 1000000, txBytesPerSec: 500000 },
              { name: "eth1", rxBytesPerSec: 2000000, txBytesPerSec: 1000000 },
              // Virtual interfaces — should be excluded
              { name: "lo", rxBytesPerSec: 999999, txBytesPerSec: 999999 },
              { name: "docker0", rxBytesPerSec: 888888, txBytesPerSec: 888888 },
              { name: "veth1234", rxBytesPerSec: 777777, txBytesPerSec: 777777 },
              { name: "br-abcdef", rxBytesPerSec: 666666, txBytesPerSec: 666666 },
              { name: "virbr0", rxBytesPerSec: 555555, txBytesPerSec: 555555 },
            ],
          },
        }),
      },
    }
    renderDashboard(state)

    // Aggregated physical: RX = 1000000 + 2000000 = 3000000 = 3.0 MB/s
    // TX = 500000 + 1000000 = 1500000 = 1.5 MB/s
    expect(screen.getByText("Network")).toBeInTheDocument()
    expect(screen.getByText(/3\.0 MB\/s/)).toBeInTheDocument()
    expect(screen.getByText(/1\.5 MB\/s/)).toBeInTheDocument()
  })

  // --- Slice 6: Cards are clickable, linking to /server/:id ---

  it("cards are clickable links to /server/:id", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "Web Server" })],
    }
    renderDashboard(state)

    const link = screen.getByRole("link", { name: /web server/i })
    expect(link).toHaveAttribute("href", "/server/srv-1")
  })

  // --- Slice 7: Color coding via thresholds ---

  it("applies green color to low-usage metrics", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 25 }, cores: [] },
        }),
      },
    }
    renderDashboard(state)

    const cpuBar = screen.getByRole("progressbar", { name: /cpu/i })
    expect(cpuBar.dataset.color).toBe("green")
  })

  it("applies yellow color to medium-usage metrics", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 65 }, cores: [] },
        }),
      },
    }
    renderDashboard(state)

    const cpuBar = screen.getByRole("progressbar", { name: /cpu/i })
    expect(cpuBar.dataset.color).toBe("yellow")
  })

  it("applies red color to high-usage metrics", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          cpu: { aggregate: { usagePercent: 90 }, cores: [] },
        }),
      },
    }
    renderDashboard(state)

    const cpuBar = screen.getByRole("progressbar", { name: /cpu/i })
    expect(cpuBar.dataset.color).toBe("red")
  })

  // --- Slice 8: Graceful handling when no metrics available ---

  it("renders card without metrics when server has no metrics yet", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1", name: "New Server" })],
      // no metrics entry for srv-1
    }
    renderDashboard(state)

    expect(screen.getByText("New Server")).toBeInTheDocument()
    // Should not crash — no progress bars rendered
    expect(screen.queryByRole("progressbar")).toBeNull()
  })

  // --- Slice 9: Grid layout (responsive) ---

  it("renders servers in a grid layout", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [
        makeServer({ id: "srv-1", name: "Server A" }),
        makeServer({ id: "srv-2", name: "Server B" }),
      ],
    }
    const { container } = renderDashboard(state)

    // Look for a grid container
    const grid = container.querySelector("[data-testid='server-grid']")
    expect(grid).toBeInTheDocument()
  })
})
