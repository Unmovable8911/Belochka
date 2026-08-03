import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
} from "../hooks/useMonitorState"
import type { Dispatch } from "react"
import type { Process } from "@/types/server"

// Mock the API client
vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>()
  return {
    ...actual,
    getProcesses: vi.fn(),
    killProcess: vi.fn(),
  }
})

import { getProcesses } from "../api/client"
import ServerDetail from "../pages/ServerDetail"

function makeServer(overrides: Record<string, unknown> = {}) {
  return {
    id: "srv-1",
    name: "Test Server",
    host: "10.0.0.1",
    port: 22,
    auth_type: "password",
    username: "root",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    status: "connected",
    ...overrides,
  }
}

function makeProcess(overrides: Partial<Process> = {}): Process {
  return {
    pid: overrides.pid ?? 1,
    ppid: overrides.ppid ?? 0,
    user: overrides.user ?? "root",
    rss: overrides.rss ?? 10000,
    cpuPct: overrides.cpuPct ?? 0.5,
    memPct: overrides.memPct ?? 1.0,
    etime: overrides.etime ?? "01:00:00",
    command: overrides.command ?? "systemd",
    command_name: overrides.command_name ?? "systemd",
    protected: overrides.protected ?? false,
  }
}

function renderDetail(state?: Partial<MonitorState>) {
  const s: MonitorState = {
    ...initialMonitorState,
    servers: [makeServer()],
    ...state,
  }
  const noopDispatch: Dispatch<unknown> = () => {}
  return render(
    <MonitorContext.Provider value={{ state: s, dispatch: noopDispatch }}>
      <MemoryRouter initialEntries={["/server/srv-1"]}>
        <Routes>
          <Route path="/server/:id" element={<ServerDetail />} />
        </Routes>
      </MemoryRouter>
    </MonitorContext.Provider>,
  )
}

describe("ServerDetail — Processes tab", () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it("renders the Processes tab button", () => {
    renderDetail()
    expect(screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })).toBeInTheDocument()
  })

  it("shows loading state when tab is selected", async () => {
    vi.mocked(getProcesses).mockReturnValue(new Promise(() => {})) // never resolves
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    expect(screen.getByTestId("processes-loading")).toBeInTheDocument()
  })

  it("shows error state when fetch fails", async () => {
    vi.mocked(getProcesses).mockRejectedValue(new Error("SSH connection failed"))
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("processes-error")).toBeInTheDocument()
    })
  })

  it("shows empty state when process list is empty", async () => {
    vi.mocked(getProcesses).mockResolvedValue([])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("processes-empty")).toBeInTheDocument()
    })
  })

  it("renders process rows when data is loaded", async () => {
    vi.mocked(getProcesses).mockResolvedValue([
      makeProcess({ pid: 1, ppid: 0, command: "systemd" }),
      makeProcess({ pid: 100, ppid: 0, command: "sshd" }),
      makeProcess({ pid: 200, ppid: 0, user: "www-data", command: "nginx" }),
    ])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("process-table")).toBeInTheDocument()
    })

    expect(screen.getByTestId("process-row-1")).toBeInTheDocument()
    expect(screen.getByTestId("process-row-100")).toBeInTheDocument()
    expect(screen.getByTestId("process-row-200")).toBeInTheDocument()
  })

  it("shows process count badge", async () => {
    vi.mocked(getProcesses).mockResolvedValue([
      makeProcess({ pid: 1 }),
      makeProcess({ pid: 2 }),
    ])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      const badge = screen.getByTestId("process-count")
      expect(badge).toBeInTheDocument()
    })
  })

  it("filters processes by search keyword", async () => {
    vi.mocked(getProcesses).mockResolvedValue([
      makeProcess({ pid: 1, command: "systemd" }),
      makeProcess({ pid: 2, command: "nginx master" }),
    ])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("process-table")).toBeInTheDocument()
    })

    const searchInput = screen.getByTestId("process-search")
    await userEvent.type(searchInput, "nginx")

    await waitFor(() => {
      expect(screen.queryByTestId("process-row-1")).not.toBeInTheDocument()
      expect(screen.getByTestId("process-row-2")).toBeInTheDocument()
    })
  })

  it("kill button opens confirmation dialog", async () => {
    vi.mocked(getProcesses).mockResolvedValue([
      makeProcess({ pid: 1, command: "myapp" }),
    ])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("process-table")).toBeInTheDocument()
    })

    const killBtn = screen.getByTestId("kill-btn-1")
    await userEvent.click(killBtn)

    // Dialog should appear
    const dialog = screen.getByRole("dialog")
    expect(within(dialog).getByText(/myapp/)).toBeInTheDocument()
    expect(screen.getByTestId("kill-confirm-btn")).toBeInTheDocument()
  })

  it("kill button is disabled for protected processes", async () => {
    vi.mocked(getProcesses).mockResolvedValue([
      makeProcess({ pid: 1, command: "/usr/sbin/sshd", command_name: "sshd", protected: true }),
    ])
    renderDetail()

    const tab = screen.getByRole("tab", { name: /processes|进程|процессы|processus/i })
    await userEvent.click(tab)

    await waitFor(() => {
      expect(screen.getByTestId("process-table")).toBeInTheDocument()
    })

    const killBtn = screen.getByTestId("kill-btn-1")
    expect(killBtn).toBeDisabled()
  })

})
