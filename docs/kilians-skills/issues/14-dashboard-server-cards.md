## What to build

Build the main dashboard view: a responsive grid of server cards showing live metrics from the WebSocket state. Each card displays the server's name, host, and four metric sections: CPU usage bar, memory usage bar, disk (highest-usage partition with mount point label), and network throughput (aggregated from physical interfaces).

Progress bars are color-coded using the threshold functions from slice #13. Cards are ordered by `CreatedAt` timestamp (add order) — position is fixed regardless of connection state. Cards are wrapped in `React.memo` for render optimization.

Network aggregation filters out virtual interfaces (lo, docker*, veth*, br-*, virbr*) and sums throughput from remaining physical interfaces.

## Acceptance criteria

- [ ] Dashboard route (`/`) renders a responsive grid of server cards
- [ ] Each card shows: server name, host address, status indicator
- [ ] CPU usage: horizontal progress bar with percentage, color-coded
- [ ] Memory usage: horizontal progress bar with percentage, color-coded
- [ ] Disk: highest-usage partition shown with mount point label, usage bar, color-coded
- [ ] Network: aggregated throughput (RX/TX) from physical interfaces only
- [ ] Network filtering: excludes lo, docker*, veth*, br-*, virbr*
- [ ] Cards ordered by CreatedAt, position stable
- [ ] Cards wrapped in `React.memo` to prevent unnecessary re-renders
- [ ] Cards are clickable, navigating to `/server/:id`
- [ ] Live updates every 2 seconds from WebSocket state

## Blocked by

- #10 Metrics Collector
- #12 React WebSocket State Management
- #13 Data Formatting Utilities
