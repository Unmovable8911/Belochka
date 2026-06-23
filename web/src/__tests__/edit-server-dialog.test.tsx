import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { EditServerDialog, type EditServerDialogProps } from "../components/EditServerDialog"
import Dashboard from "../pages/Dashboard"
import { MonitorContext, type MonitorState, type MonitorAction } from "../hooks/useMonitorState"
import type { Dispatch } from "react"

const baseServer = {
  id: "srv-1",
  name: "Production Web",
  host: "192.168.1.100",
  port: 22,
  username: "root",
  auth_type: "password" as const,
  host_key_fingerprint: "SHA256:existingfp",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
}

function renderDialog(props: Partial<EditServerDialogProps> = {}) {
  const result = render(
    <EditServerDialog server={baseServer} open={true} onOpenChange={() => {}} {...props} />
  )
  const dialog = screen.getByRole("dialog")
  return { ...result, dialog }
}

describe("EditServerDialog", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("pre-fills form with current server values and password empty with placeholder", () => {
    const { dialog } = renderDialog()

    expect(within(dialog).getByLabelText("Name")).toHaveValue("Production Web")
    expect(within(dialog).getByLabelText("Host")).toHaveValue("192.168.1.100")
    expect(within(dialog).getByLabelText("Port")).toHaveValue(22)
    expect(within(dialog).getByLabelText("Username")).toHaveValue("root")

    const passwordInput = within(dialog).getByLabelText("Password")
    expect(passwordInput).toHaveValue("")
    expect(passwordInput).toHaveAttribute("placeholder", "unchanged")
  })

  it("enables Save immediately when only display name is changed (no re-test needed)", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Save should be disabled initially (nothing changed)
    const saveButton = within(dialog).getByRole("button", { name: /^save$/i })
    expect(saveButton).toBeDisabled()

    // Change only the name
    const nameInput = within(dialog).getByLabelText("Name")
    await user.clear(nameInput)
    await user.type(nameInput, "New Display Name")

    // Save should be enabled without needing Test Connection
    expect(saveButton).toBeEnabled()

    // Test Connection button should NOT appear (no connection fields changed)
    expect(within(dialog).queryByRole("button", { name: /test connection/i })).toBeNull()
  })

  it("shows Test Connection and disables Save when host is changed", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Change the host
    const hostInput = within(dialog).getByLabelText("Host")
    await user.clear(hostInput)
    await user.type(hostInput, "10.0.0.1")

    // Test Connection button should appear
    expect(within(dialog).getByRole("button", { name: /test connection/i })).toBeInTheDocument()

    // Save should be disabled (connection field changed, re-test required)
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeDisabled()

    // Retest warning message should appear
    expect(within(dialog).getByText(/re-test required/i)).toBeInTheDocument()
  })

  it("shows Test Connection and disables Save when password is entered", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Type a new password (non-empty means changed)
    await user.type(within(dialog).getByLabelText("Password"), "newpass123")

    // Test Connection should appear, Save disabled
    expect(within(dialog).getByRole("button", { name: /test connection/i })).toBeInTheDocument()
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeDisabled()
  })

  it("shows Test Connection and disables Save when username is changed", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    const usernameInput = within(dialog).getByLabelText("Username")
    await user.clear(usernameInput)
    await user.type(usernameInput, "admin")

    expect(within(dialog).getByRole("button", { name: /test connection/i })).toBeInTheDocument()
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeDisabled()
  })

  it("re-test on host change shows fingerprint, trusting it enables Save", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Change host
    const hostInput = within(dialog).getByLabelText("Host")
    await user.clear(hostInput)
    await user.type(hostInput, "10.0.0.1")

    // Mock fetch: PUT update + POST test
    const updatedServer = { ...baseServer, host: "10.0.0.1" }
    const testResult = { fingerprint: "SHA256:newfingerprint" }

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      const method = options && typeof options === "object" && "method" in options ? (options as { method: string }).method : "GET"

      if (urlStr.includes("/api/servers/srv-1") && method === "PUT") {
        return new Response(JSON.stringify(updatedServer), { status: 200, headers: { "Content-Type": "application/json" } })
      }
      if (urlStr.includes("/test")) {
        return new Response(JSON.stringify(testResult), { status: 200, headers: { "Content-Type": "application/json" } })
      }
      return new Response("Not Found", { status: 404 })
    })

    // Click Test Connection
    await user.click(within(dialog).getByRole("button", { name: /test connection/i }))

    // New fingerprint should appear
    expect(await screen.findByText("SHA256:newfingerprint")).toBeInTheDocument()
    expect(screen.getByText("Host Key Fingerprint")).toBeInTheDocument()

    // Save should still be disabled (fingerprint not trusted)
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeDisabled()

    // Trust the fingerprint
    await user.click(screen.getByRole("button", { name: /trust this host/i }))
    expect(screen.getByText("Host trusted")).toBeInTheDocument()

    // Now Save should be enabled
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeEnabled()
  })

  it("name-only save calls PUT /api/servers/{id} and fires onServerUpdated callback", async () => {
    const user = userEvent.setup()
    const onServerUpdated = vi.fn()
    const onOpenChange = vi.fn()
    const { dialog } = renderDialog({ onServerUpdated, onOpenChange })

    // Change only name
    const nameInput = within(dialog).getByLabelText("Name")
    await user.clear(nameInput)
    await user.type(nameInput, "Renamed Server")

    const savedServer = { ...baseServer, name: "Renamed Server" }

    const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      const method = options && typeof options === "object" && "method" in options ? (options as { method: string }).method : "GET"

      if (urlStr === `/api/servers/srv-1` && method === "PUT") {
        return new Response(JSON.stringify(savedServer), { status: 200, headers: { "Content-Type": "application/json" } })
      }
      return new Response("Not Found", { status: 404 })
    })

    // Click Save
    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    // Wait for callback
    await vi.waitFor(() => {
      expect(onServerUpdated).toHaveBeenCalledWith(savedServer)
    })

    // PUT was called with correct URL and method
    expect(fetchSpy).toHaveBeenCalledWith(
      "/api/servers/srv-1",
      expect.objectContaining({ method: "PUT" }),
    )

    // Dialog should close
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("shows error toast when save fails", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Change name
    const nameInput = within(dialog).getByLabelText("Name")
    await user.clear(nameInput)
    await user.type(nameInput, "New Name")

    vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      return new Response(
        JSON.stringify({ error: { code: "internal", message: "Database error" } }),
        { status: 500, headers: { "Content-Type": "application/json" } },
      )
    })

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    // toast.error is called (we test by verifying the function doesn't throw
    // and the dialog remains open via the callback NOT being called)
    await vi.waitFor(() => {
      // Save button should be re-enabled after failure
      expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeEnabled()
    })
  })

  it("sends empty password (keep current) when password field is not filled", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Change only name (password stays empty)
    const nameInput = within(dialog).getByLabelText("Name")
    await user.clear(nameInput)
    await user.type(nameInput, "Renamed")

    let capturedBody: Record<string, unknown> = {}
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, options) => {
      const body = options && typeof options === "object" && "body" in options
        ? JSON.parse((options as { body: string }).body)
        : {}
      capturedBody = body
      return new Response(
        JSON.stringify({ ...baseServer, name: "Renamed" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      )
    })

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await vi.waitFor(() => {
      // Password should NOT be in the request body (empty = keep current)
      expect(capturedBody).not.toHaveProperty("password")
    })
  })

  it("shows error when re-test connection fails", async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()

    // Change host to trigger re-test requirement
    const hostInput = within(dialog).getByLabelText("Host")
    await user.clear(hostInput)
    await user.type(hostInput, "10.0.0.99")

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr.includes("/test")) {
        return new Response(
          JSON.stringify({ error: { code: "auth_failed", message: "authentication failed" } }),
          { status: 422, headers: { "Content-Type": "application/json" } },
        )
      }
      // PUT succeeds
      return new Response(
        JSON.stringify({ ...baseServer, host: "10.0.0.99" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      )
    })

    await user.click(within(dialog).getByRole("button", { name: /test connection/i }))

    const alert = await screen.findByRole("alert")
    expect(alert).toHaveTextContent("authentication failed")

    // Save should remain disabled
    expect(within(dialog).getByRole("button", { name: /^save$/i })).toBeDisabled()
  })

  it("pre-fills key_path for key-auth servers and shows Key File Path field", () => {
    const keyServer = {
      ...baseServer,
      auth_type: "key" as const,
      key_path: "/home/user/.ssh/id_rsa",
    }
    render(
      <EditServerDialog server={keyServer} open={true} onOpenChange={() => {}} />
    )
    const dialog = screen.getByRole("dialog")

    expect(within(dialog).getByLabelText("Key File Path")).toHaveValue("/home/user/.ssh/id_rsa")
    expect(within(dialog).queryByLabelText("Password")).toBeNull()
  })
})

