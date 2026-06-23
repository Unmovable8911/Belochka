import { describe, it, expect, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { I18nextProvider } from "react-i18next"
import { vi } from "vitest"
import i18n from "../i18n"
import { MonitorContext, initialMonitorState, type MonitorState } from "../hooks/useMonitorState"
import Dashboard from "../pages/Dashboard"

function renderDashboard(state: MonitorState = initialMonitorState, lang = "en") {
  i18n.changeLanguage(lang)
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <MonitorContext value={{ state, dispatch: vi.fn() }}>
          <Dashboard />
        </MonitorContext>
      </MemoryRouter>
    </I18nextProvider>,
  )
}

describe("Dashboard i18n", () => {
  beforeEach(() => {
    i18n.changeLanguage("en")
  })

  afterEach(() => {
    cleanup()
  })

  it("renders title in Chinese when language is zh", () => {
    renderDashboard(initialMonitorState, "zh")
    expect(screen.getByText("仪表盘")).toBeInTheDocument()
  })

  it("renders empty state text in French", () => {
    renderDashboard(initialMonitorState, "fr")
    expect(screen.getByText("Aucun serveur configuré")).toBeInTheDocument()
  })
})
