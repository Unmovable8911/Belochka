package process

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"belochka/internal/model"
	"belochka/internal/ssh"
)

// ErrProtectedProcess is returned when attempting to kill a protected process.
var ErrProtectedProcess = errors.New("process is protected")

// ErrProcessNotFound is returned when a process PID does not exist.
var ErrProcessNotFound = errors.New("process not found")

// Service provides process management operations on remote servers.
type Service struct {
	executor ssh.Executor
}

// NewService creates a Service backed by the given executor.
func NewService(executor ssh.Executor) *Service {
	return &Service{executor: executor}
}

// List fetches all processes from the remote server using ps -eo.
func (s *Service) List(ctx context.Context, serverID string) ([]model.Process, error) {
	cmd := "ps -eo pid,ppid,user,rss,%cpu,%mem,etime,args --sort=-%cpu --no-headers"
	output, err := s.executor.Execute(ctx, serverID, cmd)
	if err != nil {
		return nil, err
	}
	return ParsePSOutput(output), nil
}

// ParsePSOutput parses the output of ps -eo pid,ppid,user,rss,%cpu,%mem,etime,args --no-headers.
// Returns a flat list of Process structs (no tree — frontend builds children from ppid).
func ParsePSOutput(input string) []model.Process {
	var processes []model.Process
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		p, ok := parsePSLine(line)
		if !ok {
			continue
		}
		processes = append(processes, p)
	}
	return processes
}

// extractCommandName extracts the executable name from a full command path.
// "/usr/sbin/sshd -D" returns "sshd".
func extractCommandName(cmd string) string {
	base := cmd
	if idx := strings.LastIndex(cmd, "/"); idx != -1 {
		base = cmd[idx+1:]
	}
	if idx := strings.Index(base, " "); idx != -1 {
		base = base[:idx]
	}
	if base == "" {
		return cmd
	}
	return base
}

// parsePSLine parses a single line of ps -eo output.
// Format: pid ppid user rss %cpu %mem etime args
// The first 7 fields are single tokens; the 8th (args) may contain spaces.
func parsePSLine(line string) (model.Process, bool) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return model.Process{}, false
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return model.Process{}, false
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return model.Process{}, false
	}

	rss, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return model.Process{}, false
	}

	cpuPct, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return model.Process{}, false
	}

	memPct, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return model.Process{}, false
	}

	// Command is everything from index 7 onward (may contain spaces)
	command := strings.Join(fields[7:], " ")
	commName := extractCommandName(command)

	return model.Process{
		PID:         pid,
		PPID:        ppid,
		User:        fields[2],
		RSS:         rss,
		CPUPct:      cpuPct,
		MemPct:      memPct,
		ETime:       fields[6],
		Command:     command,
		CommandName: commName,
		Protected:   model.IsProtectedCommand(commName),
	}, true
}

// Kill sends a signal to a process on the remote server.
// It checks for protected processes before sending the signal.
func (s *Service) Kill(ctx context.Context, serverID string, pid int, signal string) error {
	if signal == "" {
		signal = "SIGTERM"
	}
	if signal != "SIGTERM" && signal != "SIGKILL" {
		return fmt.Errorf("invalid signal: %s", signal)
	}

	// Check if process is protected
	comm, err := s.executor.Execute(ctx, serverID, fmt.Sprintf("cat /proc/%d/comm 2>/dev/null || echo ''", pid))
	if err != nil {
		return err
	}
	comm = strings.TrimSpace(comm)
	if comm == "" {
		return ErrProcessNotFound
	}
	if model.IsProtectedCommand(comm) {
		return ErrProtectedProcess
	}

	// Send the signal
	killCmd := fmt.Sprintf("kill -%s %d", signal, pid)
	_, err = s.executor.Execute(ctx, serverID, killCmd)
	return err
}