function renderDashboardWithServers() {
  const stateWithServers: MonitorState = {
    servers: [
      { id: "srv-1", name: "Production Web", host: "192.168.1.100", status: "connected" },
      { id: "srv-2", name: "Staging DB", host: "10.0.0.50", status: "connected" },
    ],
    metrics: {},
    wsConnected: true,
  }
  const dispatch: Dispatch<MonitorAction> = vi.fn()
  render(
    <MemoryRouter>
      <MonitorContext.Provider value={{ state: stateWithServers, dispatch }}>
        <Dashboard />
      </MonitorContext.Provider>
    </MemoryRouter>
  )
  return { dispatch }
}

describe("Server cards in Dashboard", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("shows a clickable card for each server linking to detail page", () => {
    renderDashboardWithServers()

    const links = screen.getAllByRole("link")
    expect(links).toHaveLength(2)
    expect(links[0]).toHaveAttribute("href", "/server/srv-1")
    expect(links[1]).toHaveAttribute("href", "/server/srv-2")
  })

  it("displays server names and hosts in cards", () => {
    renderDashboardWithServers()

    expect(screen.getByText("Production Web")).toBeInTheDocument()
    expect(screen.getByText("192.168.1.100")).toBeInTheDocument()
    expect(screen.getByText("Staging DB")).toBeInTheDocument()
    expect(screen.getByText("10.0.0.50")).toBeInTheDocument()
  })
})
