import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from "vitest"
import { render, screen, cleanup, within, fireEvent, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { RouterProvider, createMemoryRouter } from "react-router-dom"
import { MonitorContext, initialMonitorState } from "../hooks/useMonitorState"
import { ThemeProvider } from "../components/theme-provider"
import { Layout } from "../components/Layout"
import type { Group, ServerInfo } from "../types/server"

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
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  deleteGroup: vi.fn(),
  updateServer: vi.fn(),
  logout: vi.fn(),
}))

import { ApiError, getGroups, createGroup, updateGroup, deleteGroup, updateServer } from "@/api/client"

const mockGetGroups = getGroups as ReturnType<typeof vi.fn>
const mockCreateGroup = createGroup as ReturnType<typeof vi.fn>
const mockUpdateGroup = updateGroup as ReturnType<typeof vi.fn>
const mockDeleteGroup = deleteGroup as ReturnType<typeof vi.fn>
const mockUpdateServer = updateServer as ReturnType<typeof vi.fn>

// --- DnD helpers ---

// jsdom does not implement DataTransfer; a minimal mock is enough for the
// server drag/drop handlers (setData on dragStart, getData on drop).
function makeDataTransfer(): Record<string, unknown> {
  const data = new Map<string, string>()
  return {
    setData: (type: string, value: string) => { data.set(type, value) },
    getData: (type: string) => data.get(type) ?? "",
    effectAllowed: "move",
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

const groups: Group[] = [
  makeGroup({ id: "g1", name: "Production", member_count: 2 }),
  makeGroup({ id: "g2", name: "Database", member_count: 1 }),
]

const servers: ServerInfo[] = [
  { id: "srv-1", name: "web-1", host: "10.0.0.1", status: "connected", group_id: "g1" },
  { id: "srv-2", name: "db-1", host: "10.0.0.2", status: "connected", group_id: "g2" },
]

// --- Harness ---

function renderSidebar(initialEntries: string[] = ["/"], testServers: ServerInfo[] = servers) {
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: (
          <Layout>
            <div>main content</div>
          </Layout>
        ),
      },
    ],
    { initialEntries }
  )
  render(
    <MonitorContext value={{ state: { ...initialMonitorState, servers: testServers }, dispatch: vi.fn() }}>
      <ThemeProvider>
        <RouterProvider router={router} />
      </ThemeProvider>
    </MonitorContext>
  )
  return { router }
}

