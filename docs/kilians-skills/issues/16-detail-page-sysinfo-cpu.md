## What to build

Build the server detail page with the first two sections: system information bar and CPU visualization. The detail page is accessed by clicking a server card, navigating to `/server/:id` (UUID in URL).

The system info bar displays: hostname, kernel version, uptime, OS name, and CPU core count — all from the parsed system info metrics.

The CPU section shows an overall CPU usage ring gauge (implemented with CSS conic-gradient) and per-core horizontal progress bars below it. Both use the standard color thresholds (green/yellow/red).

## Acceptance criteria

- [ ] Detail page route `/server/:id` resolves to detail view
- [ ] Page receives live metrics from the shared WebSocket state (no separate connection)
- [ ] System info bar displays: hostname, kernel, uptime (human-readable), OS, core count
- [ ] CPU ring gauge using conic-gradient showing overall CPU percentage
- [ ] Ring gauge color follows threshold (green/yellow/red)
- [ ] Per-core horizontal progress bars listed below the ring gauge
- [ ] Each core bar labeled (Core 0, Core 1, ...) with percentage and color coding
- [ ] Values update every 2 seconds from WebSocket
- [ ] Navigation back to dashboard (back button or breadcrumb)
- [ ] Handles missing/loading state gracefully (server not yet sending metrics)

## Blocked by

- #14 Dashboard Server Cards
