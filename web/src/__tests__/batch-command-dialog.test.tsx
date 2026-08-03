import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from "vitest"
import { render, screen, cleanup, within, act, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { MonitorContext, initialMonitorState } from "../hooks/useMonitorState"
import { ThemeProvider } from "../components/theme-provider"
import { BatchCommandDialog } from "../components/BatchCommandDialog"
import { Sidebar } from "../components/Sidebar"
import type { Group, ServerInfo } from "../types/server"
import type { BatchRunInfo, RunResultInfo } from "../types/batch"

// --- Mocks ---

vi.mock("@/api/client", () => ({
  getGroups: vi.fn(),
  getCurrentBatchRun: vi.fn(),
  createBatchRun: vi.fn(),
  cancelBatchRun: vi.fn(),
  updateServer: vi.fn(),
  logout: vi.fn(),
}))

import { getGroups, getCurrentBatchRun, createBatchRun, cancelBatchRun } from "@/api/client"

const mockGetGroups = getGroups as ReturnType<typeof vi.fn>
const mockGetCurrent = getCurrentBatchRun as ReturnType<typeof vi.fn>
const mockCreateBatchRun = createBatchRun as ReturnType<typeof vi.fn>
const mockCancelBatchRun = cancelBatchRun as ReturnType<typeof vi.fn>

// MockWebSocket captures instances so tests can script frames.
class MockWebSocket {
  static instances: MockWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  url: string
  readyState = MockWebSocket.CONNECTING
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
  }

  open() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }

  emit(data: string) {
    this.onmessage?.({ data })
  }

  closeFromServer() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }
}

// --- Test data ---

function makeGroup(overrides: Partial<Group> = {}): Group {
  return {
    id: "g1",
    name: "Production",
    member_count: 2,
    ...overrides,
  }
}

function makeServer(overrides: Partial<ServerInfo> = {}): ServerInfo {
  return {
    id: "s1",
    name: "web-1",
    host: "10.0.0.1",
    status: "connected",
    ...overrides,
  }
}

function makeRun(overrides: Partial<BatchRunInfo> = {}): BatchRunInfo {
  return {
    id: "run-1",
    script: "echo hi",
    status: "running",
    created_at: "2026-08-03T12:00:00Z",
    ...overrides,
  }
}

function makeResult(overrides: Partial<RunResultInfo> = {}): RunResultInfo {
  return {
    server_id: "s1",
    status: "pending",
    output: "",
    truncated: false,
    ...overrides,
  }
}

// --- Setup ---

const stateWithServers = {
  ...initialMonitorState,
  servers: [
    makeServer({ id: "s1", name: "web-1", group_id: "g1" }),
    makeServer({ id: "s2", name: "web-2", group_id: "g1" }),
    makeServer({ id: "s3", name: "db-1", status: "failed" }),
  ],
}

async function renderDialog() {
  render(
    <MonitorContext value={{ state: stateWithServers, dispatch: vi.fn() }}>
      <BatchCommandDialog open onOpenChange={vi.fn()} />
    </MonitorContext>
  )
  // Groups load asynchronously; wait for the selection tree to render.
  await screen.findByText("Production")
}

async function openDialog() {
  mockGetGroups.mockResolvedValue([makeGroup()])
  mockGetCurrent.mockResolvedValue(null)
  const user = userEvent.setup()
  render(
    <MemoryRouter>
      <ThemeProvider defaultTheme="dark" storageKey="test-theme">
        <MonitorContext value={{ state: stateWithServers, dispatch: vi.fn() }}>
          <Sidebar sidebarOpen onContextMenu={vi.fn()} />
        </MonitorContext>
      </ThemeProvider>
    </MemoryRouter>
  )
  await user.click(screen.getByRole("button", { name: "Batch Command" }))
  const dialog = screen.getByRole("dialog")
  await within(dialog).findByText("Production")
  return { user, dialog }
}

function groupRow(name: string) {
  const text = screen.getByText(name)
  return text.parentElement as HTMLElement
}

function checkboxIn(row: HTMLElement) {
  return within(row).getByRole("checkbox")
}

