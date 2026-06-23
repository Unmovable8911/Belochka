## What to build

Add the network throughput section and process table to the server detail page, completing the 2x2 metrics grid and the process view below it.

The network section shows per-interface throughput for ALL interfaces (unlike the dashboard which filters). Each interface displays its name with RX and TX rates in human-readable network speed units. No color coding for network — values only.

The process table shows the top 20 processes, directly replaced on each 2-second update. Column headers are clickable for client-side re-sorting. Default sort is by CPU%. Available sort columns: PID, user, CPU%, memory%, command.

## Acceptance criteria

- [ ] Network section shows all interfaces (including virtual ones, unlike dashboard)
- [ ] Each interface: name, RX rate, TX rate in KB/s or MB/s
- [ ] No color coding on network values
- [ ] Process table with columns: PID, User, CPU%, Memory%, Command
- [ ] Top 20 processes displayed
- [ ] Default sort by CPU% descending
- [ ] Clickable column headers toggle sort (ascending/descending)
- [ ] Client-side sorting — no server request on re-sort
- [ ] Table content directly replaced on each update (no animation/transition)
- [ ] Values update live every 2 seconds
- [ ] Graceful handling of empty process list (BusyBox systems)

## Blocked by

- #16 Server Detail Page — System Info and CPU
