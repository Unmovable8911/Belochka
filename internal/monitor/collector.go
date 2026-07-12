package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"belochka/internal/clock"
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
	clock    clock.Clock

	// OnFailureThreshold is called when consecutive failures reach
	// CollectionFailureThreshold (3). It fires once per threshold crossing.
	// The failure count is passed as an argument.
	OnFailureThreshold func(failures int)

	mu             sync.RWMutex
	latest         *model.Snapshot
	prevMetrics    *model.Metrics
	prevTime       time.Time
	failures       int
	thresholdFired bool // true after OnFailureThreshold has been called
}

// NewCollector creates a new Collector for the given server.
func NewCollector(serverID string, executor SSHExecutor, opts CollectorOptions, clk clock.Clock) *Collector {
	return &Collector{
		serverID: serverID,
		executor: executor,
		opts:     opts.withDefaults(),
		clock:    clk,
	}
}

// Run starts the collection loop. It blocks until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	ticker := c.clock.NewTicker(c.opts.Interval)
	defer ticker.Stop()

	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
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
		// Only log at WARN for the first few failures; after that,
		// the connection is likely permanently dead and reconnection
		// has already stopped. Switch to DEBUG to avoid log spam.
		if failures <= 5 {
			slog.Warn("3+ consecutive collection failures",
				"server_id", c.serverID,
				"failures", failures,
			)
		} else {
			slog.Debug("collection failure (connection dead)",
				"server_id", c.serverID,
				"failures", failures,
			)
		}
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

	now := c.clock.Now()

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
			ServerID:     c.serverID,
			AggregateCPU: &cpuUsages[0],
			Cores:        cpuUsages[1:],
			Memory:       metrics.Memory,
			Disk:         metrics.Disk,
			Network:      netRates,
			Process:      metrics.Process,
			System:       metrics.System,
			CollectedAt:  now,
			Partial:      false,
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

// managedCollector pairs a Collector with its cancel function.
type managedCollector struct {
	collector *Collector
	cancel    context.CancelFunc
}

// Manager manages collectors for multiple servers.
type Manager struct {
	executor SSHExecutor
	opts     CollectorOptions
	clock    clock.Clock

	mu                 sync.RWMutex
	collectors         map[string]*managedCollector
	onFailureThreshold func(serverID string, failures int)
}

// NewManager creates a new Manager.
func NewManager(executor SSHExecutor, opts CollectorOptions, clk clock.Clock) *Manager {
	return &Manager{
		executor:   executor,
		opts:       opts,
		clock:      clk,
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
	c := NewCollector(serverID, m.executor, m.opts, m.clock)
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

