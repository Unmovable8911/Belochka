import { describe, it, expect } from "vitest"
import { cronReducer, initialCronState } from "./useCrons"
import type { CronState, CronAction } from "./useCrons"
import type { CronEntry } from "../types/server"

function makeEntry(overrides?: Partial<CronEntry>): CronEntry {
  return {
    minute: "*",
    hour: "*",
    dayOfMonth: "*",
    month: "*",
    dayOfWeek: "*",
    command: "/usr/bin/true",
    enabled: true,
    raw: "* * * * * /usr/bin/true",
    ...overrides,
  }
}

function makeCronResult(entries: CronEntry[]) {
  return { entries, passthroughs: [] }
}

// Helper: apply an action to the initial state
function apply(action: CronAction): CronState {
  return cronReducer(initialCronState, action)
}

// Helper: apply an action to a state with preloaded crons
function applyWithCrons(action: CronAction): CronState {
  const state: CronState = {
    ...initialCronState,
    cronsResult: makeCronResult([makeEntry(), makeEntry({ command: "/bin/ls", raw: "* * * * * /bin/ls" })]),
    cronsFetched: true,
  }
  return cronReducer(state, action)
}

// --- SET_ADD_CRON_OPEN ---
describe("SET_ADD_CRON_OPEN", () => {
  it("sets addCronOpen to true", () => {
    const state = apply({ type: "SET_ADD_CRON_OPEN", open: true })
    expect(state.addCronOpen).toBe(true)
  })

  it("sets addCronOpen to false", () => {
    const state = apply({ type: "SET_ADD_CRON_OPEN", open: false })
    expect(state.addCronOpen).toBe(false)
  })
})

// --- SET_EDIT_ENTRY ---
describe("SET_EDIT_ENTRY", () => {
  it("sets editEntry", () => {
    const entry = makeEntry()
    const state = apply({ type: "SET_EDIT_ENTRY", entry })
    expect(state.editEntry).toBe(entry)
  })

  it("clears editEntry with undefined", () => {
    const state = apply({ type: "SET_EDIT_ENTRY", entry: undefined })
    expect(state.editEntry).toBeUndefined()
  })
})

// --- SET_EDIT_INDEX ---
describe("SET_EDIT_INDEX", () => {
  it("sets editIndex", () => {
    const state = apply({ type: "SET_EDIT_INDEX", index: 3 })
    expect(state.editIndex).toBe(3)
  })

  it("clears editIndex with undefined", () => {
    const state = apply({ type: "SET_EDIT_INDEX", index: undefined })
    expect(state.editIndex).toBeUndefined()
  })
})

// --- SET_DELETE_CONFIRM_INDEX ---
describe("SET_DELETE_CONFIRM_INDEX", () => {
  it("sets deleteConfirmIndex", () => {
    const state = apply({ type: "SET_DELETE_CONFIRM_INDEX", index: 2 })
    expect(state.deleteConfirmIndex).toBe(2)
  })

  it("clears deleteConfirmIndex with null", () => {
    const state = apply({ type: "SET_DELETE_CONFIRM_INDEX", index: null })
    expect(state.deleteConfirmIndex).toBeNull()
  })
})

// --- SET_RUN_RESULT ---
describe("SET_RUN_RESULT", () => {
  it("sets runResult", () => {
    const result = { command: "ls", result: { exitCode: 0, output: "ok" } }
    const state = apply({ type: "SET_RUN_RESULT", result })
    expect(state.runResult).toBe(result)
  })

  it("clears runResult with null", () => {
    const state = apply({ type: "SET_RUN_RESULT", result: null })
    expect(state.runResult).toBeNull()
  })
})

// --- SET_RUNNING_INDEX ---
describe("SET_RUNNING_INDEX", () => {
  it("sets runningIndex", () => {
    const state = apply({ type: "SET_RUNNING_INDEX", index: 1 })
    expect(state.runningIndex).toBe(1)
  })

  it("clears runningIndex with null", () => {
    const state = apply({ type: "SET_RUNNING_INDEX", index: null })
    expect(state.runningIndex).toBeNull()
  })
})

