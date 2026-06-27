import { describe, it, expect, vi, afterEach, beforeEach } from "vitest"
import { render, screen, cleanup, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  type MonitorState,
  type MonitorAction,
} from "../hooks/useMonitorState"
import ServerDetail from "../pages/ServerDetail"
import type { Dispatch } from "react"
import type { CronResult } from "../types/server"

// Mock the API module so we can control getCrons/createCron responses
vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>()
  return {
    ...actual,
    getCrons: vi.fn(),
    createCron: vi.fn(),
  }
})

import { getCrons, createCron } from "../api/client"

const mockGetCrons = vi.mocked(getCrons)
const mockCreateCron = vi.mocked(createCron)

function makeServer() {
  return { id: "srv-1", name: "Web Server", host: "10.0.0.1", status: "connected" }
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

const baseState: MonitorState = {
  ...initialMonitorState,
  servers: [makeServer()],
  metrics: {},
}

describe("ServerDetail — tabs", () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it("shows Overview and Cron Jobs tabs", () => {
    mockGetCrons.mockResolvedValue({ entries: [], passthroughs: [] })
    renderDetail(baseState)

    expect(screen.getByRole("tab", { name: /overview/i })).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: /cron jobs/i })).toBeInTheDocument()
  })

  it("Overview tab is selected by default and shows existing metrics content", () => {
    mockGetCrons.mockResolvedValue({ entries: [], passthroughs: [] })
    const state: MonitorState = {
      ...baseState,
      metrics: {
        "srv-1": {
          cpu: { aggregate: { usagePercent: 42 }, cores: [] },
          memory: { total: 8e9, used: 4e9, available: 4e9, swapTotal: 0, swapUsed: 0 },
          disk: { partitions: [] },
          network: { interfaces: [] },
          process: { processes: [] },
          system: { hostname: "web-01", kernel: "5.15", uptimeSec: 3600, osName: "Ubuntu", coreCount: 2 },
        },
      },
    }
    renderDetail(state)

    // metrics grid is in overview tab (visible by default)
    expect(screen.getByTestId("metrics-grid")).toBeInTheDocument()
    // Cron Jobs tab panel should not be visible
    expect(screen.queryByTestId("cron-jobs-tab")).toBeNull()
  })

  it("clicking Cron Jobs tab shows cron table area", async () => {
    const user = userEvent.setup()
    const cronResult: CronResult = {
      entries: [
        { minute: "0", hour: "*", dayOfMonth: "*", month: "*", dayOfWeek: "*", command: "/usr/bin/hourly.sh", enabled: true, raw: "0 * * * * /usr/bin/hourly.sh" },
      ],
      passthroughs: [],
    }
    mockGetCrons.mockResolvedValue(cronResult)
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    await waitFor(() => {
      expect(screen.getByTestId("cron-jobs-tab")).toBeInTheDocument()
    })
  })

  it("Cron Jobs tab renders entry with schedule and command columns", async () => {
    const user = userEvent.setup()
    const cronResult: CronResult = {
      entries: [
        { minute: "0", hour: "2", dayOfMonth: "*", month: "*", dayOfWeek: "0", command: "/usr/bin/weekly.sh", enabled: true, raw: "0 2 * * 0 /usr/bin/weekly.sh" },
      ],
      passthroughs: [],
    }
    mockGetCrons.mockResolvedValue(cronResult)
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    await waitFor(() => {
      expect(screen.getByText("/usr/bin/weekly.sh")).toBeInTheDocument()
    })

    // Schedule composed from fields
    expect(screen.getByText("0 2 * * 0")).toBeInTheDocument()
  })

  it("shows disabled entry with visual indicator", async () => {
    const user = userEvent.setup()
    const cronResult: CronResult = {
      entries: [
        { minute: "*/5", hour: "*", dayOfMonth: "*", month: "*", dayOfWeek: "*", command: "/usr/bin/check.sh", enabled: false, raw: "#[disabled] */5 * * * * /usr/bin/check.sh" },
      ],
      passthroughs: [],
    }
    mockGetCrons.mockResolvedValue(cronResult)
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    await waitFor(() => {
      expect(screen.getByText("/usr/bin/check.sh")).toBeInTheDocument()
    })

    // The disabled row should have some data-testid or content indicating disabled state
    expect(screen.getByTestId("cron-status-0")).toBeInTheDocument()
  })

  it("shows loading spinner while fetching cron jobs", async () => {
    const user = userEvent.setup()
    // Never resolves during this test
    mockGetCrons.mockReturnValue(new Promise(() => {}))
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    expect(screen.getByTestId("cron-loading")).toBeInTheDocument()
  })

  it("shows inline error message on fetch failure", async () => {
    const user = userEvent.setup()
    mockGetCrons.mockRejectedValue(new Error("SSH connection failed"))
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    await waitFor(() => {
      expect(screen.getByTestId("cron-error")).toBeInTheDocument()
    })
    // Error shown inline, not as a toast — the element should be in the document
    expect(screen.getByTestId("cron-error")).toBeInTheDocument()
  })

  it("shows empty state message when crontab has no entries", async () => {
    const user = userEvent.setup()
    mockGetCrons.mockResolvedValue({ entries: [], passthroughs: [] })
    renderDetail(baseState)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))

    await waitFor(() => {
      expect(screen.getByTestId("cron-empty")).toBeInTheDocument()
    })
  })

  it("switching back to Overview tab hides cron content and shows metrics grid", async () => {
    const user = userEvent.setup()
    mockGetCrons.mockResolvedValue({ entries: [], passthroughs: [] })
    const state: MonitorState = {
      ...baseState,
      metrics: {
        "srv-1": {
          cpu: { aggregate: { usagePercent: 42 }, cores: [] },
          memory: { total: 8e9, used: 4e9, available: 4e9, swapTotal: 0, swapUsed: 0 },
          disk: { partitions: [] },
          network: { interfaces: [] },
          process: { processes: [] },
          system: { hostname: "web-01", kernel: "5.15", uptimeSec: 3600, osName: "Ubuntu", coreCount: 2 },
        },
      },
    }
    renderDetail(state)

    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))
    await waitFor(() => {
      expect(screen.getByTestId("cron-jobs-tab")).toBeInTheDocument()
    })

    await user.click(screen.getByRole("tab", { name: /overview/i }))

    // Back to overview: metrics grid visible, cron tab hidden
    expect(screen.getByTestId("metrics-grid")).toBeInTheDocument()
    expect(screen.queryByTestId("cron-jobs-tab")).toBeNull()
  })
})

