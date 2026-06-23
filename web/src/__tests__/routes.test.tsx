import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { Routes, Route } from "react-router-dom"
import { describe, it, expect } from "vitest"
import Dashboard from "../pages/Dashboard"
import ServerDetail from "../pages/ServerDetail"

function renderWithRouter(initialEntries: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/server/:id" element={<ServerDetail />} />
      </Routes>
    </MemoryRouter>
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
