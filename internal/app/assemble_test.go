package app

import (
	"encoding/json"
	"testing"

	"belochka/internal/model"
)

func TestAssemble_WithSnapshot(t *testing.T) {
	servers := []serverInfo{
		{
			ID:    "srv1",
			Name:  "web-1",
			Host:  "10.0.0.1",
			State: "connected",
		},
	}

	snap := &model.Snapshot{
		ServerID:     "srv1",
		AggregateCPU: &model.CPUUsage{Name: "cpu", UsedPct: 45.2},
		Cores: []model.CPUUsage{
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
		Network: model.NetworkRateList{
			Interfaces: []model.NetworkRate{
				{Name: "eth0", RxBytesPS: 1024, TxBytesPS: 512},
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

	data, err := assemble(servers, snapshots)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var got broadcastMsg
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(got.Servers))
	}
	s := got.Servers[0]
	if s.ID != "srv1" || s.Name != "web-1" || s.Host != "10.0.0.1" || s.State != "connected" {
		t.Errorf("server info mismatch: %+v", s)
	}

	m, ok := got.Metrics["srv1"]
	if !ok {
		t.Fatal("missing metrics for srv1")
	}

	if m.AggregateCPU == nil || m.AggregateCPU.UsedPct != 45.2 {
		t.Errorf("cpu aggregate: got %v, want 45.2", m.AggregateCPU)
	}
	if len(m.Cores) != 1 || m.Cores[0].UsedPct != 50.0 {
		t.Errorf("cpu cores: %+v", m.Cores)
	}
	if m.Memory.Total != 8*1024*1024*1024 {
		t.Errorf("memory total: got %d", m.Memory.Total)
	}
	if len(m.Disk.Partitions) != 1 || m.Disk.Partitions[0].MountPoint != "/" {
		t.Errorf("disk: %+v", m.Disk.Partitions)
	}
	if len(m.Network.Interfaces) != 1 || m.Network.Interfaces[0].RxBytesPS != 1024 {
		t.Errorf("network: %+v", m.Network.Interfaces)
	}
	if m.System.Hostname != "web-1" || m.System.CoreCount != 4 {
		t.Errorf("system: %+v", m.System)
	}
}

func TestAssemble_NoSnapshot(t *testing.T) {
	servers := []serverInfo{
		{ID: "srv1", Name: "web-1", Host: "10.0.0.1", State: "reconnecting", Attempts: 3, LastError: "timeout"},
	}

	data, err := assemble(servers, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var got broadcastMsg
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(got.Servers))
	}
	s := got.Servers[0]
	if s.State != "reconnecting" || s.Attempts != 3 || s.LastError != "timeout" {
		t.Errorf("server status mismatch: %+v", s)
	}
	if len(got.Metrics) != 0 {
		t.Errorf("expected empty metrics, got %d entries", len(got.Metrics))
	}
}

func TestAssemble_GroupID(t *testing.T) {
	groupID := "grp-abc"
	servers := []serverInfo{
		{ID: "srv1", Name: "web-1", Host: "10.0.0.1", State: "connected", GroupID: &groupID},
	}

	data, err := assemble(servers, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var got broadcastMsg
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(got.Servers))
	}
	s := got.Servers[0]
	if s.GroupID == nil {
		t.Fatal("group_id should not be nil when set")
	}
	if *s.GroupID != "grp-abc" {
		t.Errorf("group_id: got %q, want %q", *s.GroupID, "grp-abc")
	}
}

func TestAssemble_NoGroupID(t *testing.T) {
	servers := []serverInfo{
		{ID: "srv1", Name: "web-1", Host: "10.0.0.1", State: "connected"},
	}

	data, err := assemble(servers, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var got broadcastMsg
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	s := got.Servers[0]
	if s.GroupID != nil {
		t.Errorf("group_id should be omitted when not set, got %q", *s.GroupID)
	}

	// Verify the JSON output doesn't contain "group_id" key when nil
	var raw struct {
		Servers []map[string]interface{} `json:"servers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("raw unmarshal: %v", err)
	}
	if _, ok := raw.Servers[0]["group_id"]; ok {
		t.Error("JSON output should not contain group_id when nil")
	}
}

func TestAssemble_EmptyServers(t *testing.T) {
	data, err := assemble(nil, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	var got broadcastMsg
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
