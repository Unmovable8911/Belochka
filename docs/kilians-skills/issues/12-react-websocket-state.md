## What to build

Implement the React-side WebSocket integration: a top-level provider component that establishes a single WebSocket connection shared across all pages, and a state management layer using useReducer + Context.

The provider connects to `/api/ws`, handles the three message types (snapshot, metrics, status), and dispatches them into a reducer that maintains the canonical server state. Components anywhere in the tree access this state via context. The WebSocket connection is application-level — established once in the provider, not per-page.

For now, basic reconnection on close (full resilience with exponential backoff is slice #20).

## Acceptance criteria

- [ ] `WebSocketProvider` component wrapping the app at the top level
- [ ] Single WebSocket connection to `/api/ws` established on mount
- [ ] `useReducer` managing server state: server list, latest metrics per server, connection statuses
- [ ] `snapshot` message initializes full state
- [ ] `metrics` message updates metrics for each server
- [ ] `status` message updates server connection state
- [ ] Context hook (e.g., `useMonitorState`) for consuming state in components
- [ ] Basic reconnection on unexpected close (simple retry, no backoff yet)
- [ ] Connection state exposed (connected/disconnected) for UI consumption
- [ ] Provider does not re-render children unnecessarily (stable context reference)

## Blocked by

- #11 WebSocket Hub Server-Side
