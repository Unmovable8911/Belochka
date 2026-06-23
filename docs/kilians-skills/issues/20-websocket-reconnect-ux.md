## What to build

Implement client-side WebSocket auto-reconnection with exponential backoff, and the disconnection UX: a top banner warning and visual dimming of stale data.

When the WebSocket connection to the Belochka server is lost, the client automatically attempts to reconnect with exponential backoff. During disconnection, a banner appears at the top of the page ("Connection lost, reconnecting..."), and the last received data is retained but displayed with reduced opacity to indicate staleness. On successful reconnection, the banner disappears, opacity restores, and the hub sends a fresh snapshot.

## Acceptance criteria

- [ ] WebSocket auto-reconnect with exponential backoff on unexpected close
- [ ] Top banner displayed when connection is lost: "Connection lost, reconnecting..."
- [ ] Last received metric data retained in state during disconnection
- [ ] Retained data displayed with reduced opacity (visual dimming overlay)
- [ ] Banner disappears on successful reconnection
- [ ] Data opacity restores to normal on reconnection
- [ ] Fresh snapshot received on reconnection re-initializes state
- [ ] Intentional close (page navigation) does not trigger reconnection
- [ ] Backoff resets on successful reconnection

## Blocked by

- #12 React WebSocket State Management
