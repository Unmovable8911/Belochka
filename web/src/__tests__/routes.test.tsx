import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { Routes, Route } from "react-router-dom"
import { describe, it, expect, vi } from "vitest"
import Dashboard from "../pages/Dashboard"
import ServerDetail from "../pages/ServerDetail"
import { MonitorContext, initialMonitorState } from "../hooks/useMonitorState"

function renderWithRouter(initialEntries: string[]) {
  return render(
    <MonitorContext value={{ state: initialMonitorState, dispatch: vi.fn() }}>
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/server/:id" element={<ServerDetail />} />
        </Routes>
      </MemoryRouter>
    </MonitorContext>
  )
}

describe("routing", () => {
  it("renders dashboard at /", () => {
    renderWithRouter(["/"])
    expect(screen.getByText("Dashboard")).toBeInTheDocument()
  })

  it("renders server detail at /server/:id", () => {
    renderWithRouter(["/server/abc-123"])
    expect(screen.getByText("Server Detail")).toBeInTheDocument()
    expect(screen.getByText(/abc-123/)).toBeInTheDocument()
  })
})
