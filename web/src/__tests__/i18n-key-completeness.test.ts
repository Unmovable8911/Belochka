import { describe, it, expect } from "vitest"
import en from "../i18n/en.json"
import zh from "../i18n/zh.json"
import fr from "../i18n/fr.json"
import ru from "../i18n/ru.json"
import de from "../i18n/de.json"
import es from "../i18n/es.json"
import pt from "../i18n/pt.json"
import zhTW from "../i18n/zh-TW.json"
import itLang from "../i18n/it.json"

function extractKeys(obj: Record<string, unknown>, prefix = ""): string[] {
  const keys: string[] = []
  for (const key of Object.keys(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key
    const value = obj[key]
    if (typeof value === "object" && value !== null && !Array.isArray(value)) {
      keys.push(...extractKeys(value as Record<string, unknown>, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys.sort()
}

describe("Translation key completeness", () => {
  const enKeys = extractKeys(en)

  it.each([
    ["zh", zh],
    ["fr", fr],
    ["ru", ru],
    ["de", de],
    ["es", es],
    ["pt", pt],
    ["zh-TW", zhTW],
    ["it", itLang],
  ] as const)("%s has exactly the same keys as en", (_, translations) => {
    const keys = extractKeys(translations as Record<string, unknown>)
    expect(keys).toEqual(enKeys)
  })

  it("en has at least one key", () => {
    expect(enKeys.length).toBeGreaterThan(0)
  })
})
