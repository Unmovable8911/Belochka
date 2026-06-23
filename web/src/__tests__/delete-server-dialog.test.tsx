import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { DeleteServerDialog } from "../components/DeleteServerDialog"

const testServer = {
  id: "srv-1",
  name: "Production Web",
}

describe("DeleteServerDialog", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("shows confirmation dialog with server name when open", () => {
    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={() => {}}
        onDeleted={() => {}}
      />
    )

    const dialog = screen.getByRole("dialog")
    expect(dialog).toHaveTextContent("Production Web")
    expect(dialog).toHaveTextContent(/are you sure/i)
  })

  it("has cancel and delete buttons", () => {
    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={() => {}}
        onDeleted={() => {}}
      />
    )

    const dialog = screen.getByRole("dialog")
    expect(within(dialog).getByRole("button", { name: /cancel/i })).toBeInTheDocument()
    expect(within(dialog).getByRole("button", { name: /delete/i })).toBeInTheDocument()
  })

  it("calls onOpenChange(false) when cancel is clicked without calling the API", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const onDeleted = vi.fn()
    const fetchSpy = vi.spyOn(globalThis, "fetch")

    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={onOpenChange}
        onDeleted={onDeleted}
      />
    )

    await user.click(screen.getByRole("button", { name: /cancel/i }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(onDeleted).not.toHaveBeenCalled()
  })

  it("calls DELETE API and invokes onDeleted on successful deletion", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const onDeleted = vi.fn()

    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 204 })
    )

    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={onOpenChange}
        onDeleted={onDeleted}
      />
    )

    await user.click(screen.getByRole("button", { name: /^delete$/i }))

    await vi.waitFor(() => {
      expect(onDeleted).toHaveBeenCalledWith("srv-1")
    })

    expect(globalThis.fetch).toHaveBeenCalledWith("/api/servers/srv-1", {
      method: "DELETE",
    })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("shows loading state while deleting", async () => {
    const user = userEvent.setup()
    let resolveFetch!: (value: Response) => void
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise((resolve) => { resolveFetch = resolve })
    )

    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={() => {}}
        onDeleted={() => {}}
      />
    )

    await user.click(screen.getByRole("button", { name: /^delete$/i }))

    expect(screen.getByRole("button", { name: /deleting/i })).toBeDisabled()
    expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled()

    // Resolve to prevent hanging
    resolveFetch(new Response(null, { status: 204 }))
  })

  it("handles 404 gracefully as successful deletion", async () => {
    const user = userEvent.setup()
    const onDeleted = vi.fn()

    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "not_found", message: "Server not found" } }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      })
    )

    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={() => {}}
        onDeleted={onDeleted}
      />
    )

    await user.click(screen.getByRole("button", { name: /^delete$/i }))

    await vi.waitFor(() => {
      expect(onDeleted).toHaveBeenCalledWith("srv-1")
    })
  })

  it("shows error toast on API failure", async () => {
    const user = userEvent.setup()
    const onDeleted = vi.fn()

    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "store_error", message: "Failed to delete server" } }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      })
    )

    render(
      <DeleteServerDialog
        server={testServer}
        open={true}
        onOpenChange={() => {}}
        onDeleted={onDeleted}
      />
    )

    await user.click(screen.getByRole("button", { name: /^delete$/i }))

    // Should NOT call onDeleted on failure
    await vi.waitFor(() => {
      // After deleting finishes, the button should return to "Delete"
      expect(screen.getByRole("button", { name: /^delete$/i })).toBeEnabled()
    })

    expect(onDeleted).not.toHaveBeenCalled()
  })
})
