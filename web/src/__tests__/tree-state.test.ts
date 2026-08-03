import { describe, it, expect } from "vitest"
import { treeReducer, parseDrop, ROOT_DROP, type TreeState } from "../hooks/useTreeState"

// --- treeReducer ---

function initialState(): TreeState {
  return { expanded: new Set(), seeded: new Set(), dragOverId: null }
}

describe("treeReducer", () => {
  it("toggles a group id in and out of the expanded set", () => {
    let state = initialState()
    state = treeReducer(state, { type: "toggle", id: "g1" })
    expect(state.expanded.has("g1")).toBe(true)

    state = treeReducer(state, { type: "toggle", id: "g1" })
    expect(state.expanded.has("g1")).toBe(false)
    // Toggling one group never touches the others.
    state = treeReducer(state, { type: "toggle", id: "g2" })
    expect(state.expanded.has("g2")).toBe(true)
  })

  it("seeds and expands every group on the first groups_updated", () => {
    const state = treeReducer(initialState(), { type: "groups_updated", ids: ["g1", "g2"] })
    expect(state.seeded).toEqual(new Set(["g1", "g2"]))
    expect(state.expanded).toEqual(new Set(["g1", "g2"]))
  })

  it("is a no-op (same state) when groups_updated carries no fresh ids", () => {
    const seeded = treeReducer(initialState(), { type: "groups_updated", ids: ["g1"] })
    const again = treeReducer(seeded, { type: "groups_updated", ids: ["g1"] })
    expect(again).toBe(seeded)
  })

  it("never re-expands a group the user collapsed, even after a refetch", () => {
    let state = treeReducer(initialState(), { type: "groups_updated", ids: ["g1", "g2"] })
    state = treeReducer(state, { type: "toggle", id: "g1" })
    expect(state.expanded.has("g1")).toBe(false)

    // A refetch carries the same ids — g1 must stay collapsed.
    const refetched = treeReducer(state, { type: "groups_updated", ids: ["g1", "g2"] })
    expect(refetched.expanded.has("g1")).toBe(false)
    expect(refetched.expanded.has("g2")).toBe(true)
  })

  it("auto-expands only the fresh group when new ids appear", () => {
    let state = treeReducer(initialState(), { type: "groups_updated", ids: ["g1"] })
    state = treeReducer(state, { type: "toggle", id: "g1" }) // user collapses g1

    // A new group is created and appears in the next fetch.
    state = treeReducer(state, { type: "groups_updated", ids: ["g1", "g2"] })
    expect(state.expanded.has("g1")).toBe(false) // user's choice respected
    expect(state.expanded.has("g2")).toBe(true) // fresh group auto-expands
  })

  it("tracks the drag target (group id or ROOT_DROP) and clears it", () => {
    let state = treeReducer(initialState(), { type: "drag_over", target: "g1" })
    expect(state.dragOverId).toBe("g1")

    state = treeReducer(state, { type: "drag_over", target: ROOT_DROP })
    expect(state.dragOverId).toBe(ROOT_DROP)

    state = treeReducer(state, { type: "drag_leave" })
    expect(state.dragOverId).toBe(null)
  })

  it("drag_leave on an already-clear target returns the same state", () => {
    const state = initialState()
    expect(treeReducer(state, { type: "drag_leave" })).toBe(state)
  })
})

// --- parseDrop ---

describe("parseDrop", () => {
  it("resolves a valid server drag payload", () => {
    const parsed = parseDrop(JSON.stringify({ type: "server", id: "srv-1" }))
    expect(parsed).toEqual({ ok: true, serverId: "srv-1" })
  })

  it("rejects an empty payload", () => {
    expect(parseDrop("")).toEqual({ ok: false, reason: "empty" })
  })

  it("rejects malformed JSON", () => {
    expect(parseDrop("not-json")).toEqual({ ok: false, reason: "malformed" })
    expect(parseDrop("42")).toEqual({ ok: false, reason: "malformed" })
    expect(parseDrop("null")).toEqual({ ok: false, reason: "malformed" })
  })

  it("rejects payloads that are not server drags", () => {
    expect(parseDrop(JSON.stringify({ type: "group", id: "g1" }))).toEqual({ ok: false, reason: "not_server" })
    expect(parseDrop(JSON.stringify({ type: "server" }))).toEqual({ ok: false, reason: "not_server" })
    expect(parseDrop(JSON.stringify({ type: "server", id: "" }))).toEqual({ ok: false, reason: "not_server" })
  })
})
