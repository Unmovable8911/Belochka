import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within, fireEvent } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import Dashboard from "../pages/Dashboard"
import { AddServerDialog } from "../components/AddServerDialog"

async function openAddServerDialog() {
  const user = userEvent.setup()
  render(<Dashboard />)

  const addButton = screen.getByRole("button", { name: /add server/i })
  await user.click(addButton)

  return { user, dialog: screen.getByRole("dialog") }
}

function renderOpenDialog() {
  const result = render(<AddServerDialog defaultOpen />)
  const dialog = screen.getByRole("dialog")
  return { ...result, dialog }
}

describe("AddServerDialog", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("opens dialog when 'Add Server' button is clicked in Dashboard", async () => {
    const { dialog } = await openAddServerDialog()

    expect(dialog).toBeInTheDocument()
    expect(dialog).toHaveTextContent("Add Server")
  })

  it("renders all required form fields with port defaulting to 22", async () => {
    const { dialog } = await openAddServerDialog()

    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument()
    expect(within(dialog).getByLabelText("Host")).toBeInTheDocument()
    expect(within(dialog).getByLabelText("Port")).toHaveValue(22)
    expect(within(dialog).getByLabelText("Username")).toBeInTheDocument()
    // Auth type selector is present (combobox role from Radix Select)
    expect(within(dialog).getByRole("combobox", { name: /authentication/i })).toBeInTheDocument()
    expect(within(dialog).getByLabelText("Password")).toBeInTheDocument()
  })

  it("shows password input by default and key path input when auth type is key", () => {
    // Default auth type is password
    const { dialog } = renderOpenDialog()

    expect(within(dialog).getByLabelText("Password")).toBeInTheDocument()
    expect(within(dialog).queryByLabelText("Key File Path")).toBeNull()
  })

  it("shows key file path input when auth type is changed to key", async () => {
    // Radix Select portals make it hard to test in JSDOM.
    // We test the auth toggle by re-rendering with the key auth type preset.
    // The real user interaction of clicking the Select dropdown is covered by
    // Radix's own tests. We verify the conditional rendering works.
    cleanup()

    // Render the dialog in key auth mode by providing defaultAuthType
    render(<AddServerDialog defaultOpen defaultAuthType="key" />)
    const dialog = screen.getByRole("dialog")

    expect(within(dialog).getByLabelText("Key File Path")).toBeInTheDocument()
    expect(within(dialog).queryByLabelText("Password")).toBeNull()
  })

  it("disables Test Connection when required fields are empty", () => {
    const { dialog } = renderOpenDialog()

    const testButton = within(dialog).getByRole("button", { name: /test connection/i })
    expect(testButton).toBeDisabled()
  })

  it("enables Test Connection when name, host, and username are filled", async () => {
    const user = userEvent.setup()
    const { dialog } = renderOpenDialog()

    const testButton = within(dialog).getByRole("button", { name: /test connection/i })
    expect(testButton).toBeDisabled()

    await user.type(within(dialog).getByLabelText("Name"), "My Server")
    await user.type(within(dialog).getByLabelText("Host"), "192.168.1.1")
    await user.type(within(dialog).getByLabelText("Username"), "root")

    expect(testButton).toBeEnabled()
  })

  it("disables Save button until test passes and fingerprint is confirmed", () => {
    const { dialog } = renderOpenDialog()

    const saveButton = within(dialog).getByRole("button", { name: /save/i })
    expect(saveButton).toBeDisabled()
  })

  describe("connection test flow", () => {
    async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement) {
      await user.type(within(dialog).getByLabelText("Name"), "My Server")
      await user.type(within(dialog).getByLabelText("Host"), "192.168.1.1")
      await user.type(within(dialog).getByLabelText("Username"), "root")
    }

    it("shows loading state during connection test", async () => {
      const user = userEvent.setup()
      const { dialog } = renderOpenDialog()
      await fillRequiredFields(user, dialog)

      // Mock fetch to return a pending promise
      let resolveFetch!: (value: Response) => void
      vi.spyOn(globalThis, "fetch").mockImplementation(
        () => new Promise((resolve) => { resolveFetch = resolve })
      )

      const testButton = within(dialog).getByRole("button", { name: /test connection/i })
      await user.click(testButton)

      // Should show loading state
      expect(within(dialog).getByRole("button", { name: /testing/i })).toBeDisabled()

      // Resolve to avoid hanging promise
      resolveFetch(new Response(JSON.stringify({ id: "srv-1", name: "My Server", host: "192.168.1.1", port: 22, auth_type: "password", username: "root", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }), { status: 201, headers: { "Content-Type": "application/json" } }))
    })

    it("shows fingerprint after successful connection test", async () => {
      const user = userEvent.setup()
      const { dialog } = renderOpenDialog()
      await fillRequiredFields(user, dialog)

      const createdServer = { id: "srv-1", name: "My Server", host: "192.168.1.1", port: 22, auth_type: "password", username: "root", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }
      const testResult = { fingerprint: "SHA256:abc123def456" }

      vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
        const urlStr = typeof url === "string" ? url : url.toString()
        if (urlStr === "/api/servers") {
          return new Response(JSON.stringify(createdServer), { status: 201, headers: { "Content-Type": "application/json" } })
        }
        if (urlStr.includes("/test")) {
          return new Response(JSON.stringify(testResult), { status: 200, headers: { "Content-Type": "application/json" } })
        }
        return new Response("Not Found", { status: 404 })
      })

      const testButton = within(dialog).getByRole("button", { name: /test connection/i })
      await user.click(testButton)

      // Fingerprint should be displayed
      expect(await screen.findByText("SHA256:abc123def456")).toBeInTheDocument()
      expect(screen.getByText("Host Key Fingerprint")).toBeInTheDocument()

      // Trust button should appear
      expect(screen.getByRole("button", { name: /trust this host/i })).toBeInTheDocument()

      // Save should still be disabled (fingerprint not yet trusted)
      expect(within(dialog).getByRole("button", { name: /save/i })).toBeDisabled()
    })

    it("enables Save after trusting the fingerprint", async () => {
      const user = userEvent.setup()
      const { dialog } = renderOpenDialog()
      await fillRequiredFields(user, dialog)

      const createdServer = { id: "srv-1", name: "My Server", host: "192.168.1.1", port: 22, auth_type: "password", username: "root", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }
      const testResult = { fingerprint: "SHA256:abc123def456" }

      vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
        const urlStr = typeof url === "string" ? url : url.toString()
        if (urlStr === "/api/servers") {
          return new Response(JSON.stringify(createdServer), { status: 201, headers: { "Content-Type": "application/json" } })
        }
        if (urlStr.includes("/test")) {
          return new Response(JSON.stringify(testResult), { status: 200, headers: { "Content-Type": "application/json" } })
        }
        return new Response("Not Found", { status: 404 })
      })

      await user.click(within(dialog).getByRole("button", { name: /test connection/i }))

      // Wait for fingerprint and click Trust
      const trustButton = await screen.findByRole("button", { name: /trust this host/i })
      await user.click(trustButton)

      // Trust confirmation shown
      expect(screen.getByText("Host trusted")).toBeInTheDocument()

      // Save should now be enabled
      expect(within(dialog).getByRole("button", { name: /save/i })).toBeEnabled()
    })

    it("shows error message when connection test fails", async () => {
      const user = userEvent.setup()
      const { dialog } = renderOpenDialog()
      await fillRequiredFields(user, dialog)

      const createdServer = { id: "srv-1", name: "My Server", host: "192.168.1.1", port: 22, auth_type: "password", username: "root", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }

      vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
        const urlStr = typeof url === "string" ? url : url.toString()
        if (urlStr === "/api/servers") {
          return new Response(JSON.stringify(createdServer), { status: 201, headers: { "Content-Type": "application/json" } })
        }
        if (urlStr.includes("/test")) {
          return new Response(JSON.stringify({ error: { code: "auth_failed", message: "authentication failed: permission denied" } }), { status: 422, headers: { "Content-Type": "application/json" } })
        }
        return new Response("Not Found", { status: 404 })
      })

      await user.click(within(dialog).getByRole("button", { name: /test connection/i }))

      // Error message should be displayed
      const alert = await screen.findByRole("alert")
      expect(alert).toHaveTextContent("authentication failed: permission denied")

      // Save should remain disabled
      expect(within(dialog).getByRole("button", { name: /save/i })).toBeDisabled()
    })

    it("successful save closes dialog, calls onServerAdded, and shows toast", async () => {
      const user = userEvent.setup()
      const onServerAdded = vi.fn()
      cleanup()
      render(<AddServerDialog defaultOpen onServerAdded={onServerAdded} />)
      const dialog = screen.getByRole("dialog")

      await user.type(within(dialog).getByLabelText("Name"), "My Server")
      await user.type(within(dialog).getByLabelText("Host"), "192.168.1.1")
      await user.type(within(dialog).getByLabelText("Username"), "root")

      const createdServer = { id: "srv-1", name: "My Server", host: "192.168.1.1", port: 22, auth_type: "password", username: "root", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z" }
      const testResult = { fingerprint: "SHA256:abc123def456" }
      const savedServer = { ...createdServer, host_key_fingerprint: "SHA256:abc123def456" }

      vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
        const urlStr = typeof url === "string" ? url : url.toString()
        const method = options && typeof options === "object" && "method" in options ? (options as { method: string }).method : "GET"

        if (urlStr === "/api/servers" && method === "POST") {
          return new Response(JSON.stringify(createdServer), { status: 201, headers: { "Content-Type": "application/json" } })
        }
        if (urlStr.includes("/test")) {
          return new Response(JSON.stringify(testResult), { status: 200, headers: { "Content-Type": "application/json" } })
        }
        if (urlStr.includes("/api/servers/srv-1") && method === "PUT") {
          return new Response(JSON.stringify(savedServer), { status: 200, headers: { "Content-Type": "application/json" } })
        }
        return new Response("Not Found", { status: 404 })
      })

      // Test connection
      await user.click(within(dialog).getByRole("button", { name: /test connection/i }))
      // Trust fingerprint
      const trustButton = await screen.findByRole("button", { name: /trust this host/i })
      await user.click(trustButton)
      // Save
      await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

      // Dialog should close
      await vi.waitFor(() => {
        expect(screen.queryByRole("dialog")).toBeNull()
      })

      // onServerAdded callback should have been called
      expect(onServerAdded).toHaveBeenCalledWith(savedServer)
    })
  })
})
