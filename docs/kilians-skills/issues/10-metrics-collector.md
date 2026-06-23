## What to build

Implement the metrics collector that runs one goroutine per monitored server, executes a combined SSH command every 2 seconds, parses the output using the metrics parsers, computes rate-based metrics via cross-cycle deltas, and makes the latest metrics available for broadcasting.

All metric-gathering commands are combined into a single SSH exec call per cycle (no sleep or multi-sample within a cycle). CPU usage percentages and network throughput rates are computed by comparing current raw values against the previous cycle's stored values. The first cycle after connection produces raw values only (no rates until the second cycle).

Each collection has a 5-second timeout. A single timeout skips the current cycle. Three consecutive failures trigger an SSH reconnection request (implemented in slice #19; for now, just track the failure count and log it).

The collector integrates with the server lifecycle: when a server is added (saved to DB), a collector starts; when deleted, it stops.

## Acceptance criteria

- [ ] One goroutine per server, managed by a collector manager
- [ ] Combined SSH command collects all metrics in single exec call
- [ ] 2-second collection interval via ticker
- [ ] Cross-cycle delta computation for CPU percentages
- [ ] Cross-cycle delta computation for network throughput (bytes/s)
- [ ] First cycle produces partial metrics (no rates), second cycle produces full metrics
- [ ] 5-second per-collection timeout; timeout skips cycle without error
- [ ] Consecutive failure counter incremented on collection error, reset on success
- [ ] Log warning at 3 consecutive failures (reconnection hook point for slice #19)
- [ ] Collector starts when server is added, stops when server is deleted
- [ ] Context-based cancellation for clean shutdown
- [ ] Latest metrics stored in memory, accessible for hub broadcasting

## Blocked by

- #4 SSH Connection Testing and Host Key Verification
- #9 Metrics Parsers
