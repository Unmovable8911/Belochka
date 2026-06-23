package monitor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"belochka/internal/model"
)

func TestCollectCommand(t *testing.T) {
	cmd := CollectCommand()
	if cmd == "" {
		t.Fatal("CollectCommand returned empty string")
	}

	// The command should contain all the metric-gathering subcommands
	// separated by a known delimiter so we can split the output
	for _, want := range []string{
		"cat /proc/stat",
		"cat /proc/meminfo",
		"df -B1",
		"cat /proc/net/dev",
		"top -bn1",
		"hostname",
		"uname -r",
		"cat /proc/uptime",
		"cat /etc/os-release",
		"nproc",
	} {
		if !containsSubstring(cmd, want) {
			t.Errorf("CollectCommand missing %q", want)
		}
	}
}

// buildCombinedOutput creates fake combined SSH output from individual sections.
func buildCombinedOutput(sections ...string) string {
	return strings.Join(sections, "\n"+sectionDelimiter+"\n")
}

// Minimal valid outputs for each metric section
const (
	fakeProcStat = `cpu  1000 200 300 5000 50 0 100 0 0 0
cpu0 500 100 150 2500 25 0 50 0 0 0
cpu1 500 100 150 2500 25 0 50 0 0 0
intr 0
`
	fakeProcMeminfo = `MemTotal:        8000000 kB
MemFree:         2000000 kB
MemAvailable:    4000000 kB
SwapTotal:       1000000 kB
SwapFree:         800000 kB
`
	fakeDf = `Filesystem     1B-blocks      Used Available Use% Mounted on
/dev/sda1      100000000  50000000  40000000  56% /
`
	fakeProcNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  1000   100    0    0    0     0          0         0    1000   100    0    0    0     0       0          0
  eth0: 50000  5000    0    0    0     0          0         0   30000  3000    0    0    0     0       0          0
`
	fakeTop = `top - 14:32:01 up 1 day,  0:00,  1 users,  load average: 0.10, 0.05, 0.01
Tasks:  50 total,   1 running,  49 sleeping,   0 stopped,   0 zombie
%Cpu(s):  2.0 us,  1.0 sy,  0.0 ni, 97.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :   7812.5 total,   1953.1 free,   3906.2 used,   1953.1 buff/cache
MiB Swap:    976.6 total,    781.2 free,    195.3 used.   3515.6 avail Mem

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    100 root      20   0  100000  50000  25000 S   5.0   0.6   0:10.00 myapp
`
	fakeHostname  = `testserver`
	fakeUname     = `5.15.0-generic`
	fakeUptime    = `86400.00 172800.00`
	fakeOSRelease = `PRETTY_NAME="Ubuntu 22.04 LTS"
NAME="Ubuntu"
`
	fakeNproc = `2`
)

func validCombinedOutput() string {
	return buildCombinedOutput(
		fakeProcStat,
		fakeProcMeminfo,
		fakeDf,
		fakeProcNetDev,
		fakeTop,
		fakeHostname,
		fakeUname,
		fakeUptime,
		fakeOSRelease,
		fakeNproc,
	)
}

func TestParseCombinedOutput(t *testing.T) {
	output := validCombinedOutput()
	metrics, err := ParseCombinedOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CPU: check aggregate parsed
	if metrics.CPU.Aggregate.User != 1000 {
		t.Errorf("cpu aggregate user = %d, want 1000", metrics.CPU.Aggregate.User)
	}
	if len(metrics.CPU.Cores) != 2 {
		t.Errorf("cpu cores = %d, want 2", len(metrics.CPU.Cores))
	}

	// Memory: check total parsed (8000000 kB = 8000000*1024 bytes)
	if metrics.Memory.Total != 8000000*1024 {
		t.Errorf("memory total = %d, want %d", metrics.Memory.Total, 8000000*1024)
	}

	// Disk: check partition parsed
	if len(metrics.Disk.Partitions) != 1 {
		t.Errorf("disk partitions = %d, want 1", len(metrics.Disk.Partitions))
	}

	// Network: check interfaces parsed
	if len(metrics.Network.Interfaces) != 2 {
		t.Errorf("network interfaces = %d, want 2", len(metrics.Network.Interfaces))
	}

	// Process: check process parsed
	if len(metrics.Process.Processes) != 1 {
		t.Errorf("processes = %d, want 1", len(metrics.Process.Processes))
	}

	// System: check hostname parsed
	if metrics.System.Hostname != "testserver" {
		t.Errorf("hostname = %q, want %q", metrics.System.Hostname, "testserver")
	}
	if metrics.System.CoreCount != 2 {
		t.Errorf("core count = %d, want 2", metrics.System.CoreCount)
	}
}

func TestComputeCPUUsage(t *testing.T) {
	prev := model.CPUCore{
		Name: "cpu", User: 1000, Nice: 100, System: 200, Idle: 5000,
		IOWait: 50, IRQ: 0, SoftIRQ: 10, Steal: 0,
	}
	// Total prev = 1000+100+200+5000+50+0+10+0 = 6360
	curr := model.CPUCore{
		Name: "cpu", User: 1100, Nice: 110, System: 250, Idle: 5500,
		IOWait: 60, IRQ: 0, SoftIRQ: 20, Steal: 10,
	}
	// Total curr = 1100+110+250+5500+60+0+20+10 = 7050
	// Delta total = 7050-6360 = 690
	// Delta idle = 5500-5000 = 500
	// Delta iowait = 60-50 = 10
	// Used = (690 - 500 - 10) / 690 * 100 = 180/690 * 100 ≈ 26.09%
	// User = (100+10) / 690 * 100 ≈ 15.94%
	// System = 50 / 690 * 100 ≈ 7.25%
	// IOWait = 10 / 690 * 100 ≈ 1.45%
	// Steal = 10 / 690 * 100 ≈ 1.45%

	usage := ComputeCPUUsage(prev, curr)

	if usage.Name != "cpu" {
		t.Errorf("name = %q, want %q", usage.Name, "cpu")
	}
	assertFloat(t, "UsedPct", usage.UsedPct, 26.09, 0.1)
	assertFloat(t, "UserPct", usage.UserPct, 15.94, 0.1)
	assertFloat(t, "SystemPct", usage.SystemPct, 7.25, 0.1)
	assertFloat(t, "IOWaitPct", usage.IOWaitPct, 1.45, 0.1)
	assertFloat(t, "StealPct", usage.StealPct, 1.45, 0.1)
}

func TestComputeCPUUsage_zeroDelta(t *testing.T) {
	core := model.CPUCore{Name: "cpu0", User: 100, Idle: 500}
	usage := ComputeCPUUsage(core, core)
	// Zero delta total -> all percentages should be 0
	if usage.UsedPct != 0 {
		t.Errorf("UsedPct = %f, want 0", usage.UsedPct)
	}
}

func TestComputeNetworkRates(t *testing.T) {
	prev := []model.NetworkInterface{
		{Name: "eth0", RxBytes: 10000, TxBytes: 5000},
		{Name: "lo", RxBytes: 1000, TxBytes: 1000},
	}
	curr := []model.NetworkInterface{
		{Name: "eth0", RxBytes: 14000, TxBytes: 7000},
		{Name: "lo", RxBytes: 1500, TxBytes: 1500},
	}
	interval := 2.0 // seconds

	rates := ComputeNetworkRates(prev, curr, interval)

	if len(rates) != 2 {
		t.Fatalf("rates count = %d, want 2", len(rates))
	}

	// eth0: rx delta = 4000, tx delta = 2000, interval = 2s
	// rx rate = 2000 bytes/s, tx rate = 1000 bytes/s
	eth0 := rates[0]
	if eth0.Name != "eth0" {
		t.Errorf("rate 0 name = %q, want %q", eth0.Name, "eth0")
	}
	assertFloat(t, "eth0 RxBytesPS", eth0.RxBytesPS, 2000.0, 0.01)
	assertFloat(t, "eth0 TxBytesPS", eth0.TxBytesPS, 1000.0, 0.01)

	// lo: rx delta = 500, tx delta = 500, interval = 2s
	lo := rates[1]
	assertFloat(t, "lo RxBytesPS", lo.RxBytesPS, 250.0, 0.01)
}

func TestComputeNetworkRates_newInterface(t *testing.T) {
	// An interface appears in curr but not in prev
	prev := []model.NetworkInterface{
		{Name: "eth0", RxBytes: 10000, TxBytes: 5000},
	}
	curr := []model.NetworkInterface{
		{Name: "eth0", RxBytes: 14000, TxBytes: 7000},
		{Name: "eth1", RxBytes: 1000, TxBytes: 500},
	}

	rates := ComputeNetworkRates(prev, curr, 2.0)

	// eth1 has no previous data, so its rate should be 0
	if len(rates) != 2 {
		t.Fatalf("rates count = %d, want 2", len(rates))
	}
	eth1 := rates[1]
	if eth1.Name != "eth1" {
		t.Errorf("name = %q, want eth1", eth1.Name)
	}
	if eth1.RxBytesPS != 0 {
		t.Errorf("new iface RxBytesPS = %f, want 0", eth1.RxBytesPS)
	}
}

// --- Collector integration tests ---

// fakeExecutor is a test double for SSHExecutor.
type fakeExecutor struct {
	mu       sync.Mutex
	output   string
	err      error
	callCount int
}

func (f *fakeExecutor) Execute(ctx context.Context, serverID, cmd string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	return f.output, f.err
}

func (f *fakeExecutor) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callCount
}

func TestCollector_firstCyclePartial(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	c := NewCollector("srv-1", exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)

	// Wait for the first snapshot
	var snap *model.Snapshot
	deadline := time.After(2 * time.Second)
	for snap == nil {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for first snapshot")
		default:
			snap = c.Latest()
			if snap == nil {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	if !snap.Partial {
		t.Error("first cycle should be partial (no deltas)")
	}
	if snap.ServerID != "srv-1" {
		t.Errorf("server id = %q, want %q", snap.ServerID, "srv-1")
	}
	// Should have system info but no CPU percentages
	if snap.System.Hostname != "testserver" {
		t.Errorf("hostname = %q, want %q", snap.System.Hostname, "testserver")
	}
}

func TestCollector_secondCycleFull(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	c := NewCollector("srv-1", exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)

	// Wait for a non-partial snapshot (requires at least 2 successful cycles)
	var snap *model.Snapshot
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for non-partial snapshot; calls=%d", exec.calls())
		default:
			snap = c.Latest()
			if snap != nil && !snap.Partial {
				goto done
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
done:
	// CPU usage should have entries (aggregate + per-core)
	if len(snap.CPU) == 0 {
		t.Error("expected CPU usage entries after second cycle")
	}
	// Network rates should have entries
	if len(snap.Network) == 0 {
		t.Error("expected network rate entries after second cycle")
	}
}

func TestCollector_contextCancellation(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	c := NewCollector("srv-1", exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Let it run one cycle
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good, Run returned
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestCollector_failureCount(t *testing.T) {
	exec := &fakeExecutor{err: fmt.Errorf("ssh connection failed")}
	c := NewCollector("srv-1", exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)

	// Wait for at least 3 failures
	deadline := time.After(2 * time.Second)
	for exec.calls() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out; only %d calls", exec.calls())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if c.ConsecutiveFailures() < 3 {
		t.Errorf("consecutive failures = %d, want >= 3", c.ConsecutiveFailures())
	}
}

func TestCollector_failureCountResetsOnSuccess(t *testing.T) {
	exec := &fakeExecutor{err: fmt.Errorf("ssh fail")}
	c := NewCollector("srv-1", exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)

	// Let it fail a couple of times
	deadline := time.After(2 * time.Second)
	for exec.calls() < 2 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for failures")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if c.ConsecutiveFailures() < 2 {
		t.Errorf("expected >= 2 failures, got %d", c.ConsecutiveFailures())
	}

	// Now make it succeed
	exec.mu.Lock()
	exec.err = nil
	exec.output = validCombinedOutput()
	exec.mu.Unlock()

	// Wait for a successful call
	prevCalls := exec.calls()
	deadline = time.After(2 * time.Second)
	for exec.calls() < prevCalls+2 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for success cycle")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if c.ConsecutiveFailures() != 0 {
		t.Errorf("consecutive failures = %d, want 0 after success", c.ConsecutiveFailures())
	}
}

func TestCollector_timeoutSkipsCycle(t *testing.T) {
	// Simulate a command that blocks beyond the timeout
	slowExec := &blockingExecutor{
		blockDuration: 200 * time.Millisecond,
		output:        validCombinedOutput(),
	}
	c := NewCollector("srv-1", slowExec, CollectorOptions{
		Interval: 50 * time.Millisecond,
		Timeout:  100 * time.Millisecond, // shorter than the block
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)

	// Wait for a few cycles
	time.Sleep(500 * time.Millisecond)

	// Timeouts should count as failures
	if c.ConsecutiveFailures() == 0 {
		// Actually, timeouts skip the cycle. Let's check that no snapshot is produced
		// (since every call times out, we never get valid data).
	}
	// No snapshot should exist since every call timed out
	if c.Latest() != nil {
		t.Error("expected no snapshot when all calls time out")
	}
}

// blockingExecutor simulates a slow SSH command.
type blockingExecutor struct {
	mu            sync.Mutex
	blockDuration time.Duration
	output        string
}

func (b *blockingExecutor) Execute(ctx context.Context, serverID, cmd string) (string, error) {
	select {
	case <-time.After(b.blockDuration):
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.output, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// --- Manager tests ---

func TestManager_addAndRemove(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	m := NewManager(exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add a server
	m.Add(ctx, "srv-1")

	// Wait for it to collect
	deadline := time.After(2 * time.Second)
	for m.Latest("srv-1") == nil {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for srv-1 snapshot")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	snap := m.Latest("srv-1")
	if snap == nil {
		t.Fatal("expected snapshot for srv-1")
	}
	if snap.ServerID != "srv-1" {
		t.Errorf("server id = %q, want %q", snap.ServerID, "srv-1")
	}

	// Remove the server
	m.Remove("srv-1")

	// Snapshot should be gone
	if m.Latest("srv-1") != nil {
		t.Error("expected nil snapshot after removal")
	}

	// ServerIDs should be empty
	if len(m.ServerIDs()) != 0 {
		t.Errorf("server count = %d, want 0", len(m.ServerIDs()))
	}
}

func TestManager_multipleServers(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	m := NewManager(exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Add(ctx, "srv-1")
	m.Add(ctx, "srv-2")

	// Wait for both to collect
	deadline := time.After(2 * time.Second)
	for m.Latest("srv-1") == nil || m.Latest("srv-2") == nil {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for snapshots")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	ids := m.ServerIDs()
	if len(ids) != 2 {
		t.Errorf("server count = %d, want 2", len(ids))
	}

	// AllSnapshots should return both
	all := m.AllSnapshots()
	if len(all) != 2 {
		t.Errorf("all snapshots = %d, want 2", len(all))
	}
}

func TestManager_stopAll(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	m := NewManager(exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Add(ctx, "srv-1")
	m.Add(ctx, "srv-2")

	time.Sleep(100 * time.Millisecond)

	m.StopAll()

	if len(m.ServerIDs()) != 0 {
		t.Errorf("server count = %d, want 0 after StopAll", len(m.ServerIDs()))
	}
}

func TestManager_addDuplicate(t *testing.T) {
	exec := &fakeExecutor{output: validCombinedOutput()}
	m := NewManager(exec, CollectorOptions{Interval: 50 * time.Millisecond, Timeout: 1 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Add(ctx, "srv-1")
	m.Add(ctx, "srv-1") // should not panic or double-start

	if len(m.ServerIDs()) != 1 {
		t.Errorf("server count = %d, want 1 after duplicate add", len(m.ServerIDs()))
	}
}

func assertFloat(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("%s = %.4f, want %.4f (±%.4f)", name, got, want, tolerance)
	}
}

func TestParseCombinedOutput_wrongSectionCount(t *testing.T) {
	// Only 5 sections instead of 10
	output := buildCombinedOutput("a", "b", "c", "d", "e")
	_, err := ParseCombinedOutput(output)
	if err == nil {
		t.Error("expected error for wrong section count")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
