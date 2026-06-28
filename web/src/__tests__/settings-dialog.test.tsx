import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, cleanup, within, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { I18nextProvider } from "react-i18next"
import i18n from "../i18n"
import { SettingsDialog } from "../components/SettingsDialog"

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { toast } from "sonner"

const mockConfig = {
  port: 53136,
  data_dir: "./data",
  language: "en",
  log_path: "",
  log_retention_days: 3,
}

function makeFetchOk(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

function renderDialog() {
  return render(
    <I18nextProvider i18n={i18n}>
      <SettingsDialog />
    </I18nextProvider>,
  )
}

beforeEach(() => {
  i18n.changeLanguage("en")
  vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
    const urlStr = typeof url === "string" ? url : url.toString()
    if (urlStr === "/api/config" ) {
      return makeFetchOk(mockConfig)
    }
    return new Response("Not Found", { status: 404 })
  })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe("SettingsDialog", () => {
  it("renders a gear icon button", () => {
    renderDialog()
    expect(screen.getByRole("button", { name: /settings/i })).toBeInTheDocument()
  })

  it("opens the dialog when the gear button is clicked", async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.click(screen.getByRole("button", { name: /settings/i }))

    expect(await screen.findByRole("dialog")).toBeInTheDocument()
  })

  it("fetches config and pre-populates all five fields", async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    // Language field (combobox)
    expect(within(dialog).getByRole("combobox", { name: /language/i })).toBeInTheDocument()

    // Log path
    const logPathInput = within(dialog).getByLabelText(/log path/i)
    expect(logPathInput).toBeInTheDocument()
    expect(logPathInput).toHaveValue("")

    // Log retention
    const retentionInput = within(dialog).getByLabelText(/log retention/i)
    expect(retentionInput).toBeInTheDocument()
    expect(retentionInput).toHaveValue(3)

    // Port
    const portInput = within(dialog).getByLabelText(/^port$/i)
    expect(portInput).toBeInTheDocument()
    expect(portInput).toHaveValue(53136)

    // Data directory
    const dataDirInput = within(dialog).getByLabelText(/data dir/i)
    expect(dataDirInput).toBeInTheDocument()
    expect(dataDirInput).toHaveValue("./data")
  })

  it("calls PATCH /api/config with only changed fields on save", async () => {
    const user = userEvent.setup()
    const patchSpy = vi.fn().mockResolvedValue(makeFetchOk({ ...mockConfig, log_retention_days: 7 }))

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr === "/api/config" && (!options?.method || options.method === "GET")) {
        return makeFetchOk(mockConfig)
      }
      if (urlStr === "/api/config" && options?.method === "PATCH") {
        return patchSpy(url, options)
      }
      return new Response("Not Found", { status: 404 })
    })

    renderDialog()
    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    // Change only log retention
    const retentionInput = within(dialog).getByLabelText(/log retention/i)
    await user.clear(retentionInput)
    await user.type(retentionInput, "7")

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await waitFor(() => {
      expect(patchSpy).toHaveBeenCalledTimes(1)
      const body = JSON.parse(patchSpy.mock.calls[0][1].body)
      expect(body).toEqual({ log_retention_days: 7 })
    })
  })

  it("shows a success toast after saving", async () => {
    const user = userEvent.setup()

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr === "/api/config" && options?.method === "PATCH") {
        return makeFetchOk({ ...mockConfig })
      }
      return makeFetchOk(mockConfig)
    })

    renderDialog()
    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    // Change a field so PATCH is meaningful
    const retentionInput = within(dialog).getByLabelText(/log retention/i)
    await user.clear(retentionInput)
    await user.type(retentionInput, "7")

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalled()
    })
  })

  it("shows an error toast when PATCH fails", async () => {
    const user = userEvent.setup()

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr === "/api/config" && options?.method === "PATCH") {
        return new Response(
          JSON.stringify({ error: { code: "persist_error", message: "disk full" } }),
          { status: 500, headers: { "Content-Type": "application/json" } },
        )
      }
      return makeFetchOk(mockConfig)
    })

    renderDialog()
    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    const retentionInput = within(dialog).getByLabelText(/log retention/i)
    await user.clear(retentionInput)
    await user.type(retentionInput, "7")

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  it("shows restart_required notice when port or data_dir is changed", async () => {
    const user = userEvent.setup()

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr === "/api/config" && options?.method === "PATCH") {
        return makeFetchOk({ ...mockConfig, port: 9000, restart_required: true })
      }
      return makeFetchOk(mockConfig)
    })

    renderDialog()
    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    const portInput = within(dialog).getByLabelText(/^port$/i)
    await user.clear(portInput)
    await user.type(portInput, "9000")

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await waitFor(() => {
      expect(within(dialog).getByTestId("restart-notice")).toBeInTheDocument()
    })
  })

  it("calls i18n.changeLanguage when language field is changed and save succeeds", async () => {
    const user = userEvent.setup()
    const changeLangSpy = vi.spyOn(i18n, "changeLanguage")

    vi.spyOn(globalThis, "fetch").mockImplementation(async (url, options) => {
      const urlStr = typeof url === "string" ? url : url.toString()
      if (urlStr === "/api/config" && options?.method === "PATCH") {
        return makeFetchOk({ ...mockConfig, language: "zh" })
      }
      return makeFetchOk(mockConfig)
    })

    renderDialog()
    await user.click(screen.getByRole("button", { name: /settings/i }))

    const dialog = await screen.findByRole("dialog")

    // Click the language combobox to open dropdown
    const langCombobox = within(dialog).getByRole("combobox", { name: /language/i })
    await user.click(langCombobox)

    // Select Chinese
    const zhOption = await screen.findByRole("option", { name: "中文" })
    await user.click(zhOption)

    await user.click(within(dialog).getByRole("button", { name: /^save$/i }))

    await waitFor(() => {
      expect(changeLangSpy).toHaveBeenCalledWith("zh")
    })
  })
})
