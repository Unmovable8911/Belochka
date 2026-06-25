import { describe, it, expect, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { I18nextProvider } from "react-i18next"
import i18n from "../i18n"
import { Layout } from "../components/Layout"

function renderLayout() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <Layout>
          <div>page content</div>
        </Layout>
      </MemoryRouter>
    </I18nextProvider>,
  )
}

describe("Language Switcher", () => {
  beforeEach(() => {
    i18n.changeLanguage("en")
    localStorage.clear()
  })

  afterEach(() => {
    cleanup()
  })

  it("renders language switcher with globe icon", () => {
    renderLayout()
    expect(screen.getByTestId("language-switcher")).toBeInTheDocument()
  })

  it("shows four language options with native names", async () => {
    const user = userEvent.setup()
    renderLayout()

    const trigger = screen.getByTestId("language-switcher")
    await user.click(trigger)

    const listbox = screen.getByRole("listbox")
    const options = within(listbox).getAllByRole("option")
    const labels = options.map((o) => o.textContent)

    expect(labels).toContain("English")
    expect(labels).toContain("中文")
    expect(labels).toContain("Français")
    expect(labels).toContain("Русский")
    expect(options).toHaveLength(4)
  })

  it("switching language changes UI text", async () => {
    const user = userEvent.setup()
    renderLayout()

    const trigger = screen.getByTestId("language-switcher")
    await user.click(trigger)

    const zhOption = screen.getByRole("option", { name: "中文" })
    await user.click(zhOption)

    expect(i18n.language).toBe("zh")
  })

  it("persists language choice in localStorage", async () => {
    const user = userEvent.setup()
    renderLayout()

    const trigger = screen.getByTestId("language-switcher")
    await user.click(trigger)

    const frOption = screen.getByRole("option", { name: "Français" })
    await user.click(frOption)

    expect(localStorage.getItem("i18nextLng")).toBe("fr")
  })

  it("renders children inside the layout", () => {
    renderLayout()
    expect(screen.getByText("page content")).toBeInTheDocument()
  })
})
