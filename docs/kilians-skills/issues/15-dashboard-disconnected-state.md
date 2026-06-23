## What to build

When a server's SSH connection is lost, its dashboard card should display a status message instead of stale metric data. The card retains its size and position in the grid but replaces metric content with a status icon and descriptive message (e.g., "Reconnecting (3/∞)", "Auth failed — check configuration").

Different disconnect reasons show different messages: transient failures show reconnection attempt count, authentication failures show a clear "fix configuration" message, and initial connection pending shows a loading state.

## Acceptance criteria

- [ ] Disconnected server card retains same size and grid position as connected cards
- [ ] Metric content replaced with centered status icon and message
- [ ] Reconnecting state shows attempt count (e.g., "Reconnecting (3/∞)")
- [ ] Auth failure shows clear error (e.g., "Auth failed — check configuration")
- [ ] Host key mismatch shows specific error
- [ ] Initial connecting state shows loading indicator
- [ ] Card remains clickable (navigates to detail page) even when disconnected
- [ ] Status transitions are smooth (no layout jumps)

## Blocked by

- #14 Dashboard Server Cards
