## What to build

Implement graceful shutdown triggered by SIGTERM or SIGINT. The shutdown sequence follows a specific order to ensure clean resource release: stop accepting new HTTP connections, close WebSocket connections (send close frames), stop all collector goroutines, close all SSH connections, close SQLite database.

A `context.Context` is propagated through all components and cancelled on signal receipt. If the graceful shutdown takes longer than 10 seconds, the process force-exits.

## Acceptance criteria

- [ ] SIGTERM and SIGINT caught and trigger shutdown
- [ ] `context.Context` cancellation propagated to all components
- [ ] Shutdown order: HTTP → WebSocket (close frames) → collectors → SSH → SQLite
- [ ] WebSocket clients receive proper close frame before disconnect
- [ ] All collector goroutines stop cleanly on context cancellation
- [ ] All SSH connections closed
- [ ] SQLite database closed (WAL checkpoint)
- [ ] 10-second hard timeout: force exit if graceful shutdown stalls
- [ ] slog messages for each shutdown phase
- [ ] No goroutine leaks after shutdown

## Blocked by

- #10 Metrics Collector
- #11 WebSocket Hub Server-Side