// --- SET_ROW_ERROR / CLEAR_ROW_ERROR ---
describe("row error actions", () => {
  it("SET_ROW_ERROR adds an error at the given index", () => {
    const state = apply({ type: "SET_ROW_ERROR", index: 0, msg: "bang" })
    expect(state.rowErrors).toEqual({ 0: "bang" })
  })

  it("SET_ROW_ERROR does not remove other errors", () => {
    const s1 = cronReducer(initialCronState, { type: "SET_ROW_ERROR", index: 0, msg: "first" })
    const s2 = cronReducer(s1, { type: "SET_ROW_ERROR", index: 3, msg: "second" })
    expect(s2.rowErrors).toEqual({ 0: "first", 3: "second" })
  })

  it("CLEAR_ROW_ERROR removes the error at the given index", () => {
    const s1 = cronReducer(initialCronState, { type: "SET_ROW_ERROR", index: 0, msg: "first" })
    const s2 = cronReducer(s1, { type: "SET_ROW_ERROR", index: 3, msg: "second" })
    const s3 = cronReducer(s2, { type: "CLEAR_ROW_ERROR", index: 0 })
    expect(s3.rowErrors).toEqual({ 3: "second" })
  })
})

// --- FETCH actions ---
describe("FETCH actions", () => {
  it("FETCH_START sets loading and clears error", () => {
    const s1 = cronReducer(initialCronState, { type: "FETCH_ERROR", error: "previous" })
    const s2 = cronReducer(s1, { type: "FETCH_START" })
    expect(s2.cronsLoading).toBe(true)
    expect(s2.cronsError).toBeNull()
  })

  it("FETCH_SUCCESS stores data and clears loading", () => {
    const result = makeCronResult([makeEntry()])
    const state = apply({ type: "FETCH_SUCCESS", data: result })
    expect(state.cronsResult).toBe(result)
    expect(state.cronsLoading).toBe(false)
  })

  it("FETCH_ERROR stores error and clears loading", () => {
    const state = apply({ type: "FETCH_ERROR", error: "timeout" })
    expect(state.cronsError).toBe("timeout")
    expect(state.cronsLoading).toBe(false)
  })

  it("SET_CRONS_FETCHED sets cronsFetched to true", () => {
    const state = apply({ type: "SET_CRONS_FETCHED" })
    expect(state.cronsFetched).toBe(true)
  })
})

// --- CRON_CREATED ---
describe("CRON_CREATED", () => {
  it("appends entry when no editIndex", () => {
    const state = applyWithCrons({
      type: "CRON_CREATED",
      entry: makeEntry({ command: "new", raw: "* * * * * new" }),
    })
    expect(state.cronsResult!.entries).toHaveLength(3)
    expect(state.cronsResult!.entries[2].command).toBe("new")
    expect(state.editEntry).toBeUndefined()
    expect(state.editIndex).toBeUndefined()
  })

  it("replaces entry at editIndex when editing", () => {
    const s1: CronState = {
      ...initialCronState,
      editIndex: 0,
      cronsResult: makeCronResult([makeEntry(), makeEntry({ command: "unchanged" })]),
    }
    const state = cronReducer(s1, {
      type: "CRON_CREATED",
      entry: makeEntry({ command: "replaced", raw: "replaced" }),
    })
    expect(state.cronsResult!.entries).toHaveLength(2)
    expect(state.cronsResult!.entries[0].command).toBe("replaced")
    expect(state.cronsResult!.entries[1].command).toBe("unchanged")
    expect(state.editEntry).toBeUndefined()
    expect(state.editIndex).toBeUndefined()
  })

  it("creates a new result when cronsResult is null", () => {
    const state = apply({ type: "CRON_CREATED", entry: makeEntry() })
    expect(state.cronsResult!.entries).toHaveLength(1)
    expect(state.cronsResult!.passthroughs).toEqual([])
  })
})

