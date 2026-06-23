package broadcast

import "belochka/internal/model"

type wireServerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

type wireSnapshot struct {
	Servers []wireServerInfo       `json:"servers"`
	Metrics map[string]wireMetrics `json:"metrics"`
}

type wireMetrics struct {
	CPU     wireCPU     `json:"cpu"`
	Memory  wireMemory  `json:"memory"`
	Disk    wireDisk    `json:"disk"`
	Network wireNetwork `json:"network"`
	Process wireProcess `json:"process"`
	System  wireSystem  `json:"system"`
}

type wireCPUCore struct {
	Name         string  `json:"name,omitempty"`
	UsagePercent float64 `json:"usagePercent"`
}

type wireCPU struct {
	Aggregate wireCPUCore   `json:"aggregate"`
	Cores     []wireCPUCore `json:"cores"`
}

type wireMemory struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`
}

type wireDiskPartition struct {
	Filesystem string `json:"filesystem"`
	MountPoint string `json:"mountPoint"`
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	Available  uint64 `json:"available"`
}

type wireDisk struct {
	Partitions []wireDiskPartition `json:"partitions"`
}

type wireNetIface struct {
	Name          string  `json:"name"`
	RxBytesPerSec float64 `json:"rxBytesPerSec"`
	TxBytesPerSec float64 `json:"txBytesPerSec"`
}

type wireNetwork struct {
	Interfaces []wireNetIface `json:"interfaces"`
}

type wireProcessEntry struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPUPct  float64 `json:"cpuPct"`
	MemPct  float64 `json:"memPct"`
	Command string  `json:"command"`
}

type wireProcess struct {
	Processes []wireProcessEntry `json:"processes"`
}

type wireSystem struct {
	Hostname  string  `json:"hostname"`
	Kernel    string  `json:"kernel"`
	UptimeSec float64 `json:"uptimeSec"`
	OSName    string  `json:"osName"`
	CoreCount int     `json:"coreCount"`
}

func snapshotToWire(snap model.Snapshot) wireMetrics {
	cpu := wireCPU{Cores: []wireCPUCore{}}
	if len(snap.CPU) > 0 {
		cpu.Aggregate = wireCPUCore{
			Name:         snap.CPU[0].Name,
			UsagePercent: snap.CPU[0].UsedPct,
		}
		for _, c := range snap.CPU[1:] {
			cpu.Cores = append(cpu.Cores, wireCPUCore{
				Name:         c.Name,
				UsagePercent: c.UsedPct,
			})
		}
	}

	mem := wireMemory{
		Total:     snap.Memory.Total,
		Used:      snap.Memory.Used,
		Available: snap.Memory.Available,
		SwapTotal: snap.Memory.SwapTotal,
		SwapUsed:  snap.Memory.SwapUsed,
	}

	parts := make([]wireDiskPartition, len(snap.Disk.Partitions))
	for i, p := range snap.Disk.Partitions {
		parts[i] = wireDiskPartition{
			Filesystem: p.Filesystem,
			MountPoint: p.MountPoint,
			Total:      p.Total,
			Used:       p.Used,
			Available:  p.Available,
		}
	}

	ifaces := make([]wireNetIface, len(snap.Network))
	for i, n := range snap.Network {
		ifaces[i] = wireNetIface{
			Name:          n.Name,
			RxBytesPerSec: n.RxBytesPS,
			TxBytesPerSec: n.TxBytesPS,
		}
	}

	procs := make([]wireProcessEntry, len(snap.Process.Processes))
	for i, p := range snap.Process.Processes {
		procs[i] = wireProcessEntry{
			PID:     p.PID,
			User:    p.User,
			CPUPct:  p.CPUPct,
			MemPct:  p.MemPct,
			Command: p.Command,
		}
	}

	return wireMetrics{
		CPU:     cpu,
		Memory:  mem,
		Disk:    wireDisk{Partitions: parts},
		Network: wireNetwork{Interfaces: ifaces},
		Process: wireProcess{Processes: procs},
		System: wireSystem{
			Hostname:  snap.System.Hostname,
			Kernel:    snap.System.Kernel,
			UptimeSec: snap.System.UptimeSec,
			OSName:    snap.System.OSName,
			CoreCount: snap.System.CoreCount,
		},
	}
}
