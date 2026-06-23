import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
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

describe("ServerDetail — Network section", () => {
  afterEach(() => {
    cleanup()
  })

  it("shows all network interfaces with name, RX, and TX rates", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          network: {
            interfaces: [
              { name: "eth0", rxBytesPerSec: 1500000, txBytesPerSec: 500000 },
              { name: "lo", rxBytesPerSec: 1000, txBytesPerSec: 1000 },
              { name: "docker0", rxBytesPerSec: 0, txBytesPerSec: 0 },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    const networkSection = screen.getByTestId("network-section")
    expect(networkSection).toBeInTheDocument()
    expect(screen.getByText("eth0")).toBeInTheDocument()
    expect(screen.getByText("lo")).toBeInTheDocument()
    expect(screen.getByText("docker0")).toBeInTheDocument()

    // eth0: 1500000 B/s = 1.5 MB/s, 500000 = 500.0 KB/s
    expect(screen.getByText(/1\.5 MB\/s/)).toBeInTheDocument()
    expect(screen.getByText(/500\.0 KB\/s/)).toBeInTheDocument()
  })
})

describe("ServerDetail — Process table", () => {
  afterEach(() => {
    cleanup()
  })

  it("renders process table with PID, User, CPU%, Memory%, Command columns", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          process: {
            processes: [
              { pid: 1234, user: "root", cpuPct: 25.3, memPct: 10.2, command: "nginx" },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    const processSection = screen.getByTestId("process-section")
    expect(processSection).toBeInTheDocument()

    // Column headers exist
    expect(screen.getByRole("columnheader", { name: /pid/i })).toBeInTheDocument()
    expect(screen.getByRole("columnheader", { name: /user/i })).toBeInTheDocument()
    expect(screen.getByRole("columnheader", { name: /cpu/i })).toBeInTheDocument()
    expect(screen.getByRole("columnheader", { name: /memory/i })).toBeInTheDocument()
    expect(screen.getByRole("columnheader", { name: /command/i })).toBeInTheDocument()

    // Process data displayed
    expect(screen.getByText("1234")).toBeInTheDocument()
    expect(screen.getByText("root")).toBeInTheDocument()
    expect(screen.getByText("25.3%")).toBeInTheDocument()
    expect(screen.getByText("10.2%")).toBeInTheDocument()
    expect(screen.getByText("nginx")).toBeInTheDocument()
  })

  it("limits display to 20 processes and sorts by CPU% descending by default", () => {
    // Create 25 processes with varying CPU%
    const processes = Array.from({ length: 25 }, (_, i) => ({
      pid: 1000 + i,
      user: "user",
      cpuPct: i * 2, // 0, 2, 4, ..., 48
      memPct: 1.0,
      command: `proc-${i}`,
    }))

    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          process: { processes },
        }),
      },
    }
    renderDetail(state)

    const rows = screen.getAllByRole("row")
    // 1 header row + 20 data rows = 21 (capped at 20)
    expect(rows).toHaveLength(21)

    // First data row should have highest CPU% from the first 20 (proc-19 = 38%)
    const firstDataRow = rows[1]
    expect(within(firstDataRow).getByText("proc-19")).toBeInTheDocument()
    expect(within(firstDataRow).getByText("38.0%")).toBeInTheDocument()

    // Processes beyond the first 20 should not appear (proc-20..proc-24)
    expect(screen.queryByText("proc-20")).toBeNull()
    expect(screen.queryByText("proc-24")).toBeNull()
  })

  it("toggles sort direction when clicking the active column header", async () => {
    const user = userEvent.setup()
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          process: {
            processes: [
              { pid: 1, user: "root", cpuPct: 80.0, memPct: 10.0, command: "high-cpu" },
              { pid: 2, user: "www", cpuPct: 5.0, memPct: 50.0, command: "low-cpu" },
              { pid: 3, user: "app", cpuPct: 30.0, memPct: 20.0, command: "mid-cpu" },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    // Default: CPU% descending — first row should be "high-cpu"
    let rows = screen.getAllByRole("row")
    expect(within(rows[1]).getByText("high-cpu")).toBeInTheDocument()
    expect(within(rows[3]).getByText("low-cpu")).toBeInTheDocument()

    // Click CPU% header to toggle to ascending
    const cpuHeader = screen.getByRole("columnheader", { name: /cpu/i })
    await user.click(cpuHeader)

    rows = screen.getAllByRole("row")
    // Now ascending: first data row should be "low-cpu"
    expect(within(rows[1]).getByText("low-cpu")).toBeInTheDocument()
    expect(within(rows[3]).getByText("high-cpu")).toBeInTheDocument()
  })

  it("sorts by a different column when clicking its header", async () => {
    const user = userEvent.setup()
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          process: {
            processes: [
              { pid: 1, user: "root", cpuPct: 80.0, memPct: 10.0, command: "nginx" },
              { pid: 2, user: "www", cpuPct: 5.0, memPct: 50.0, command: "apache" },
              { pid: 3, user: "app", cpuPct: 30.0, memPct: 20.0, command: "node" },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    // Click Memory% header — should sort by memory descending
    const memHeader = screen.getByRole("columnheader", { name: /memory/i })
    await user.click(memHeader)

    const rows = screen.getAllByRole("row")
    // Highest memory first (50%)
    expect(within(rows[1]).getByText("apache")).toBeInTheDocument()
    expect(within(rows[1]).getByText("50.0%")).toBeInTheDocument()
  })

  it("shows empty state message when process list is empty", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          process: { processes: [] },
        }),
      },
    }
    renderDetail(state)

    expect(screen.getByText("No process data available.")).toBeInTheDocument()
    // No table should be rendered
    expect(screen.queryByRole("table")).toBeNull()
  })

  it("does not apply color coding to network values", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          network: {
            interfaces: [
              { name: "eth0", rxBytesPerSec: 1500000, txBytesPerSec: 500000 },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    const networkSection = screen.getByTestId("network-section")
    // No data-color attributes should exist in the network section
    const coloredElements = networkSection.querySelectorAll("[data-color]")
    expect(coloredElements).toHaveLength(0)
  })
})