// --- TOGGLE_OPTIMISTIC ---
describe("TOGGLE_OPTIMISTIC", () => {
  it("flips enabled from true to false", () => {
    const state = applyWithCrons({ type: "TOGGLE_OPTIMISTIC", index: 0 })
    expect(state.cronsResult!.entries[0].enabled).toBe(false)
  })

  it("flips enabled from false to true", () => {
    const s1: CronState = {
      ...initialCronState,
      cronsResult: makeCronResult([makeEntry({ enabled: false })]),
    }
    const state = cronReducer(s1, { type: "TOGGLE_OPTIMISTIC", index: 0 })
    expect(state.cronsResult!.entries[0].enabled).toBe(true)
  })

  it("no-ops when cronsResult is null", () => {
    const state = apply({ type: "TOGGLE_OPTIMISTIC", index: 0 })
    expect(state.cronsResult).toBeNull()
  })

  it("does not modify other entries", () => {
    const state = applyWithCrons({ type: "TOGGLE_OPTIMISTIC", index: 0 })
    expect(state.cronsResult!.entries[1].enabled).toBe(true)
  })
})

// --- TOGGLE_CONFIRM ---
describe("TOGGLE_CONFIRM", () => {
  it("replaces entry at index with server response", () => {
    const updated = makeEntry({ command: "confirmed", raw: "confirmed-raw" })
    const state = applyWithCrons({ type: "TOGGLE_CONFIRM", index: 0, updated })
    expect(state.cronsResult!.entries[0]).toBe(updated)
  })

  it("no-ops when cronsResult is null", () => {
    const state = apply({ type: "TOGGLE_CONFIRM", index: 0, updated: makeEntry() })
    expect(state.cronsResult).toBeNull()
  })
})

// --- TOGGLE_REVERT ---
describe("TOGGLE_REVERT", () => {
  it("restores original entry at index", () => {
    const original = makeEntry({ command: "original", raw: "original-raw", enabled: false })
    const s1: CronState = {
      ...initialCronState,
      cronsResult: makeCronResult([original, makeEntry()]),
    }
    // Simulate optimistic toggle then revert
    const s2 = cronReducer(s1, { type: "TOGGLE_OPTIMISTIC", index: 0 })
    expect(s2.cronsResult!.entries[0].enabled).toBe(true) // flipped
    const s3 = cronReducer(s2, { type: "TOGGLE_REVERT", index: 0, original })
    expect(s3.cronsResult!.entries[0]).toBe(original) // restored
  })

  it("no-ops when cronsResult is null", () => {
    const state = apply({ type: "TOGGLE_REVERT", index: 0, original: makeEntry() })
    expect(state.cronsResult).toBeNull()
  })
})

// --- DELETE_CRON ---
describe("DELETE_CRON", () => {
  it("removes the entry at index", () => {
    const state = applyWithCrons({ type: "DELETE_CRON", index: 0 })
    expect(state.cronsResult!.entries).toHaveLength(1)
    expect(state.cronsResult!.entries[0].command).toBe("/bin/ls")
  })

  it("re-indexes rowErrors after deletion", () => {
    const s1: CronState = {
      ...initialCronState,
      cronsResult: makeCronResult([makeEntry(), makeEntry(), makeEntry()]),
      rowErrors: { 0: "err0", 1: "err1", 3: "err3" },
    }
    const state = cronReducer(s1, { type: "DELETE_CRON", index: 1 })
    // After deleting index 1:
    // index 0 stays 0, index 3 becomes 2
    expect(state.rowErrors).toEqual({ 0: "err0", 2: "err3" })
  })

  it("no-ops when cronsResult is null", () => {
    const state = apply({ type: "DELETE_CRON", index: 0 })
    expect(state.cronsResult).toBeNull()
  })
})

// --- Unknown action ---
describe("unknown action", () => {
  it("returns the same state", () => {
    const state = apply({ type: "UNKNOWN" } as unknown as CronAction)
    expect(state).toBe(initialCronState)
  })
})
