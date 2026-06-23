## What to build

Add the memory and disk sections to the server detail page. These appear alongside the CPU section in a 2x2 grid layout.

The memory section shows a ring gauge (conic-gradient) for memory usage percentage, plus swap information displayed as text (swap used / swap total). The ring gauge uses standard color thresholds.

The disk section shows all physical disk partitions (virtual filesystems already excluded by the `df` command). Each partition has a usage bar with mount point label, used/total values, and color coding.

## Acceptance criteria

- [ ] Memory ring gauge showing overall RAM usage percentage with color thresholds
- [ ] Memory details: used / total displayed in human-readable units (GiB/MiB)
- [ ] Swap information: swap used / swap total displayed below memory gauge
- [ ] Disk section: list of all physical partitions from metrics
- [ ] Each partition shows: mount point label, usage bar, used/total in human-readable units
- [ ] Disk bars use color thresholds (green/yellow/red)
- [ ] 2x2 grid layout maintained (CPU | Memory over Disk | Network)
- [ ] Values update live every 2 seconds

## Blocked by

- #16 Server Detail Page — System Info and CPU