describe("Sidebar group management UX", () => {
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
    mockGetGroups.mockResolvedValue(groups)
  })

  afterEach(() => {
    cleanup()
  })

  it("shows a hover delete quick-action button on each group row", async () => {
    renderSidebar()
    await screen.findByText("Production")

    expect(screen.getByRole("button", { name: "Delete Production" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Delete Database" })).toBeInTheDocument()
    // No subgroup creation entry point exists.
    expect(screen.queryByRole("button", { name: /Add subgroup/ })).toBeNull()
  })

  it("opens a delete confirmation dialog from the hover button and deletes on confirm", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    await user.click(screen.getByRole("button", { name: "Delete Production" }))
    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("Delete Group")
    // The confirm message states the ungrouping consequence.
    expect(dialog).toHaveTextContent(/become ungrouped/)

    await user.click(within(dialog).getByRole("button", { name: "Delete" }))
    await waitFor(() => {
      expect(mockDeleteGroup).toHaveBeenCalledWith("g1")
    })
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull()
    })
  })

  it("cancels out of the delete dialog without deleting", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    await user.click(screen.getByRole("button", { name: "Delete Production" }))
    await screen.findByRole("dialog")

    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(screen.queryByRole("dialog")).toBeNull()
    expect(mockDeleteGroup).not.toHaveBeenCalled()
  })

  it("closes the delete dialog on Escape without deleting", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    await user.click(screen.getByRole("button", { name: "Delete Production" }))
    await screen.findByRole("dialog")

    await user.keyboard("{Escape}")
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull()
    })
    expect(mockDeleteGroup).not.toHaveBeenCalled()
  })

  it("keeps the delete dialog open after a failed delete", async () => {
    mockDeleteGroup.mockRejectedValueOnce(new ApiError("delete_failed", "boom"))
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    await user.click(screen.getByRole("button", { name: "Delete Production" }))
    const dialog = await screen.findByRole("dialog")
    await user.click(within(dialog).getByRole("button", { name: "Delete" }))

    // The failing request settles (Deleting... reverts to Delete) and the
    // dialog stays open so the user can retry or cancel.
    await waitFor(() => {
      expect(within(dialog).queryByRole("button", { name: "Deleting..." })).toBeNull()
    })
    expect(screen.getByRole("dialog")).toBeInTheDocument()
  })

  it("shows group context-menu items (rename, delete) and opens the delete confirmation dialog", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByText("Production"))
    expect(screen.getByRole("menuitem", { name: /rename/i })).toBeInTheDocument()
    expect(screen.getByRole("menuitem", { name: /delete/i })).toBeInTheDocument()
    // No subgroup or group-moving entries remain.
    expect(screen.queryByRole("menuitem", { name: /new subgroup/i })).toBeNull()
    expect(screen.queryByRole("menuitem", { name: /move to/i })).toBeNull()

    await user.click(screen.getByRole("menuitem", { name: /delete/i }))
    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("Delete Group")
    expect(mockDeleteGroup).not.toHaveBeenCalled()
  })

  it("right-clicking blank tree-area space shows a New Group menu", async () => {
    renderSidebar()
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByRole("navigation"))
    expect(screen.getByRole("menuitem", { name: /new group/i })).toBeInTheDocument()
  })

  it("creates a root-level group from the blank-space New Group menu", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByRole("navigation"))
    await user.click(screen.getByRole("menuitem", { name: /new group/i }))

    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("New Group")
    await user.type(within(dialog).getByLabelText("Group name"), "Frontend")
    await user.click(within(dialog).getByRole("button", { name: "Save" }))

    await waitFor(() => {
      expect(mockCreateGroup).toHaveBeenCalledWith({ name: "Frontend" })
    })
  })

  it("opens a rename dialog prefilled with the current name and renames on save", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByText("Production"))
    await user.click(screen.getByRole("menuitem", { name: /rename/i }))

    const dialog = await screen.findByRole("dialog")
    expect(dialog).toHaveTextContent("Rename Group")
    const input = within(dialog).getByLabelText("Group name")
    expect(input).toHaveValue("Production")

    await user.clear(input)
    await user.type(input, "Prod")
    await user.click(within(dialog).getByRole("button", { name: "Save" }))

    await waitFor(() => {
      expect(mockUpdateGroup).toHaveBeenCalledWith("g1", { name: "Prod" })
    })
  })

  it("shows an inline error in the create dialog for a duplicate group name", async () => {
    mockCreateGroup.mockRejectedValueOnce(new ApiError("duplicate_name", "group already exists"))
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByRole("navigation"))
    await user.click(screen.getByRole("menuitem", { name: /new group/i }))
    const dialog = await screen.findByRole("dialog")
    await user.type(within(dialog).getByLabelText("Group name"), "Database")
    await user.click(within(dialog).getByRole("button", { name: "Save" }))

    const alert = await screen.findByRole("alert")
    expect(alert).toHaveTextContent("A group with this name already exists")
  })

  it("resets the filter to All Servers after deleting the currently-filtered group", async () => {
    const user = userEvent.setup()
    const { router } = renderSidebar(["/?group_id=g2"])
    await screen.findByText("Database")

    await user.click(screen.getByRole("button", { name: "Delete Database" }))
    const dialog = await screen.findByRole("dialog")
    await user.click(within(dialog).getByRole("button", { name: "Delete" }))

    await waitFor(() => {
      expect(mockDeleteGroup).toHaveBeenCalledWith("g2")
    })
    await waitFor(() => {
      expect(router.state.location.search).toBe("")
    })
  })

  it("auto-expands a group created after initial load", async () => {
    const user = userEvent.setup()
    // Refetch (after create) must return the new group for the seeding to
    // see it; a mutable mock list captures that.
    let currentGroups = groups
    mockGetGroups.mockImplementation(() => Promise.resolve(currentGroups))
    renderSidebar(["/"], [
      ...servers,
      { id: "srv-3", name: "front-1", host: "10.0.0.3", status: "connected", group_id: "g3" },
    ])
    await screen.findByText("Production")

    fireEvent.contextMenu(screen.getByRole("navigation"))
    await user.click(screen.getByRole("menuitem", { name: /new group/i }))
    const dialog = await screen.findByRole("dialog")
    await user.type(within(dialog).getByLabelText("Group name"), "Frontend")
    currentGroups = [...groups, makeGroup({ id: "g3", name: "Frontend", member_count: 1 })]
    await user.click(within(dialog).getByRole("button", { name: "Save" }))

    // The freshly created group is seeded and auto-expands: its member server
    // renders immediately (member rows only show while a group is expanded).
    await screen.findByText("front-1")
    const row = screen.getByText("Frontend").closest("div") as HTMLElement
    expect(within(row).getByRole("button", { name: "Collapse" })).toBeInTheDocument()
  })

  it("collapses a group row and hides its servers, including the last expanded group", async () => {
    const user = userEvent.setup()
    renderSidebar()
    await screen.findByText("Production")

    // web-1 under Production, db-1 under Database (both groups start expanded).
    // Groups expand via an effect, so wait for a member server to appear.
    await screen.findByText("web-1")
    expect(screen.getAllByText("web-1")).toHaveLength(1)
    expect(screen.getAllByText("db-1")).toHaveLength(1)

    // Collapse Database
    const dbRow = screen.getByText("Database").closest("div") as HTMLElement
    await user.click(within(dbRow).getByRole("button", { name: "Collapse" }))
    expect(screen.queryByText("db-1")).toBeNull()
    expect(screen.getByText("web-1")).toBeInTheDocument()

    // Collapse Production — now no group is expanded, and they must stay collapsed
    const prodRow = screen.getByText("Production").closest("div") as HTMLElement
    await user.click(within(prodRow).getByRole("button", { name: "Collapse" }))
    expect(screen.queryByText("web-1")).toBeNull()
    expect(screen.queryByText("db-1")).toBeNull()
  })

  it("renders ungrouped servers at the root, as siblings of the groups, with no virtual nodes", async () => {
    const withUngrouped: ServerInfo[] = [
      ...servers,
      { id: "srv-3", name: "standalone-1", host: "10.0.0.3", status: "connected", group_id: null },
      { id: "srv-4", name: "alpha-1", host: "10.0.0.4", status: "connected", group_id: null },
    ]
    renderSidebar(["/"], withUngrouped)
    await screen.findByText("Production")

    // The virtual "All Servers" and "Ungrouped" nodes no longer exist.
    expect(screen.queryByText("All Servers")).toBeNull()
    expect(screen.queryByText("Ungrouped")).toBeNull()

    // Ungrouped servers are visible immediately, directly at the root, sorted.
    await screen.findByText("alpha-1")
    expect(screen.getByText("standalone-1")).toBeInTheDocument()
  })

  it("drops a server on blank tree space to ungroup it", async () => {
    renderSidebar()
    await screen.findByText("Production")

    const dt = makeDataTransfer()
    const serverRow = await screen.findByText("web-1")
    fireEvent.dragStart(serverRow, { dataTransfer: dt } as unknown as EventInit)
    // Blank tree area = the whole <nav>
    const nav = screen.getByRole("navigation")
    fireEvent.dragOver(nav, { dataTransfer: dt } as unknown as EventInit)
    fireEvent.drop(nav, { dataTransfer: dt } as unknown as EventInit)
    fireEvent.dragEnd(serverRow, { dataTransfer: dt } as unknown as EventInit)

    await waitFor(() => {
      expect(mockUpdateServer).toHaveBeenCalledWith("srv-1", { group_id: null })
    })
  })
})
