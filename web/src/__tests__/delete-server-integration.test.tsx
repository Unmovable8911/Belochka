import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { MonitorContext, initialMonitorState, type MonitorState, type MonitorAction } from "../hooks/useMonitorState"
import Dashboard from "../pages/Dashboard"
import type { Dispatch } from "react"

function renderDashboard(
  state: MonitorState = initialMonitorState,
  dispatch?: Dispatch<MonitorAction>,
) {
  const dispatchFn: Dispatch<MonitorAction> = dispatch ?? vi.fn()
  return render(
    <MemoryRouter>
      <MonitorContext value={{ state, dispatch: dispatchFn }}>
        <Dashboard />
      </MonitorContext>
    </MemoryRouter>,
  )
}

const stateWithServers: MonitorState = {
  ...initialMonitorState,
  servers: [
    { id: "srv-1", name: "Production Web", host: "10.0.0.1", status: "connected" },
    { id: "srv-2", name: "Database", host: "10.0.0.2", status: "disconnected" },
  ],
}

describe("Delete Server from Dashboard", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("shows a delete button for each server in the list", () => {
    renderDashboard(stateWithServers)

    const deleteButtons = screen.getAllByRole("button", { name: /delete/i })
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2)
  })

  it("opens confirmation dialog with server name when delete button is clicked", async () => {
    const user = userEvent.setup()
    renderDashboard(stateWithServers)

    // Find the delete button associated with "Production Web"
    const deleteButtons = screen.getAllByRole("button", { name: /delete/i })
    await user.click(deleteButtons[0])

    const dialog = screen.getByRole("dialog")
    expect(dialog).toHaveTextContent("Production Web")
    expect(dialog).toHaveTextContent(/are you sure/i)
  })

  it("dispatches remove_server and removes server from list after successful delete", async () => {
    const user = userEvent.setup()
    const dispatch = vi.fn()

    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 204 })
    )

    renderDashboard(stateWithServers, dispatch)

    // Click delete on the first server
    const deleteButtons = screen.getAllByRole("button", { name: /delete/i })
    await user.click(deleteButtons[0])

    // Confirm deletion in the dialog
    const dialog = screen.getByRole("dialog")
    await user.click(within(dialog).getByRole("button", { name: /^delete$/i }))

    await vi.waitFor(() => {
      expect(dispatch).toHaveBeenCalledWith({
        type: "remove_server",
        data: { serverId: "srv-1" },
      })
    })
  })
})
