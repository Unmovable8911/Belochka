import { describe, it, expect } from "vitest"
import { getReconnectDelay } from "../lib/reconnect"

describe("getReconnectDelay", () => {
  it("returns initial delay on first attempt", () => {
    const delay = getReconnectDelay(0)
    expect(delay).toBe(1000)
  })

  it("doubles delay with each attempt", () => {
    expect(getReconnectDelay(1)).toBe(2000)
    expect(getReconnectDelay(2)).toBe(4000)
    expect(getReconnectDelay(3)).toBe(8000)
  })

  it("caps delay at 30 seconds", () => {
    expect(getReconnectDelay(10)).toBe(30000)
    expect(getReconnectDelay(100)).toBe(30000)
  })
})
