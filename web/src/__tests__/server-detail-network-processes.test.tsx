import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
  type MonitorAction,
} from "../hooks/useMonitorState"
import type { ServerMetrics } from "../types/server"
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
    aggregate: overrides.aggregate ?? { usagePercent: 45.2 },
    cores: [
        { name: "cpu0", usagePercent: 50.0 },
        { name: "cpu1", usagePercent: 40.0 },
      ],
    memory: overrides.memory ?? {
      total: 8 * 1024 * 1024 * 1024,
      used: 4 * 1024 * 1024 * 1024,
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
        },
      ],
    },
    network: overrides.network ?? {
      interfaces: [
        { name: "eth0", rxBytesPerSec: 1500000, txBytesPerSec: 500000 },
      ],
    },
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

  it("renders network chart with interface selector and rate indicators", () => {
    const state: MonitorState = {
      ...initialMonitorState,
      servers: [makeServer({ id: "srv-1" })],
      metrics: {
        "srv-1": makeMetrics({
          network: {
            interfaces: [
              { name: "eth0", rxBytesPerSec: 1500000, txBytesPerSec: 500000 },
              { name: "eth1", rxBytesPerSec: 800000, txBytesPerSec: 200000 },
            ],
          },
        }),
      },
    }
    renderDetail(state)

    const networkSection = screen.getByTestId("network-section")
    expect(networkSection).toBeInTheDocument()

    // The Select trigger (combobox) renders with the selected interface name.
    const selector = screen.getByRole("combobox")
    expect(selector).toBeInTheDocument()
    expect(selector).toHaveTextContent("eth0")

    // Rate indicators: 1500000 B/s = 1.5 MB/s, 500000 = 500.0 KB/s
    // (text may also appear in Y-axis labels, so use getAllByText)
    expect(screen.getAllByText(/1\.5 MB\/s/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/500\.0 KB\/s/).length).toBeGreaterThanOrEqual(1)

    // RX and TX labels are present.
    expect(screen.getByText(/RX/)).toBeInTheDocument()
    expect(screen.getByText(/TX/)).toBeInTheDocument()
  })
})

