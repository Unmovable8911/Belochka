import { describe, it, expect } from "vitest"
import i18n from "../i18n"

describe("i18n initialization", () => {
  it("loads all four languages", () => {
    const languages = Object.keys(i18n.options.resources ?? {})
    expect(languages).toEqual(expect.arrayContaining(["en", "zh", "fr", "ru"]))
    expect(languages).toHaveLength(4)
  })

  it("falls back to en for unsupported language", () => {
    i18n.changeLanguage("ja")
    expect(i18n.t("dashboard.title")).toBe("Dashboard")
  })

  it("resolves keys in non-default language", () => {
    i18n.changeLanguage("zh")
    expect(i18n.t("dashboard.title")).toBe("仪表盘")
    i18n.changeLanguage("en")
  })
})
