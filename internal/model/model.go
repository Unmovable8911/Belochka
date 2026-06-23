package model

import (
	"time"
)

// AuthType represents the SSH authentication method for a server.
type AuthType string

const (
	AuthTypePassword AuthType = "password"
	AuthTypeKey      AuthType = "key"
)

// Server represents a monitored remote server's configuration.
type Server struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	AuthType           AuthType  `json:"auth_type"`
	Username           string    `json:"username"`
	Password           string    `json:"password,omitempty"`
	KeyPath            string    `json:"key_path,omitempty"`
	HostKeyFingerprint string    `json:"host_key_fingerprint,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CPUCore holds raw jiffy counters for a single CPU core (or "cpu" for aggregate).
type CPUCore struct {
	Name    string // "cpu", "cpu0", "cpu1", ...
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

// CPUMetrics holds aggregate and per-core CPU jiffy counters.
type CPUMetrics struct {
	Aggregate CPUCore
	Cores     []CPUCore
}

// MemoryMetrics holds memory usage in bytes.
type MemoryMetrics struct {
	Total     uint64
	Used      uint64
	Available uint64
	SwapTotal uint64
	SwapUsed  uint64
}

// DiskPartition holds usage for a single mounted partition.
type DiskPartition struct {
	Filesystem string
	MountPoint string
	Total      uint64
	Used       uint64
	Available  uint64
}

// DiskMetrics holds a list of disk partitions.
type DiskMetrics struct {
	Partitions []DiskPartition
}

// NetworkInterface holds raw byte counters for a single network interface.
type NetworkInterface struct {
	Name     string
	RxBytes  uint64
	TxBytes  uint64
}

// NetworkMetrics holds a list of network interfaces.
type NetworkMetrics struct {
	Interfaces []NetworkInterface
}

// Process holds information about a single running process.
type Process struct {
	PID     int
	User    string
	CPUPct  float64
	MemPct  float64
	Command string
}

// ProcessMetrics holds a list of top processes.
type ProcessMetrics struct {
	Processes []Process
}

// SystemInfo holds static system information.
type SystemInfo struct {
	Hostname  string
	Kernel    string
	UptimeSec float64
	OSName    string
	CoreCount int
}

// Metrics is the top-level container for all raw metric types from a single collection.
type Metrics struct {
	CPU     CPUMetrics
	Memory  MemoryMetrics
	Disk    DiskMetrics
	Network NetworkMetrics
	Process ProcessMetrics
	System  SystemInfo
}

// CPUUsage holds computed CPU usage percentages for one core (or aggregate).
type CPUUsage struct {
	Name       string  `json:"name"`
	UsedPct    float64 `json:"used_pct"`    // user + nice + system + irq + softirq + steal
	UserPct    float64 `json:"user_pct"`
	SystemPct  float64 `json:"system_pct"`
	IOWaitPct  float64 `json:"iowait_pct"`
	StealPct   float64 `json:"steal_pct"`
}

// NetworkRate holds computed throughput for one interface.
type NetworkRate struct {
	Name      string  `json:"name"`
	RxBytesPS float64 `json:"rx_bytes_ps"` // receive bytes per second
	TxBytesPS float64 `json:"tx_bytes_ps"` // transmit bytes per second
}

// Snapshot holds computed metrics ready for broadcasting to clients.
type Snapshot struct {
	ServerID   string         `json:"server_id"`
	CPU        []CPUUsage     `json:"cpu"`         // aggregate first, then per-core
	Memory     MemoryMetrics  `json:"memory"`
	Disk       DiskMetrics    `json:"disk"`
	Network    []NetworkRate  `json:"network"`
	Process    ProcessMetrics `json:"process"`
	System     SystemInfo     `json:"system"`
	CollectedAt time.Time    `json:"collected_at"`
	Partial    bool           `json:"partial"` // true on first cycle (no rates available)
}
