import { useCallback, useEffect, useReducer } from "react"

// useTreeState owns the sidebar tree's pure state: group expansion (with the
// seed-new-groups invariant), the drag target highlight, and drop payload
// parsing. Dialog state, API orchestration (useSidebarActions) and the
// URL-derived group selection stay in the component — only primitives cross
// this seam, never DOM events.

// ROOT_DROP marks a drag hovering blank tree space, where a Server is dropped
// to move it out of its Group. The sentinel never collides with a real Group
// id (UUIDs). Exported for the component's hover-ring styling.
export const ROOT_DROP = "__root__"

// TreeState is the expansion and drag-target state of the tree. seeded
// records every Group id that has ever been rendered, so the reducer can tell
// a genuinely new Group (auto-expand it) from one the user has collapsed
// (never re-expand it).
export interface TreeState {
  expanded: Set<string>
  seeded: Set<string>
  dragOverId: string | null
}

// TreeAction mirrors the state transitions of the tree.
export type TreeAction =
  | { type: "toggle"; id: string }
  // groups_updated carries the current Group id set; the reducer seeds the
  // fresh ids into both sets. The caller dispatches it on every groupList
  // change (initial load included); the reducer no-ops when nothing is fresh.
  | { type: "groups_updated"; ids: string[] }
  | { type: "drag_over"; target: string }
  | { type: "drag_leave" }

// initialTreeState: nothing seeded yet, so the first groups_updated expands
// every Group present at load.
const initialTreeState: TreeState = {
  expanded: new Set(),
  seeded: new Set(),
  dragOverId: null,
}

// treeReducer is the pure state transition function for the sidebar tree.
// Newly appearing Groups default to expanded, but Groups the user has
// collapsed are never re-expanded: only ids absent from seeded are added.
export function treeReducer(state: TreeState, action: TreeAction): TreeState {
  switch (action.type) {
    case "toggle": {
      const next = new Set(state.expanded)
      if (next.has(action.id)) {
        next.delete(action.id)
      } else {
        next.add(action.id)
      }
      return { ...state, expanded: next }
    }

    case "groups_updated": {
      const fresh = action.ids.filter((id) => !state.seeded.has(id))
      if (fresh.length === 0) return state
      const seeded = new Set(state.seeded)
      const expanded = new Set(state.expanded)
      for (const id of fresh) {
        seeded.add(id)
        expanded.add(id)
      }
      return { ...state, seeded, expanded }
    }

    case "drag_over":
      return { ...state, dragOverId: action.target }

    case "drag_leave":
      return state.dragOverId === null ? state : { ...state, dragOverId: null }
  }
}

// DropPayload is the outcome of resolving a dataTransfer payload against the
// tree. "empty" and "not_server" payloads are ignored silently by the caller;
// a "malformed" payload is surfaced as a toast (matching the behavior before
// the module existed).
export type DropPayload =
  | { ok: true; serverId: string }
  | { ok: false; reason: "empty" | "malformed" | "not_server" }

// parseDrop resolves the application/json payload the sidebar writes on
// dragStart (ServerRow). It accepts only {type:"server", id} payloads, so
// foreign drag payloads can never move anything.
export function parseDrop(raw: string): DropPayload {
  if (!raw) return { ok: false, reason: "empty" }
  let data: unknown
  try {
    data = JSON.parse(raw)
  } catch {
    return { ok: false, reason: "malformed" }
  }
  if (typeof data !== "object" || data === null) return { ok: false, reason: "malformed" }
  const { type, id } = data as { type?: unknown; id?: unknown }
  if (type !== "server" || typeof id !== "string" || id === "") return { ok: false, reason: "not_server" }
  return { ok: true, serverId: id }
}

// useTreeState wires the reducer into the component. It consumes only the
// current Group ids — groupList itself stays with the caller. DOM drag events
// are translated to primitives by the component.
export function useTreeState(groupIds: string[]) {
  const [state, dispatch] = useReducer(treeReducer, initialTreeState)

  // Re-seed whenever the id set changes (initial load and every refetch after
  // a mutation). The ids join is the effect dep — ids are UUIDs, so "," cannot
  // collide with an id; string deps compare by value, so a recreated array
  // with identical contents does not re-dispatch, and the reducer no-ops when
  // no id is fresh anyway.
  const idsKey = groupIds.join(",")
  useEffect(() => {
    dispatch({ type: "groups_updated", ids: groupIds })
    // oxlint(react-hooks) — idsKey is the intended dep; groupIds is captured
    // from the same render that produced idsKey.
  }, [idsKey]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleToggle = useCallback((id: string) => dispatch({ type: "toggle", id }), [])
  const handleDragOver = useCallback((target: string) => dispatch({ type: "drag_over", target }), [])
  const clearDragTarget = useCallback(() => dispatch({ type: "drag_leave" }), [])

  return {
    expanded: state.expanded,
    dragOverId: state.dragOverId,
    handleToggle,
    handleDragOver,
    clearDragTarget,
  }
}
