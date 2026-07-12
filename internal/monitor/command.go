package monitor

import (
	"fmt"
	"strings"

	"belochka/internal/model"
)

// sectionDelimiter separates the output of each command in the combined SSH exec.
const sectionDelimiter = "---BELOCHKA-SECTION---"

// CollectCommand returns the combined shell command that collects all metrics
// in a single SSH exec call. Each section is separated by sectionDelimiter.
func CollectCommand() string {
	commands := []string{
		"cat /proc/stat",
		"cat /proc/meminfo",
		"df -B1 -x tmpfs -x devtmpfs -x overlay -x squashfs",
		"cat /proc/net/dev",
		"top -bn1 -o %CPU | head -27",
		"hostname",
		"uname -r",
		"cat /proc/uptime",
		"cat /etc/os-release",
		"nproc",
	}

	parts := make([]string, 0, len(commands)*2-1)
	for i, cmd := range commands {
		if i > 0 {
			parts = append(parts, "echo '"+sectionDelimiter+"'")
		}
		parts = append(parts, cmd)
	}

	return strings.Join(parts, "; ")
}

const sectionCount = 10

// ParseCombinedOutput splits the combined SSH output by sectionDelimiter
// and parses each section into the corresponding metrics.
func ParseCombinedOutput(output string) (model.Metrics, error) {
	sections := strings.Split(output, sectionDelimiter)
	if len(sections) != sectionCount {
		return model.Metrics{}, fmt.Errorf("expected %d sections, got %d", sectionCount, len(sections))
	}

	// Trim whitespace from each section
	for i := range sections {
		sections[i] = strings.TrimSpace(sections[i])
	}

	var m model.Metrics
	var err error

	m.CPU, err = ParseCPU(sections[0])
	if err != nil {
		return m, fmt.Errorf("parse cpu: %w", err)
	}

	m.Memory, err = ParseMemory(sections[1])
	if err != nil {
		return m, fmt.Errorf("parse memory: %w", err)
	}

	m.Disk, err = ParseDisk(sections[2])
	if err != nil {
		return m, fmt.Errorf("parse disk: %w", err)
	}

	m.Network, err = ParseNetwork(sections[3])
	if err != nil {
		return m, fmt.Errorf("parse network: %w", err)
	}

	m.Process, err = ParseProcesses(sections[4])
	if err != nil {
		return m, fmt.Errorf("parse processes: %w", err)
	}

	m.System, err = ParseSystemInfo(
		sections[5], // hostname
		sections[6], // uname -r
		sections[7], // /proc/uptime
		sections[8], // /etc/os-release
		sections[9], // nproc
	)
	if err != nil {
		return m, fmt.Errorf("parse system info: %w", err)
	}

	return m, nil
}

// totalJiffies returns the sum of all jiffy counters for a CPU core.
func totalJiffies(c model.CPUCore) uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
}

// cpuPct computes the percentage contribution of delta within totalDelta.
func cpuPct(delta, totalDelta uint64) float64 {
	if totalDelta == 0 {
		return 0
	}
	return float64(delta) / float64(totalDelta) * 100
}

// ComputeCPUUsage computes CPU usage percentages from the delta between
// two consecutive readings of a single core's jiffy counters.
func ComputeCPUUsage(prev, curr model.CPUCore) model.CPUUsage {
	totalDelta := totalJiffies(curr) - totalJiffies(prev)
	if totalDelta == 0 {
		return model.CPUUsage{Name: curr.Name}
	}

	idleDelta := curr.Idle - prev.Idle
	iowaitDelta := curr.IOWait - prev.IOWait
	userDelta := (curr.User - prev.User) + (curr.Nice - prev.Nice)
	systemDelta := curr.System - prev.System
	stealDelta := curr.Steal - prev.Steal

	usedDelta := totalDelta - idleDelta - iowaitDelta

	return model.CPUUsage{
		Name:      curr.Name,
		UsedPct:   cpuPct(usedDelta, totalDelta),
		UserPct:   cpuPct(userDelta, totalDelta),
		SystemPct: cpuPct(systemDelta, totalDelta),
		IOWaitPct: cpuPct(iowaitDelta, totalDelta),
		StealPct:  cpuPct(stealDelta, totalDelta),
	}
}

// ComputeNetworkRates computes per-interface throughput in bytes/s from
// the delta between two consecutive readings divided by the interval in seconds.
// Interfaces in curr that have no matching entry in prev get zero rates.
func ComputeNetworkRates(prev, curr []model.NetworkInterface, intervalSec float64) []model.NetworkRate {
	prevMap := make(map[string]model.NetworkInterface, len(prev))
	for _, iface := range prev {
		prevMap[iface.Name] = iface
	}

	rates := make([]model.NetworkRate, 0, len(curr))
	for _, c := range curr {
		rate := model.NetworkRate{Name: c.Name}
		if p, ok := prevMap[c.Name]; ok && intervalSec > 0 {
			rate.RxBytesPS = float64(c.RxBytes-p.RxBytes) / intervalSec
			rate.TxBytesPS = float64(c.TxBytes-p.TxBytes) / intervalSec
		}
		rates = append(rates, rate)
	}
	return rates
}
