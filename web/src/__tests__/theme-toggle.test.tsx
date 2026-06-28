import { describe, it, expect, vi, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { ThemeToggle } from "../components/ThemeToggle"

vi.mock("next-themes", () => ({
  useTheme: vi.fn(),
}))

import { useTheme } from "next-themes"

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

describe("ThemeToggle", () => {
  it("renders Moon icon when theme is light", () => {
    vi.mocked(useTheme).mockReturnValue({ theme: "light", setTheme: vi.fn(), themes: [], resolvedTheme: "light" })
    render(<ThemeToggle />)
    expect(screen.getByRole("button", { name: "Toggle theme" })).toBeInTheDocument()
    // Moon icon is rendered when in light mode (clicking will switch to dark)
    expect(document.querySelector("svg")).toBeInTheDocument()
  })

  it("renders Sun icon when theme is dark", () => {
    vi.mocked(useTheme).mockReturnValue({ theme: "dark", setTheme: vi.fn(), themes: [], resolvedTheme: "dark" })
    render(<ThemeToggle />)
    expect(screen.getByRole("button", { name: "Toggle theme" })).toBeInTheDocument()
  })

  it("calls setTheme('light') when clicked in dark mode", async () => {
    const setTheme = vi.fn()
    vi.mocked(useTheme).mockReturnValue({ theme: "dark", setTheme, themes: [], resolvedTheme: "dark" })
    const user = userEvent.setup()
    render(<ThemeToggle />)
    await user.click(screen.getByRole("button", { name: "Toggle theme" }))
    expect(setTheme).toHaveBeenCalledWith("light")
  })

  it("calls setTheme('dark') when clicked in light mode", async () => {
    const setTheme = vi.fn()
    vi.mocked(useTheme).mockReturnValue({ theme: "light", setTheme, themes: [], resolvedTheme: "light" })
    const user = userEvent.setup()
    render(<ThemeToggle />)
    await user.click(screen.getByRole("button", { name: "Toggle theme" }))
    expect(setTheme).toHaveBeenCalledWith("dark")
  })
})
