package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"belochka/internal/model"
)

// SSHExecutor abstracts SSH command execution for testability.
type SSHExecutor interface {
	Execute(ctx context.Context, serverID, cmd string) (string, error)
}

// CollectorOptions configures a Collector.
type CollectorOptions struct {
	Interval time.Duration // collection interval (default 2s)
	Timeout  time.Duration // per-collection timeout (default 5s)
}

func (o CollectorOptions) withDefaults() CollectorOptions {
	if o.Interval == 0 {
		o.Interval = 2 * time.Second
	}
	if o.Timeout == 0 {
		o.Timeout = 5 * time.Second
	}
	return o
}

// Collector runs a metrics collection loop for a single server.
type Collector struct {
	serverID string
	executor SSHExecutor
	opts     CollectorOptions

	// OnFailureThreshold is called when consecutive failures reach
	// CollectionFailureThreshold (3). It fires once per threshold crossing.
	// The failure count is passed as an argument.
	OnFailureThreshold func(failures int)

	mu               sync.RWMutex
	latest           *model.Snapshot
	prevMetrics      *model.Metrics
	prevTime         time.Time
	failures         int
	thresholdFired   bool // true after OnFailureThreshold has been called
}

// NewCollector creates a new Collector for the given server.
func NewCollector(serverID string, executor SSHExecutor, opts CollectorOptions) *Collector {
	return &Collector{
		serverID: serverID,
		executor: executor,
		opts:     opts.withDefaults(),
	}
}

// Run starts the collection loop. It blocks until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.opts.Interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// recordFailure increments the failure counter and fires the threshold callback
// when it crosses CollectionFailureThreshold. Returns the current failure count.
func (c *Collector) recordFailure() int {
	c.mu.Lock()
	c.failures++
	failures := c.failures
	shouldFire := failures == 3 && !c.thresholdFired && c.OnFailureThreshold != nil
	if failures >= 3 {
		c.thresholdFired = true
	}
	c.mu.Unlock()

	if failures >= 3 {
		slog.Warn("3+ consecutive collection failures",
			"server_id", c.serverID,
			"failures", failures,
		)
	}
	if shouldFire {
		c.OnFailureThreshold(failures)
	}
	return failures
}

// collect performs a single collection cycle.
func (c *Collector) collect(ctx context.Context) {
	execCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	cmd := CollectCommand()
	output, err := c.executor.Execute(execCtx, c.serverID, cmd)
	if err != nil {
		c.recordFailure()
		return
	}

	metrics, err := ParseCombinedOutput(output)
	if err != nil {
		c.recordFailure()
		return
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures = 0
	c.thresholdFired = false

	if c.prevMetrics == nil {
		// First cycle: partial snapshot (no deltas)
		c.latest = &model.Snapshot{
			ServerID:    c.serverID,
			Memory:      metrics.Memory,
			Disk:        metrics.Disk,
			Process:     metrics.Process,
			System:      metrics.System,
			CollectedAt: now,
			Partial:     true,
		}
	} else {
		// Compute deltas
		intervalSec := now.Sub(c.prevTime).Seconds()

		// CPU usage: aggregate + per-core
		cpuUsages := make([]model.CPUUsage, 0, 1+len(metrics.CPU.Cores))
		cpuUsages = append(cpuUsages, ComputeCPUUsage(c.prevMetrics.CPU.Aggregate, metrics.CPU.Aggregate))
		for i, core := range metrics.CPU.Cores {
			if i < len(c.prevMetrics.CPU.Cores) {
				cpuUsages = append(cpuUsages, ComputeCPUUsage(c.prevMetrics.CPU.Cores[i], core))
			}
		}

		// Network rates
		netRates := ComputeNetworkRates(c.prevMetrics.Network.Interfaces, metrics.Network.Interfaces, intervalSec)

		c.latest = &model.Snapshot{
			ServerID:    c.serverID,
			CPU:         cpuUsages,
			Memory:      metrics.Memory,
			Disk:        metrics.Disk,
			Network:     netRates,
			Process:     metrics.Process,
			System:      metrics.System,
			CollectedAt: now,
			Partial:     false,
		}
	}

	c.prevMetrics = &metrics
	c.prevTime = now
}

// Latest returns the most recent snapshot, or nil if none collected yet.
func (c *Collector) Latest() *model.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

// ConsecutiveFailures returns the current consecutive failure count.
func (c *Collector) ConsecutiveFailures() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.failures
}

