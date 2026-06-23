import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
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

describe("Dashboard server cards", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it("renders clickable server cards linking to detail pages", () => {
    renderDashboard(stateWithServers)

    const links = screen.getAllByRole("link")
    expect(links).toHaveLength(2)
    expect(links[0]).toHaveAttribute("href", "/server/srv-1")
    expect(links[1]).toHaveAttribute("href", "/server/srv-2")
  })

  it("shows server names in the card grid", () => {
    renderDashboard(stateWithServers)

    expect(screen.getByText("Production Web")).toBeInTheDocument()
    expect(screen.getByText("Database")).toBeInTheDocument()
  })

  it("shows server status badges", () => {
    renderDashboard(stateWithServers)

    expect(screen.getByText("connected")).toBeInTheDocument()
    expect(screen.getByText("disconnected")).toBeInTheDocument()
  })
})
