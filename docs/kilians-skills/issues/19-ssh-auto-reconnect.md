## What to build

Implement automatic SSH reconnection with exponential backoff for the connection pool. When an SSH connection drops (detected by the collector's consecutive failure counter reaching 3, or by keepalive failure), the system automatically attempts to reconnect.

Backoff schedule: 1s → 2s → 4s → 8s → 16s → 30s (capped). Reconnection continues indefinitely for retryable errors. Non-retryable errors (authentication failure, host key mismatch) stop reconnection attempts immediately and report the error state.

SSH keepalive pings are sent every 30 seconds. Three consecutive keepalive failures mark the connection as dead and trigger reconnection.

## Acceptance criteria

- [ ] Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s cap
- [ ] Reconnection triggered by 3 consecutive collection failures
- [ ] SSH keepalive every 30 seconds
- [ ] 3 consecutive keepalive failures trigger reconnection
- [ ] Backoff resets to 1s on successful reconnection
- [ ] Non-retryable errors (auth failure, host key mismatch) stop reconnection
- [ ] Non-retryable error state reported via status message (for hub broadcast)
- [ ] Retryable errors (network, timeout) continue reconnection indefinitely
- [ ] Reconnection attempt count tracked and available for status display
- [ ] Context-based cancellation stops reconnection on server deletion or shutdown
- [ ] Tests for backoff timing, error classification, state transitions

## Blocked by

- #10 Metrics Collector
