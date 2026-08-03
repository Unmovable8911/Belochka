import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within, fireEvent, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useReducer } from "react"
import { MemoryRouter } from "react-router-dom"
import {
  MonitorContext,
  initialMonitorState,
  monitorReducer,
} from "../hooks/useMonitorState"
import Dashboard from "../pages/Dashboard"
import type { Server, ServerInfo } from "../types/server"

// --- Mock API ---

vi.mock("@/api/client", () => ({
  ApiError: class ApiError extends Error {
    code: string
    constructor(code: string, message: string) {
      super(message)
      this.code = code
      this.name = "ApiError"
    }
  },
  getGroups: vi.fn(),
  getServer: vi.fn(),
  updateServer: vi.fn(),
  deleteServer: vi.fn(),
  testConnection: vi.fn(),
  uploadKeyFile: vi.fn(),
}))

// The harness renders <Dashboard /> without a <Toaster />, so sonner's toasts
// render nothing visible. Mock the toast sink and assert on the call instead.
vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}))

import { ApiError, getGroups, getServer, deleteServer } from "@/api/client"
import { toast } from "sonner"

const mockGetGroups = getGroups as ReturnType<typeof vi.fn>
const mockGetServer = getServer as ReturnType<typeof vi.fn>
const mockDeleteServer = deleteServer as ReturnType<typeof vi.fn>
const mockToastError = toast.error as ReturnType<typeof vi.fn>

// --- Test data ---

function makeServer(overrides: Partial<ServerInfo> = {}): ServerInfo {
  return {
    id: overrides.id ?? "srv-1",
    name: overrides.name ?? "Web Server",
    host: overrides.host ?? "10.0.0.1",
    status: overrides.status ?? "connected",
    group_id: overrides.group_id,
  }
}

// Per-field nullish coalescing so partial overrides keep sibling defaults
// (the pattern CONTEXT.md prefers for new factories).
function makeFullServer(overrides: Partial<Server> = {}): Server {
  return {
    id: overrides.id ?? "srv-1",
    name: overrides.name ?? "Web Server",
    host: overrides.host ?? "10.0.0.1",
    port: overrides.port ?? 22,
    auth_type: overrides.auth_type ?? "password",
    username: overrides.username ?? "root",
    group_id: overrides.group_id ?? "g1",
    created_at: overrides.created_at ?? "2026-01-01T00:00:00Z",
    updated_at: overrides.updated_at ?? "2026-01-01T00:00:00Z",
  }
}

// --- Harness ---

// A stateful wrapper so dispatch actually updates state: after Delete confirms
// we can assert the card disappears from the grid.
function renderDashboard(servers: ServerInfo[] = []) {
  function Wrapper() {
    const [state, dispatch] = useReducer(monitorReducer, { ...initialMonitorState, servers })
    return (
      <MonitorContext value={{ state, dispatch }}>
        <MemoryRouter>
          <Dashboard />
        </MemoryRouter>
      </MonitorContext>
    )
  }
  const view = render(<Wrapper />)
  return view
}

describe("Dashboard server card context menu", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetGroups.mockResolvedValue([])
    mockGetServer.mockResolvedValue(makeFullServer())
    mockDeleteServer.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    cleanup()
  })

  it("shows Edit/Delete/Console menu on right-click of a card", async () => {
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))

    expect(screen.getByRole("menu")).toBeInTheDocument()
    expect(screen.getByRole("menuitem", { name: /edit/i })).toBeInTheDocument()
    expect(screen.getByRole("menuitem", { name: /delete/i })).toBeInTheDocument()
    expect(screen.getByRole("menuitem", { name: /console/i })).toBeInTheDocument()
  })

  it("opens the Console in a new browser tab", async () => {
    const openSpy = vi.spyOn(window, "open").mockReturnValue(null)
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    await user.click(screen.getByRole("menuitem", { name: /console/i }))

    expect(openSpy).toHaveBeenCalledWith("/server/srv-1/console", "_blank")
    // Menu closes after the item is picked.
    expect(screen.queryByRole("menu")).toBeNull()
  })

  it("fetches the full Server and opens the edit dialog on Edit", async () => {
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    await user.click(screen.getByRole("menuitem", { name: /edit/i }))

    await waitFor(() => {
      expect(mockGetServer).toHaveBeenCalledWith("srv-1")
    })
    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("Edit Server")
    // The fetched record populates the form, not just the dashboard's slim copy.
    expect(within(dialog).getByDisplayValue("Web Server")).toBeInTheDocument()
    expect(within(dialog).getByDisplayValue("root")).toBeInTheDocument()
  })

  it("surfaces a toast when the Edit fetch fails", async () => {
    mockGetServer.mockRejectedValueOnce(new ApiError("load_failed", "boom"))
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    await user.click(screen.getByRole("menuitem", { name: /edit/i }))

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith("boom")
    })
    expect(screen.queryByRole("dialog")).toBeNull()
  })

  it("confirms before deleting, then removes the card", async () => {
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    await user.click(screen.getByRole("menuitem", { name: /delete/i }))

    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("Delete Server")

    // Not deleted until confirmed.
    expect(mockDeleteServer).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole("button", { name: "Delete" }))

    await waitFor(() => {
      expect(mockDeleteServer).toHaveBeenCalledWith("srv-1")
    })
    await waitFor(() => {
      expect(screen.queryByText("Web Server")).toBeNull()
    })
  })

  it("cancels the delete dialog without deleting", async () => {
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    await user.click(screen.getByRole("menuitem", { name: /delete/i }))
    await screen.findByRole("dialog")

    await user.click(screen.getByRole("button", { name: "Cancel" }))

    expect(screen.queryByRole("dialog")).toBeNull()
    expect(mockDeleteServer).not.toHaveBeenCalled()
  })

  it("keeps the browser default menu on blank dashboard space", () => {
    const { container } = renderDashboard([makeServer()])

    // Blank space = the grid container itself (outside any card).
    const grid = container.querySelector("[data-testid='server-grid']") as HTMLElement
    fireEvent.contextMenu(grid)

    expect(screen.queryByRole("menu")).toBeNull()
  })

  it("closes the menu when clicking elsewhere", async () => {
    const user = userEvent.setup()
    renderDashboard([makeServer()])
    await screen.findByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    expect(screen.getByRole("menu")).toBeInTheDocument()

    await user.click(screen.getByTestId("context-menu-backdrop"))

    expect(screen.queryByRole("menu")).toBeNull()
  })

  // The backdrop is `fixed inset-0` above the cards, so in a real browser a
  // second right-click anywhere (including on another card) hits the backdrop,
  // whose onContextMenu closes the menu — it is not replaced. Story 10 wants
  // the menu closed on right-click again.
  it("closes the menu when right-clicking again", () => {
    renderDashboard([makeServer()])
    screen.getByText("Web Server")

    fireEvent.contextMenu(screen.getByRole("link", { name: /web server/i }))
    expect(screen.getByRole("menu")).toBeInTheDocument()

    fireEvent.contextMenu(screen.getByTestId("context-menu-backdrop"))

    expect(screen.queryByRole("menu")).toBeNull()
  })
})
