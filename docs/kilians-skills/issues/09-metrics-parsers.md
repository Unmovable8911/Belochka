## What to build

Implement pure parsing functions that take raw command output strings (as collected from remote Linux servers) and return typed Go structs. These parsers are the highest-value test seam in the project — they have no external dependencies and can be tested with captured output from various Linux distributions.

Parsers needed:
- **CPU**: Parse `/proc/stat` for overall and per-core usage (raw jiffies; rate computation is the collector's job)
- **Memory**: Parse `/proc/meminfo` for total, used, available, swap total, swap used
- **Disk**: Parse `df -B1 -x tmpfs -x devtmpfs -x overlay -x squashfs` output for mount point, total, used, available per partition
- **Network**: Parse `/proc/net/dev` for per-interface bytes received/transmitted (raw counters; rate computation is the collector's job)
- **Processes**: Parse `top -bn1 -o %CPU | head -27` for PID, user, CPU%, memory%, command
- **System info**: Parse hostname, `uname -r`, `/proc/uptime`, `/etc/os-release`, and CPU core count

Define the typed `Metrics` model structs that these parsers populate.

## Acceptance criteria

- [ ] `model.Metrics` struct hierarchy defined (CPU, Memory, Disk, Network, Processes, SystemInfo)
- [ ] CPU parser: handles `/proc/stat` format, returns per-core raw jiffies
- [ ] Memory parser: extracts total, used, available, swap from `/proc/meminfo`
- [ ] Disk parser: handles `df -B1` output, returns list of partitions with mount/total/used/avail
- [ ] Network parser: handles `/proc/net/dev`, returns per-interface raw byte counters
- [ ] Process parser: handles `top -bn1` procps-ng output, returns top process list
- [ ] Process parser: returns empty list gracefully for non-procps-ng (BusyBox) output
- [ ] System info parser: extracts hostname, kernel, uptime, OS name, core count
- [ ] Unit tests with captured output samples from Ubuntu, Debian, CentOS
- [ ] All parsers are pure functions (string in, struct out) with no side effects

## Blocked by

- #1 Project Scaffold