describe("ServerDetail — Add Cron dialog", () => {
  beforeEach(() => {
    mockGetCrons.mockResolvedValue({ entries: [], passthroughs: [] })
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  async function openCronTab(user: ReturnType<typeof userEvent.setup>) {
    renderDetail(baseState)
    await user.click(screen.getByRole("tab", { name: /cron jobs/i }))
    await waitFor(() => expect(screen.getByTestId("cron-jobs-tab")).toBeInTheDocument())
  }

  it("shows an Add button in the Cron Jobs tab", async () => {
    const user = userEvent.setup()
    await openCronTab(user)
    expect(screen.getByRole("button", { name: /add/i })).toBeInTheDocument()
  })

  it("clicking Add opens a dialog with six fields", async () => {
    const user = userEvent.setup()
    await openCronTab(user)

    await user.click(screen.getByRole("button", { name: /add/i }))

    // All six fields present
    expect(screen.getByLabelText(/^minute$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^hour$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/day of month/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^month$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/day of week/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^command$/i)).toBeInTheDocument()
  })

  it("shows a real-time schedule preview", async () => {
    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    // Default fields (*/*/*/*) with minute * → "Every minute"
    expect(screen.getByTestId("schedule-preview")).toBeInTheDocument()
  })

  it("Save button is disabled when a schedule field is invalid", async () => {
    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    const minuteInput = screen.getByLabelText(/minute/i)
    await user.clear(minuteInput)
    await user.type(minuteInput, "!invalid!")

    const saveBtn = screen.getByRole("button", { name: /save/i })
    expect(saveBtn).toBeDisabled()
  })

  it("invalid field gets red border styling", async () => {
    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    const minuteInput = screen.getByLabelText(/minute/i)
    await user.clear(minuteInput)
    await user.type(minuteInput, "!")

    expect(minuteInput).toHaveClass("border-destructive")
  })

  it("Save button is disabled when command is empty", async () => {
    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    // Command is empty by default
    const saveBtn = screen.getByRole("button", { name: /save/i })
    expect(saveBtn).toBeDisabled()
  })

  it("successful save closes dialog and refreshes cron list", async () => {
    const newEntry = {
      minute: "0", hour: "2", dayOfMonth: "*", month: "*", dayOfWeek: "1",
      command: "/usr/bin/job.sh", enabled: true, raw: "0 2 * * 1 /usr/bin/job.sh",
    }
    mockCreateCron.mockResolvedValue(newEntry)
    // After refresh, getCrons returns the new entry
    mockGetCrons.mockResolvedValueOnce({ entries: [], passthroughs: [] }) // initial load
    mockGetCrons.mockResolvedValueOnce({ entries: [newEntry], passthroughs: [] }) // after add

    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    // Fill in valid fields
    await user.clear(screen.getByLabelText(/minute/i))
    await user.type(screen.getByLabelText(/minute/i), "0")
    await user.clear(screen.getByLabelText(/hour/i))
    await user.type(screen.getByLabelText(/hour/i), "2")
    await user.type(screen.getByLabelText(/command/i), "/usr/bin/job.sh")

    await user.click(screen.getByRole("button", { name: /save/i }))

    // Dialog closes
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
    })

    // New entry appears in table
    await waitFor(() => {
      expect(screen.getByText("/usr/bin/job.sh")).toBeInTheDocument()
    })
  })

  it("API error shows inline error in dialog, not a toast", async () => {
    mockCreateCron.mockRejectedValue(new Error("permission denied"))

    const user = userEvent.setup()
    await openCronTab(user)
    await user.click(screen.getByRole("button", { name: /add/i }))

    await user.type(screen.getByLabelText(/command/i), "/usr/bin/job.sh")
    await user.click(screen.getByRole("button", { name: /save/i }))

    await waitFor(() => {
      expect(screen.getByTestId("add-cron-error")).toBeInTheDocument()
    })
    // Dialog should still be open
    expect(screen.getByRole("dialog")).toBeInTheDocument()
  })
})
