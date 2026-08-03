package model

import (
	"errors"
	"strings"
	"time"
)

// ErrServerNotFound is returned by the store when a server does not exist.
// API handlers detect it via errors.Is to map to an HTTP 404 response.
var ErrServerNotFound = errors.New("server not found")

// ErrGroupNotFound is returned by the store when a group does not exist.
var ErrGroupNotFound = errors.New("group not found")

// ErrGroupDuplicateName is returned when creating/renaming a group with a name
// that is already in use by another group.
var ErrGroupDuplicateName = errors.New("group name already exists")

// AuthType represents the SSH authentication method for a server.
type AuthType string

const (
	AuthTypePassword AuthType = "password"
	AuthTypeKey      AuthType = "key"
)

// ProtectedCommands is the set of process names that cannot be killed.
var ProtectedCommands = map[string]bool{
	"systemd": true,
	"init":    true,
	"sshd":    true,
}

// IsProtectedCommand reports whether a process name is protected from kill.
func IsProtectedCommand(comm string) bool {
	return ProtectedCommands[comm]
}

// Group represents a named root-level container that organizes servers.
// Groups are flat: a Server belongs to at most one Group, and Groups cannot
// be nested.
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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
	GroupID            *string   `json:"group_id,omitempty"`
	HostKeyFingerprint string    `json:"host_key_fingerprint,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Validate checks the server configuration and returns a list of human-readable
// problems. An empty slice means the server is valid.
func (s Server) Validate() []string {
	var problems []string
	if trimmed := strings.TrimSpace(s.Name); trimmed == "" {
		problems = append(problems, "name is required")
	}
	if trimmed := strings.TrimSpace(s.Host); trimmed == "" {
		problems = append(problems, "host is required")
	}
	if trimmed := strings.TrimSpace(s.Username); trimmed == "" {
		problems = append(problems, "username is required")
	}
	if s.Port <= 0 || s.Port > 65535 {
		problems = append(problems, "port must be between 1 and 65535")
	}
	switch s.AuthType {
	case AuthTypePassword, AuthTypeKey:
		// valid
	case "":
		problems = append(problems, "auth_type is required")
	default:
		problems = append(problems, "auth_type must be \"password\" or \"key\"")
	}
	return problems
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
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"-"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`
}

// DiskPartition holds usage for a single mounted partition.
type DiskPartition struct {
	Filesystem string `json:"filesystem"`
	MountPoint string `json:"mountPoint"`
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	Available  uint64 `json:"-"`
}

// DiskMetrics holds a list of disk partitions.
type DiskMetrics struct {
	Partitions []DiskPartition `json:"partitions"`
}

// NetworkInterface holds raw byte counters for a single network interface.
type NetworkInterface struct {
	Name    string
	RxBytes uint64
	TxBytes uint64
}

// NetworkMetrics holds a list of network interfaces.
type NetworkMetrics struct {
	Interfaces []NetworkInterface
}

// Process holds information about a single running process.
type Process struct {
	PID      int       `json:"pid"`
	PPID     int       `json:"ppid"`
	User     string    `json:"user"`
	RSS      int64     `json:"rss"`
	CPUPct   float64   `json:"cpuPct"`
	MemPct   float64   `json:"memPct"`
	ETime    string    `json:"etime"`
	Command     string    `json:"command"`
	CommandName string    `json:"command_name"`
	Protected   bool      `json:"protected"`
}

// SystemInfo holds static system information.
type SystemInfo struct {
	Hostname  string  `json:"hostname"`
	Kernel    string  `json:"kernel"`
	UptimeSec float64 `json:"uptimeSec"`
	OSName    string  `json:"osName"`
	CoreCount int     `json:"coreCount"`
}

// Metrics is the top-level container for all raw metric types from a single collection.
type Metrics struct {
	CPU     CPUMetrics
	Memory  MemoryMetrics
	Disk    DiskMetrics
	Network NetworkMetrics
	System  SystemInfo
}

// CPUUsage holds computed CPU usage percentages for one core (or aggregate).
type CPUUsage struct {
	Name    string  `json:"name,omitempty"`
	UsedPct float64 `json:"usagePercent"`
}

// NetworkRate holds computed throughput for one interface.
type NetworkRate struct {
	Name      string  `json:"name"`
	RxBytesPS float64 `json:"rxBytesPerSec"`
	TxBytesPS float64 `json:"txBytesPerSec"`
}

// NetworkRateList wraps network rates for JSON serialization.
type NetworkRateList struct {
	Interfaces []NetworkRate `json:"interfaces"`
}

// Snapshot holds computed metrics ready for broadcasting to clients.
type Snapshot struct {
	ServerID     string          `json:"serverId"`
	AggregateCPU *CPUUsage       `json:"aggregate"`
	Cores        []CPUUsage      `json:"cores"`
	Memory       MemoryMetrics   `json:"memory"`
	Disk         DiskMetrics     `json:"disk"`
	Network      NetworkRateList `json:"network"`
	System       SystemInfo      `json:"system"`
	CollectedAt  time.Time       `json:"collectedAt"`
	Partial      bool            `json:"partial"`
}