// managedCollector pairs a Collector with its cancel function.
type managedCollector struct {
	collector *Collector
	cancel    context.CancelFunc
}

// Manager manages collectors for multiple servers.
type Manager struct {
	executor SSHExecutor
	opts     CollectorOptions

	mu                 sync.RWMutex
	collectors         map[string]*managedCollector
	onFailureThreshold func(serverID string, failures int)
}

// NewManager creates a new Manager.
func NewManager(executor SSHExecutor, opts CollectorOptions) *Manager {
	return &Manager{
		executor:   executor,
		opts:       opts,
		collectors: make(map[string]*managedCollector),
	}
}

// SetOnFailureThreshold sets a callback invoked when any collector
// reaches CollectionFailureThreshold consecutive failures.
func (m *Manager) SetOnFailureThreshold(fn func(serverID string, failures int)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFailureThreshold = fn
}

// Add starts a collector for the given server. If a collector already exists
// for that server, it is a no-op.
func (m *Manager) Add(ctx context.Context, serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.collectors[serverID]; exists {
		return
	}

	collCtx, cancel := context.WithCancel(ctx)
	c := NewCollector(serverID, m.executor, m.opts)
	if m.onFailureThreshold != nil {
		fn := m.onFailureThreshold
		sid := serverID
		c.OnFailureThreshold = func(failures int) {
			fn(sid, failures)
		}
	}
	mc := &managedCollector{collector: c, cancel: cancel}
	m.collectors[serverID] = mc

	go c.Run(collCtx)
}

// Remove stops and removes the collector for the given server.
func (m *Manager) Remove(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mc, ok := m.collectors[serverID]
	if !ok {
		return
	}
	mc.cancel()
	delete(m.collectors, serverID)
}

// Latest returns the most recent snapshot for a server, or nil.
func (m *Manager) Latest(serverID string) *model.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mc, ok := m.collectors[serverID]
	if !ok {
		return nil
	}
	return mc.collector.Latest()
}

// AllSnapshots returns the latest snapshot for each managed server.
func (m *Manager) AllSnapshots() []model.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]model.Snapshot, 0, len(m.collectors))
	for _, mc := range m.collectors {
		snap := mc.collector.Latest()
		if snap != nil {
			snapshots = append(snapshots, *snap)
		}
	}
	return snapshots
}

// ServerIDs returns the IDs of all managed servers.
func (m *Manager) ServerIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.collectors))
	for id := range m.collectors {
		ids = append(ids, id)
	}
	return ids
}

// StopAll stops all collectors and clears the manager.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mc := range m.collectors {
		mc.cancel()
	}
	m.collectors = make(map[string]*managedCollector)
}

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

// ComputeCPUUsage computes CPU usage percentages from the delta between
// two consecutive readings of a single core's jiffy counters.
func ComputeCPUUsage(prev, curr model.CPUCore) model.CPUUsage {
	totalDelta := totalJiffies(curr) - totalJiffies(prev)
	if totalDelta == 0 {
		return model.CPUUsage{Name: curr.Name}
	}

	pct := func(delta uint64) float64 {
		return float64(delta) / float64(totalDelta) * 100
	}

	idleDelta := curr.Idle - prev.Idle
	iowaitDelta := curr.IOWait - prev.IOWait
	userDelta := (curr.User - prev.User) + (curr.Nice - prev.Nice)
	systemDelta := curr.System - prev.System
	stealDelta := curr.Steal - prev.Steal

	usedDelta := totalDelta - idleDelta - iowaitDelta

	return model.CPUUsage{
		Name:      curr.Name,
		UsedPct:   pct(usedDelta),
		UserPct:   pct(userDelta),
		SystemPct: pct(systemDelta),
		IOWaitPct: pct(iowaitDelta),
		StealPct:  pct(stealDelta),
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
