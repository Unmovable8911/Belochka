import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { I18nextProvider } from "react-i18next"
import i18n from "../i18n"
import SetupPage from "../pages/SetupPage"

vi.mock("@/api/client", () => ({
  setup: vi.fn(),
}))

import * as api from "@/api/client"

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <SetupPage />
    </I18nextProvider>,
  )
}

beforeEach(() => {
  i18n.changeLanguage("en")
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe("SetupPage", () => {
  describe("language step", () => {
    it("renders four language buttons", () => {
      renderPage()
      expect(screen.getByText("English")).toBeInTheDocument()
      expect(screen.getByText("中文")).toBeInTheDocument()
      expect(screen.getByText("Français")).toBeInTheDocument()
      expect(screen.getByText("Русский")).toBeInTheDocument()
    })

    it("pre-selects the browser-detected language", () => {
      renderPage()
      // The pre-selected button should have border-primary class (includes "border-primary")
      const buttons = screen.getAllByRole("button")
      const langButtons = buttons.filter((b) =>
        ["English", "中文", "Français", "Русский"].some((l) => b.textContent === l),
      )
      const selected = langButtons.find((b) => b.className.includes("border-primary"))
      expect(selected).toBeDefined()
    })

    it("highlights clicked language and calls i18n.changeLanguage", async () => {
      const user = userEvent.setup()
      const changeLangSpy = vi.spyOn(i18n, "changeLanguage")

      renderPage()

      const zhButton = screen.getByText("中文")
      await user.click(zhButton)

      expect(changeLangSpy).toHaveBeenCalledWith("zh")
      expect(zhButton.className).toContain("border-primary")
    })

    it('shows "Continue" button that advances to password step', async () => {
      const user = userEvent.setup()
      renderPage()

      await user.click(screen.getByText("Continue"))

      // Wait for CSS fade transition (150ms) to complete
      await waitFor(() => {
        expect(screen.getByLabelText("Password")).toBeInTheDocument()
      })
      expect(screen.getByLabelText("Confirm Password")).toBeInTheDocument()
    })
  })

  describe("password step", () => {
    async function goToPasswordStep() {
      const user = userEvent.setup()
      renderPage()
      await user.click(screen.getByText("Continue"))
      await waitFor(() => {
        expect(screen.getByLabelText("Password")).toBeInTheDocument()
      })
      return user
    }

    it("shows password and confirm fields", async () => {
      await goToPasswordStep()
      expect(screen.getByLabelText("Password")).toBeInTheDocument()
      expect(screen.getByLabelText("Confirm Password")).toBeInTheDocument()
    })

    it("shows strength bar when password is typed", async () => {
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      await user.type(passwordInput, "short")

      // Strength label should appear
      expect(screen.getByText("Very Weak")).toBeInTheDocument()
    })

    it("shows match indicator when confirm is typed", async () => {
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      const confirmInput = screen.getByLabelText("Confirm Password")

      await user.type(passwordInput, "secret123")
      await user.type(confirmInput, "secret123")

      expect(screen.getByText("Passwords match")).toBeInTheDocument()
    })

    it("shows mismatch indicator when confirm differs", async () => {
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      const confirmInput = screen.getByLabelText("Confirm Password")

      await user.type(passwordInput, "secret123")
      await user.type(confirmInput, "different")

      expect(screen.getByText("Passwords do not match")).toBeInTheDocument()
    })

    it("disables submit button when passwords mismatch", async () => {
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      const confirmInput = screen.getByLabelText("Confirm Password")

      await user.type(passwordInput, "secret123")
      await user.type(confirmInput, "different")

      const submitButton = screen.getByRole("button", { name: "Set Password" })
      expect(submitButton).toBeDisabled()
    })

    it("disables submit button when fields are empty", async () => {
      await goToPasswordStep()
      const submitButton = screen.getByRole("button", { name: "Set Password" })
      expect(submitButton).toBeDisabled()
    })

    it('returns to language step on "Back" click', async () => {
      const user = await goToPasswordStep()

      await user.click(screen.getByText("Back"))

      // Wait for CSS fade transition
      await waitFor(() => {
        expect(screen.getByText("English")).toBeInTheDocument()
      })
    })

    it("calls api.setup with language, password, and confirm on submit", async () => {
      const setupMock = vi.mocked(api.setup).mockResolvedValue(undefined)
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      const confirmInput = screen.getByLabelText("Confirm Password")

      await user.type(passwordInput, "secret123")
      await user.type(confirmInput, "secret123")
      await user.click(screen.getByRole("button", { name: "Set Password" }))

      await waitFor(() => {
        expect(setupMock).toHaveBeenCalledWith("en", "secret123", "secret123")
      })
    })

    it("shows error message on API failure", async () => {
      vi.mocked(api.setup).mockRejectedValue(new Error("Something went wrong"))
      const user = await goToPasswordStep()

      const passwordInput = screen.getByLabelText("Password")
      const confirmInput = screen.getByLabelText("Confirm Password")

      await user.type(passwordInput, "secret123")
      await user.type(confirmInput, "secret123")
      await user.click(screen.getByRole("button", { name: "Set Password" }))

      await waitFor(() => {
        expect(screen.getByRole("alert")).toHaveTextContent("Something went wrong")
      })
    })
  })
})
