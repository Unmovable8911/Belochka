import { describe, it, expect, beforeEach } from "vitest"

describe("i18n initialization", () => {
  it("loads all four languages", async () => {
    const i18n = (await import("../i18n")).default
    const languages = Object.keys(i18n.options.resources ?? {})
    expect(languages).toEqual(expect.arrayContaining(["en", "zh", "fr", "ru"]))
    expect(languages).toHaveLength(4)
  })

  it("falls back to en for unsupported language", async () => {
    const i18n = (await import("../i18n")).default
    i18n.changeLanguage("ja")
    expect(i18n.t("dashboard.title")).toBe("Dashboard")
  })

  it("resolves keys in non-default language", async () => {
    const i18n = (await import("../i18n")).default
    i18n.changeLanguage("zh")
    expect(i18n.t("dashboard.title")).toBe("仪表盘")
    i18n.changeLanguage("en")
  })

  it("reads language from app-lang meta tag", async () => {
    // Simulate server injecting a language into the meta tag.
    const meta = document.createElement("meta")
    meta.setAttribute("name", "app-lang")
    meta.setAttribute("content", "fr")
    document.head.appendChild(meta)

    // Re-import i18n to pick up the meta tag (module cache means we read the
    // already-initialised instance, but we can verify via changeLanguage that
    // the module read the meta tag on load by checking getAppLang directly).
    const { getAppLang } = await import("../i18n")
    expect(getAppLang()).toBe("fr")

    document.head.removeChild(meta)
  })
})
