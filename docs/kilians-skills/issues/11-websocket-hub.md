## What to build

Implement the server-side WebSocket hub that manages client connections and broadcasts metrics data. The hub is a singleton that accepts WebSocket upgrade requests at `/api/ws`, registers clients, and pushes data to all connected clients.

Three message types: `snapshot` (full state sent on new connection), `metrics` (periodic update every 2 seconds), and `status` (server connection state changes). On new client connection, the hub immediately sends a snapshot containing the current server list and latest metrics for each server.

The hub enforces a maximum of 10 concurrent WebSocket connections. The 11th connection attempt receives a WebSocket close frame with code 1013 (Try Again Later).

## Acceptance criteria

- [ ] WebSocket endpoint at `/api/ws` using gorilla/websocket
- [ ] Hub singleton managing client registration and deregistration
- [ ] `snapshot` message sent immediately on new client connection with full server state
- [ ] `metrics` message type for periodic 2-second broadcasts
- [ ] `status` message type for server connection state changes
- [ ] Full broadcast: all server data pushed to all clients each cycle
- [ ] Maximum 10 concurrent connections enforced
- [ ] 11th connection receives close frame with code 1013
- [ ] Clean client removal on disconnect (no goroutine leaks)
- [ ] Tests for broadcast delivery, connection limit, snapshot on connect

## Blocked by

- #1 Project Scaffold
