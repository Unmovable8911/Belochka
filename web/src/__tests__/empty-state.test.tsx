import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MonitorContext, initialMonitorState, type MonitorState, type MonitorAction } from "../hooks/useMonitorState"
import Dashboard from "../pages/Dashboard"
import type { Dispatch } from "react"

function renderDashboard(state: MonitorState = initialMonitorState) {
  const dispatch: Dispatch<MonitorAction> = vi.fn()
  const result = render(
    <MonitorContext value={{ state, dispatch }}>
      <Dashboard />
    </MonitorContext>
  )
  return {
    ...result,
    rerenderWithState(newState: MonitorState) {
      result.rerender(
        <MonitorContext value={{ state: newState, dispatch }}>
          <Dashboard />
        </MonitorContext>
      )
    },
  }
}

describe("Empty State Guidance", () => {
  afterEach(() => {
    cleanup()
  })

  it("displays empty state when server list is empty", () => {
    renderDashboard()

    expect(screen.getByText(/no servers configured/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /add your first server/i })).toBeInTheDocument()
  })

  it("opens add-server dialog when empty state button is clicked", async () => {
    const user = userEvent.setup()
    renderDashboard()

    const addButton = screen.getByRole("button", { name: /add your first server/i })
    await user.click(addButton)

    expect(screen.getByRole("dialog")).toBeInTheDocument()
    expect(screen.getByText("Add Server")).toBeInTheDocument()
  })

  it("hides empty state when servers are present", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [{ id: "srv-1", name: "Web Server", host: "10.0.0.1", status: "connected" }],
    }
    renderDashboard(stateWithServers)

    expect(screen.queryByText(/no servers configured/i)).toBeNull()
    expect(screen.queryByRole("button", { name: /add your first server/i })).toBeNull()
    // The normal "Add Server" button should be in the header
    expect(screen.getByRole("button", { name: /add server/i })).toBeInTheDocument()
  })

  it("reappears when last server is removed", () => {
    const stateWithServers: MonitorState = {
      ...initialMonitorState,
      servers: [{ id: "srv-1", name: "Web Server", host: "10.0.0.1", status: "connected" }],
    }
    const { rerenderWithState } = renderDashboard(stateWithServers)

    // Verify empty state is NOT shown
    expect(screen.queryByText(/no servers configured/i)).toBeNull()

    // Re-render with empty servers (simulating last server deleted)
    rerenderWithState(initialMonitorState)

    // Empty state should reappear
    expect(screen.getByText(/no servers configured/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /add your first server/i })).toBeInTheDocument()
  })
})
