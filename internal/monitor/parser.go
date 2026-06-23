package monitor

import (
	"fmt"
	"strconv"
	"strings"

	"belochka/internal/model"
)

// ParseCPU parses /proc/stat output and returns CPU metrics with raw jiffy counters.
func ParseCPU(input string) (model.CPUMetrics, error) {
	var result model.CPUMetrics
	foundAggregate := false

	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		// Only parse lines starting with "cpu" followed by optional digit
		name := fields[0]
		if name != "cpu" && (len(name) < 4 || name[3] < '0' || name[3] > '9') {
			continue
		}

		core, err := parseCPUFields(name, fields[1:])
		if err != nil {
			return result, fmt.Errorf("parsing %s: %w", name, err)
		}

		if name == "cpu" {
			result.Aggregate = core
			foundAggregate = true
		} else {
			result.Cores = append(result.Cores, core)
		}
	}

	if !foundAggregate {
		return result, fmt.Errorf("no aggregate cpu line found in /proc/stat output")
	}

	return result, nil
}

func parseCPUFields(name string, fields []string) (model.CPUCore, error) {
	vals := make([]uint64, len(fields))
	for i, f := range fields {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return model.CPUCore{}, fmt.Errorf("field %d (%q): %w", i, f, err)
		}
		vals[i] = v
	}
	core := model.CPUCore{
		Name:    name,
		User:    vals[0],
		Nice:    vals[1],
		System:  vals[2],
		Idle:    vals[3],
		IOWait:  vals[4],
		IRQ:     vals[5],
		SoftIRQ: vals[6],
	}
	if len(vals) > 7 {
		core.Steal = vals[7]
	}
	return core, nil
}

// ParseMemory parses /proc/meminfo output and returns memory metrics in bytes.
func ParseMemory(input string) (model.MemoryMetrics, error) {
	fields := map[string]uint64{}
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valParts := strings.Fields(strings.TrimSpace(parts[1]))
		if len(valParts) == 0 {
			continue
		}
		v, err := strconv.ParseUint(valParts[0], 10, 64)
		if err != nil {
			continue
		}
		// /proc/meminfo values are in kB
		fields[key] = v * 1024
	}

	total, ok := fields["MemTotal"]
	if !ok {
		return model.MemoryMetrics{}, fmt.Errorf("MemTotal not found in /proc/meminfo output")
	}

	available := fields["MemAvailable"]
	swapTotal := fields["SwapTotal"]
	swapFree := fields["SwapFree"]

	var swapUsed uint64
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}

	var used uint64
	if total > available {
		used = total - available
	}

	return model.MemoryMetrics{
		Total:     total,
		Used:      used,
		Available: available,
		SwapTotal: swapTotal,
		SwapUsed:  swapUsed,
	}, nil
}

// ParseDisk parses `df -B1` output and returns disk metrics.
// The header line is skipped; each subsequent line is a partition.
func ParseDisk(input string) (model.DiskMetrics, error) {
	var result model.DiskMetrics

	lines := strings.Split(input, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		total, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		used, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		avail, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			continue
		}

		result.Partitions = append(result.Partitions, model.DiskPartition{
			Filesystem: fields[0],
			Total:      total,
			Used:       used,
			Available:  avail,
			MountPoint: fields[5],
		})
	}

	return result, nil
}

// ParseNetwork parses /proc/net/dev output and returns per-interface raw byte counters.
func ParseNetwork(input string) (model.NetworkMetrics, error) {
	var result model.NetworkMetrics

	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		// Split on ":" — name is before, stats are after
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		// /proc/net/dev has 16 fields: 8 receive + 8 transmit
		// RxBytes is field 0, TxBytes is field 8
		if len(fields) < 10 {
			continue
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			continue
		}

		result.Interfaces = append(result.Interfaces, model.NetworkInterface{
			Name:    name,
			RxBytes: rxBytes,
			TxBytes: txBytes,
		})
	}

	return result, nil
}

// ParseProcesses parses `top -bn1 -o %CPU | head -27` output from procps-ng.
// Returns an empty process list for unrecognized formats (e.g., BusyBox top).
func ParseProcesses(input string) (model.ProcessMetrics, error) {
	var result model.ProcessMetrics

	lines := strings.Split(input, "\n")

	// Find the header line with PID column — this identifies procps-ng format.
	// procps-ng header: "  PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND"
	headerIdx := -1
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 12 && fields[0] == "PID" && fields[1] == "USER" {
			headerIdx = i
			break
		}
	}

	if headerIdx < 0 {
		// Not procps-ng format — return empty list gracefully
		return result, nil
	}

	// Parse data lines after the header
	for _, line := range lines[headerIdx+1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		// procps-ng data line: PID USER PR NI VIRT RES SHR S %CPU %MEM TIME+ COMMAND
		// Indices:              0   1   2  3  4    5   6   7  8    9    10    11+
		if len(fields) < 12 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		cpuPct, err := strconv.ParseFloat(fields[8], 64)
		if err != nil {
			continue
		}
		memPct, err := strconv.ParseFloat(fields[9], 64)
		if err != nil {
			continue
		}

		// Command may contain spaces; join remaining fields
		command := strings.Join(fields[11:], " ")

		result.Processes = append(result.Processes, model.Process{
			PID:     pid,
			User:    fields[1],
			CPUPct:  cpuPct,
			MemPct:  memPct,
			Command: command,
		})
	}

	return result, nil
}

// ParseSystemInfo parses system information from individual command outputs.
// Parameters:
//   - hostname: output of `hostname` command
//   - uname: output of `uname -r` command
//   - uptime: contents of /proc/uptime (two space-separated floats)
//   - osRelease: contents of /etc/os-release
//   - nproc: output of `nproc` command (core count)
func ParseSystemInfo(hostname, uname, uptime, osRelease, nproc string) (model.SystemInfo, error) {
	info := model.SystemInfo{
		Hostname: strings.TrimSpace(hostname),
		Kernel:   strings.TrimSpace(uname),
	}

	// Parse uptime (first field is total uptime in seconds)
	uptimeFields := strings.Fields(strings.TrimSpace(uptime))
	if len(uptimeFields) > 0 {
		u, err := strconv.ParseFloat(uptimeFields[0], 64)
		if err != nil {
			return info, fmt.Errorf("parsing uptime %q: %w", uptimeFields[0], err)
		}
		info.UptimeSec = u
	}

	// Parse OS name from PRETTY_NAME in /etc/os-release
	info.OSName = parseOSReleasePrettyName(osRelease)

	// Parse core count
	coreStr := strings.TrimSpace(nproc)
	if coreStr != "" {
		cores, err := strconv.Atoi(coreStr)
		if err != nil {
			return info, fmt.Errorf("parsing core count %q: %w", coreStr, err)
		}
		info.CoreCount = cores
	}

	return info, nil
}

// parseOSReleasePrettyName extracts PRETTY_NAME from /etc/os-release content.
func parseOSReleasePrettyName(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			value := strings.TrimPrefix(line, "PRETTY_NAME=")
			// Remove surrounding quotes if present
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}
