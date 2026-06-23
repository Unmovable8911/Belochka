import { describe, it, expect } from "vitest"
import { readFileSync } from "fs"
import { resolve } from "path"

describe("dark theme configuration", () => {
  it("index.html has dark class on html element", () => {
    const html = readFileSync(resolve(__dirname, "../../index.html"), "utf-8")
    expect(html).toContain('class="dark"')
  })
})
