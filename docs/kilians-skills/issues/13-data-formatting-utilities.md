## What to build

Implement frontend utility functions for formatting metric values into human-readable strings, and color threshold functions for usage-based color coding. These utilities are used across dashboard cards and detail pages.

Formatting rules from the PRD:
- Memory/disk: 1024-based units (KiB, MiB, GiB), 1 decimal place
- Network: bytes/s display (KB/s, MB/s), 1 decimal place
- Percentages: 1 decimal place

Color thresholds (step function, not gradient):
- Green: 0-60%
- Yellow: 60-80%
- Red: 80-100%
- Applies to CPU, memory, disk. Network has no color coding.

## Acceptance criteria

- [ ] `formatBytes(bytes)` returns human-readable string with 1024-base units (KiB, MiB, GiB), 1 decimal
- [ ] `formatNetworkSpeed(bytesPerSec)` returns KB/s or MB/s, 1 decimal
- [ ] `formatPercent(value)` returns percentage string with 1 decimal place
- [ ] `getUsageColor(percent)` returns color class/token: green (0-60%), yellow (60-80%), red (80-100%)
- [ ] Edge cases handled: 0, negative values, very large values, NaN/undefined
- [ ] Vitest unit tests for all functions with boundary values (59.9%, 60%, 60.1%, 79.9%, 80%, 80.1%)

## Blocked by

- #1 Project Scaffold