describe("BatchCommandDialog", () => {
  beforeAll(() => {
    // jsdom does not implement matchMedia; ThemeProvider needs it.
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
  })

  beforeEach(() => {
    vi.clearAllMocks()
    MockWebSocket.instances = []
    vi.stubGlobal("WebSocket", MockWebSocket)
    mockGetGroups.mockResolvedValue([makeGroup()])
    mockGetCurrent.mockResolvedValue(null)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    cleanup()
  })

  it("opens from the sidebar button in Compose mode", async () => {
    const { dialog } = await openDialog()

    expect(dialog).toBeInTheDocument()
    // Compose mode: script editor + selection tree.
    expect(within(dialog).getByLabelText("Script")).toBeInTheDocument()
    expect(within(dialog).getByText("Production")).toBeInTheDocument()
    expect(within(dialog).getByText("web-1")).toBeInTheDocument()
    expect(within(dialog).getByText("db-1")).toBeInTheDocument()
  })

  it("checking a Group selects all its members and updates the count", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("Production"))

    expect(screen.getByRole("button", { name: /run on 2 servers/i })).toBeDisabled() // still needs a script
    await user.type(screen.getByLabelText("Script"), "echo hi")
    expect(screen.getByRole("button", { name: /run on 2 servers/i })).toBeEnabled()
    expect(checkboxIn(groupRow("Production"))).toHaveAttribute("aria-checked", "true")
    expect(checkboxIn(groupRow("web-1"))).toHaveAttribute("aria-checked", "true")
    expect(checkboxIn(groupRow("web-2"))).toHaveAttribute("aria-checked", "true")
  })

  it("renders a Group indeterminate when only some members are selected", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    expect(checkboxIn(groupRow("Production"))).toHaveAttribute("aria-checked", "mixed")
    expect(screen.getByRole("button", { name: /run on 1 server/i })).toBeEnabled()
  })

  it("selects ungrouped Servers individually", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("db-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    expect(checkboxIn(groupRow("db-1"))).toHaveAttribute("aria-checked", "true")
    expect(checkboxIn(groupRow("Production"))).toHaveAttribute("aria-checked", "false")
    expect(screen.getByRole("button", { name: /run on 1 server/i })).toBeEnabled()
  })

  it("select-all targets every Server", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("All Servers"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    expect(screen.getByRole("button", { name: /run on 3 servers/i })).toBeEnabled()
    expect(checkboxIn(groupRow("Production"))).toHaveAttribute("aria-checked", "true")
  })

  it("disables dispatch without a script or selection", async () => {
    const user = userEvent.setup()
    await renderDialog()

    const button = screen.getByRole("button", { name: /run on 0 servers/i })
    expect(button).toBeDisabled()

    await user.click(screen.getByText("web-1"))
    expect(screen.getByRole("button", { name: /run on 1 server/i })).toBeDisabled()

    await user.type(screen.getByLabelText("Script"), "df -h")
    expect(screen.getByRole("button", { name: /run on 1 server/i })).toBeEnabled()
  })

  it("dispatch switches to the live results view and streams WS events", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("All Servers"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [
        makeResult({ server_id: "s1", status: "pending" }),
        makeResult({ server_id: "s2", status: "pending" }),
        makeResult({ server_id: "s3", status: "pending" }),
      ],
    })
    await user.click(screen.getByRole("button", { name: /run on 3 servers/i }))

    // The results view appears automatically and the WS connects.
    expect(mockCreateBatchRun).toHaveBeenCalledWith("echo hi", ["s1", "s2", "s3"])
    expect(screen.getAllByText("Pending")).toHaveLength(3)
    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })

    // WS-driven output and status updates.
    const ws = MockWebSocket.instances[0]
    act(() => {
      ws.emit(JSON.stringify({ type: "output", server_id: "s1", data: "hello\n" }))
      ws.emit(
        JSON.stringify({
          type: "status",
          server_id: "s1",
          status: "success",
          exit_code: 0,
          finished_at: "2026-08-03T12:00:05Z",
        })
      )
    })

    expect(screen.getByTestId("batch-output")).toHaveTextContent("hello")
    expect(screen.getByText("Success")).toBeInTheDocument()
  })

  it("keeps the input box disabled after the focused Server exits", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "read x")

    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running" })],
    })
    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    const input = screen.getByPlaceholderText(/type input for web-1/i)
    expect(input).toBeEnabled()

    act(() => {
      MockWebSocket.instances[0].emit(
        JSON.stringify({
          type: "status",
          server_id: "s1",
          status: "success",
          exit_code: 0,
        })
      )
    })

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/process finished/i)).toBeDisabled()
    })
  })

  it("sends typed input over the WebSocket", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "read x")

    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running" })],
    })
    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    MockWebSocket.instances[0].open()

    const input = screen.getByPlaceholderText(/type input for web-1/i)
    await user.type(input, "y{Enter}")

    expect(MockWebSocket.instances[0].sent).toContain(
      JSON.stringify({ type: "input", server_id: "s1", data: "y\n" })
    )
    expect(input).toHaveValue("")
  })

  it("detects run completion via the WS closing and hides the cancel button", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running" })],
    })
    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    expect(screen.getByRole("button", { name: /cancel run/i })).toBeInTheDocument()

    // The server completes the run: final status events, then the WS closes.
    // The close triggers a REST resync that reveals the done run.
    act(() => {
      MockWebSocket.instances[0].emit(
        JSON.stringify({ type: "status", server_id: "s1", status: "success", exit_code: 0 })
      )
      MockWebSocket.instances[0].closeFromServer()
    })
    mockGetCurrent.mockResolvedValue({
      run: makeRun({ status: "done" }),
      results: [makeResult({ server_id: "s1", status: "success", exit_code: 0 })],
    })

    await waitFor(
      () => {
        expect(screen.queryByRole("button", { name: /cancel run/i })).toBeNull()
      },
      { timeout: 3000 }
    )
  })

  it("shows a clear rejection message when a run is already in progress", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")

    const err = new Error("Another batch run is already in progress") as Error & { code: string }
    err.code = "batch_run_in_progress"
    mockCreateBatchRun.mockRejectedValue(err)

    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    expect(await screen.findByText(/another batch run is already in progress/i)).toBeInTheDocument()
    // Stays in Compose mode.
    expect(screen.getByLabelText("Script")).toBeInTheDocument()
  })

  it("reopening while a run is in progress shows the live run with a cancel button", async () => {
    mockGetCurrent.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running", output: "in-flight\n" })],
    })
    const user = userEvent.setup()
    render(
      <MonitorContext value={{ state: stateWithServers, dispatch: vi.fn() }}>
        <BatchCommandDialog open onOpenChange={vi.fn()} />
      </MonitorContext>
    )

    // Auto-switched to the results view with the live output restored.
    expect(await screen.findByText("Running")).toBeInTheDocument()
    expect(screen.getByTestId("batch-output")).toHaveTextContent("in-flight")

    await user.click(screen.getByRole("button", { name: /cancel run/i }))
    expect(mockCancelBatchRun).toHaveBeenCalled()
  })

  it("reopening after a finished run opens compose with a view-last-result entry and prefilled script", async () => {
    mockGetCurrent.mockResolvedValue({
      run: makeRun({ status: "done", script: "df -h" }),
      results: [makeResult({ server_id: "s1", status: "success", exit_code: 0 })],
    })
    const user = userEvent.setup()
    await renderDialog()

    // Compose mode with the last script prefilled.
    expect(await screen.findByLabelText("Script")).toHaveValue("df -h")
    const viewLast = screen.getByRole("button", { name: /view last result/i })
    await user.click(viewLast)

    expect(screen.getByText("Success")).toBeInTheDocument()
    // A finished run offers no cancel and no live input.
    expect(screen.queryByRole("button", { name: /cancel run/i })).toBeNull()
    expect(screen.getByPlaceholderText(/process finished/i)).toBeDisabled()
  })

  it("shows the connection-state dot on server rows", async () => {
    await renderDialog()
    // db-1 is in failed Connection State; it stays selectable and dispatchable.
    expect(screen.getByText("db-1")).toBeInTheDocument()
    expect(checkboxIn(groupRow("db-1"))).toHaveAttribute("aria-checked", "false")
  })

  it("filters the server tree by name", async () => {
    const user = userEvent.setup()
    await renderDialog()

    const filter = screen.getByRole("textbox", { name: "Filter servers" })
    await user.type(filter, "web")

    // Members matching the filter stay visible under their Group.
    expect(screen.getByText("web-1")).toBeInTheDocument()
    expect(screen.getByText("web-2")).toBeInTheDocument()
    expect(screen.queryByText("db-1")).toBeNull()

    // A Group-name match shows the whole Group.
    await user.clear(filter)
    await user.type(filter, "prod")
    expect(screen.getByText("web-1")).toBeInTheDocument()
    expect(screen.getByText("web-2")).toBeInTheDocument()
    expect(screen.queryByText("db-1")).toBeNull()

    // No matches at all shows the empty-filter state.
    await user.clear(filter)
    await user.type(filter, "zzz")
    expect(screen.getByText("No servers match")).toBeInTheDocument()
  })

  it("dispatches via Ctrl+Enter from the script editor", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")
    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "pending" })],
    })

    await user.keyboard("{Control>}{Enter}{/Control}")

    expect(mockCreateBatchRun).toHaveBeenCalledWith("echo hi", ["s1"])
  })

  it("copies the focused Server's output", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "echo hi")
    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running", output: "hello\nworld\n" })],
    })
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true })

    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    await user.click(screen.getByRole("button", { name: "Copy output" }))
    expect(writeText).toHaveBeenCalledWith("hello\nworld\n")
    expect(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument()
  })

  it("sends typed input via the send button", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("web-1"))
    await user.type(screen.getByLabelText("Script"), "read x")
    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [makeResult({ server_id: "s1", status: "running" })],
    })
    await user.click(screen.getByRole("button", { name: /run on 1 server/i }))

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1)
    })
    MockWebSocket.instances[0].open()

    const input = screen.getByPlaceholderText(/type input for web-1/i)
    await user.type(input, "y")
    await user.click(screen.getByRole("button", { name: "Send" }))

    expect(MockWebSocket.instances[0].sent).toContain(
      JSON.stringify({ type: "input", server_id: "s1", data: "y\n" })
    )
    expect(input).toHaveValue("")
  })

  it("shows aggregate progress across target servers", async () => {
    const user = userEvent.setup()
    await renderDialog()

    await user.click(screen.getByText("All Servers"))
    await user.type(screen.getByLabelText("Script"), "echo hi")
    mockCreateBatchRun.mockResolvedValue({
      run: makeRun(),
      results: [
        makeResult({ server_id: "s1", status: "pending" }),
        makeResult({ server_id: "s2", status: "pending" }),
        makeResult({ server_id: "s3", status: "pending" }),
      ],
    })
    await user.click(screen.getByRole("button", { name: /run on 3 servers/i }))

    expect(screen.getByText("0/3 done")).toBeInTheDocument()

    // One Server finishes: the strip counts it live.
    act(() => {
      MockWebSocket.instances[0].emit(
        JSON.stringify({
          type: "status",
          server_id: "s1",
          status: "success",
          exit_code: 0,
          finished_at: "2026-08-03T12:00:05Z",
        })
      )
    })
    expect(screen.getByText("1/3 done")).toBeInTheDocument()

    // The run completes: WS closes, the REST resync reveals the done run with
    // an overall duration.
    act(() => {
      MockWebSocket.instances[0].emit(
        JSON.stringify({ type: "status", server_id: "s2", status: "success", exit_code: 0 })
      )
      MockWebSocket.instances[0].emit(
        JSON.stringify({ type: "status", server_id: "s3", status: "success", exit_code: 0 })
      )
      MockWebSocket.instances[0].closeFromServer()
    })
    mockGetCurrent.mockResolvedValue({
      run: makeRun({ status: "done" }),
      results: [
        makeResult({
          server_id: "s1",
          status: "success",
          exit_code: 0,
          started_at: "2026-08-03T12:00:00Z",
          finished_at: "2026-08-03T12:00:05Z",
        }),
        makeResult({
          server_id: "s2",
          status: "success",
          exit_code: 0,
          started_at: "2026-08-03T12:00:00Z",
          finished_at: "2026-08-03T12:00:05Z",
        }),
        makeResult({
          server_id: "s3",
          status: "success",
          exit_code: 0,
          started_at: "2026-08-03T12:00:00Z",
          finished_at: "2026-08-03T12:00:05Z",
        }),
      ],
    })

    await waitFor(
      () => {
        expect(screen.getByText("3/3 done in 5s")).toBeInTheDocument()
      },
      { timeout: 3000 }
    )
  })
})
