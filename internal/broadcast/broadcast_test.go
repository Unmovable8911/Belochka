package broadcast

import (
	"encoding/json"
	"testing"

	"belochka/internal/model"
)

func TestAssemble_WithSnapshot(t *testing.T) {
	servers := []ServerInfo{
		{
			ID:    "srv1",
			Name:  "web-1",
			Host:  "10.0.0.1",
			State: "connected",
		},
	}

	snap := &model.Snapshot{
		ServerID: "srv1",
		CPU: []model.CPUUsage{
			{Name: "cpu", UsedPct: 45.2},
			{Name: "cpu0", UsedPct: 50.0},
		},
		Memory: model.MemoryMetrics{
			Total:     8 * 1024 * 1024 * 1024,
			Used:      4 * 1024 * 1024 * 1024,
			Available: 4 * 1024 * 1024 * 1024,
		},
		Disk: model.DiskMetrics{
			Partitions: []model.DiskPartition{
				{Filesystem: "/dev/sda1", MountPoint: "/", Total: 100e9, Used: 60e9, Available: 40e9},
			},
		},
		Network: []model.NetworkRate{
			{Name: "eth0", RxBytesPS: 1024, TxBytesPS: 512},
		},
		Process: model.ProcessMetrics{
			Processes: []model.Process{
				{PID: 1, User: "root", CPUPct: 1.5, MemPct: 0.3, Command: "systemd"},
			},
		},
		System: model.SystemInfo{
			Hostname:  "web-1",
			Kernel:    "6.1.0",
			UptimeSec: 86400,
			OSName:    "Debian",
			CoreCount: 4,
		},
	}

	snapshots := map[string]*model.Snapshot{"srv1": snap}

	data, err := Assemble(servers, snapshots)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	var got wireSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(got.Servers))
	}
	s := got.Servers[0]
	if s.ID != "srv1" || s.Name != "web-1" || s.Host != "10.0.0.1" || s.Status != "connected" {
		t.Errorf("server info mismatch: %+v", s)
	}

	m, ok := got.Metrics["srv1"]
	if !ok {
		t.Fatal("missing metrics for srv1")
	}

	if m.CPU.Aggregate.UsagePercent != 45.2 {
		t.Errorf("cpu aggregate: got %v, want 45.2", m.CPU.Aggregate.UsagePercent)
	}
	if len(m.CPU.Cores) != 1 || m.CPU.Cores[0].UsagePercent != 50.0 {
		t.Errorf("cpu cores: %+v", m.CPU.Cores)
	}
	if m.Memory.Total != 8*1024*1024*1024 {
		t.Errorf("memory total: got %d", m.Memory.Total)
	}
	if len(m.Disk.Partitions) != 1 || m.Disk.Partitions[0].MountPoint != "/" {
		t.Errorf("disk: %+v", m.Disk.Partitions)
	}
	if len(m.Network.Interfaces) != 1 || m.Network.Interfaces[0].RxBytesPerSec != 1024 {
		t.Errorf("network: %+v", m.Network.Interfaces)
	}
	if len(m.Process.Processes) != 1 || m.Process.Processes[0].PID != 1 {
		t.Errorf("processes: %+v", m.Process.Processes)
	}
	if m.System.Hostname != "web-1" || m.System.CoreCount != 4 {
		t.Errorf("system: %+v", m.System)
	}
}

func TestAssemble_NoSnapshot(t *testing.T) {
	servers := []ServerInfo{
		{ID: "srv1", Name: "web-1", Host: "10.0.0.1", State: "reconnecting", Attempts: 3, LastError: "timeout"},
	}

	data, err := Assemble(servers, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	var got wireSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(got.Servers))
	}
	s := got.Servers[0]
	if s.Status != "reconnecting" || s.Attempts != 3 || s.LastError != "timeout" {
		t.Errorf("server status mismatch: %+v", s)
	}
	if len(got.Metrics) != 0 {
		t.Errorf("expected empty metrics, got %d entries", len(got.Metrics))
	}
}

func TestAssemble_EmptyServers(t *testing.T) {
	data, err := Assemble(nil, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	var got wireSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Servers == nil {
		t.Error("servers should be empty slice, not nil")
	}
	if len(got.Servers) != 0 {
		t.Errorf("servers: got %d, want 0", len(got.Servers))
	}
}
