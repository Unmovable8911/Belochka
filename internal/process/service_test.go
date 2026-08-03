package process

import (
	"context"
	"errors"
	"strings"
	"testing"

	"belochka/internal/model"
	"belochka/internal/ssh"
)

// stubExecutor returns fixed output for testing.
type stubExecutor struct {
	outputs []string // sequential outputs for multiple calls
	callIdx int
	err     error
}

var _ ssh.Executor = (*stubExecutor)(nil)

func (s *stubExecutor) Execute(_ context.Context, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.callIdx < len(s.outputs) {
		out := s.outputs[s.callIdx]
		s.callIdx++
		return out, nil
	}
	return "", nil
}

// --- ParsePSOutput tests ---

func TestParsePSOutput_ValidOutput(t *testing.T) {
	input := strings.Join([]string{
		"    1     0 root           12560  0.0  0.1  2-03:45:01 systemd",
		"  500     1 root           45000  5.2  1.3    01:23:45 /usr/bin/myapp --verbose",
		"  501   500 www-data       23000  1.1  0.5    00:12:34 nginx: worker process",
	}, "\n")

	procs := ParsePSOutput(input)
	if len(procs) != 3 {
		t.Fatalf("process count = %d, want 3", len(procs))
	}

	// First process
	p0 := procs[0]
	if p0.PID != 1 {
		t.Errorf("p0 pid = %d, want 1", p0.PID)
	}
	if p0.PPID != 0 {
		t.Errorf("p0 ppid = %d, want 0", p0.PPID)
	}
	if p0.User != "root" {
		t.Errorf("p0 user = %q, want root", p0.User)
	}
	if p0.RSS != 12560 {
		t.Errorf("p0 rss = %d, want 12560", p0.RSS)
	}
	if p0.CPUPct != 0.0 {
		t.Errorf("p0 cpu = %f, want 0.0", p0.CPUPct)
	}
	if p0.MemPct != 0.1 {
		t.Errorf("p0 mem = %f, want 0.1", p0.MemPct)
	}
	if p0.ETime != "2-03:45:01" {
		t.Errorf("p0 etime = %q, want 2-03:45:01", p0.ETime)
	}
	if p0.Command != "systemd" {
		t.Errorf("p0 command = %q, want systemd", p0.Command)
	}

	// Second process — check command with args
	p1 := procs[1]
	if p1.PID != 500 || p1.PPID != 1 {
		t.Errorf("p1 pid/ppid = %d/%d, want 500/1", p1.PID, p1.PPID)
	}
	if p1.Command != "/usr/bin/myapp --verbose" {
		t.Errorf("p1 command = %q, want /usr/bin/myapp --verbose", p1.Command)
	}

	// Third process — spaces in command
	p2 := procs[2]
	if p2.Command != "nginx: worker process" {
		t.Errorf("p2 command = %q, want nginx: worker process", p2.Command)
	}
}

func TestParsePSOutput_EmptyInput(t *testing.T) {
	procs := ParsePSOutput("")
	if len(procs) != 0 {
		t.Errorf("process count = %d, want 0", len(procs))
	}
}

func TestParsePSOutput_WhitespaceOnly(t *testing.T) {
	procs := ParsePSOutput("   \n  \n  ")
	if len(procs) != 0 {
		t.Errorf("process count = %d, want 0", len(procs))
	}
}

func TestParsePSOutput_MalformedLine(t *testing.T) {
	// fewer than 8 fields
	input := "1 0 root 1000 0.0"
	procs := ParsePSOutput(input)
	if len(procs) != 0 {
		t.Errorf("process count = %d, want 0 for malformed line", len(procs))
	}
}

func TestParsePSOutput_InvalidNumbers(t *testing.T) {
	input := "abc xyz user 1000 0.0 0.0 00:00 cmd"
	procs := ParsePSOutput(input)
	if len(procs) != 0 {
		t.Errorf("process count = %d, want 0 for invalid numbers", len(procs))
	}
}

// --- Service tests ---

func TestServiceList_ReturnsProcesses(t *testing.T) {
	psOutput := "1 0 root 1000 0.0 0.1 01:00:00 systemd\n2 0 root 2000 1.5 0.3 00:30:00 sshd"
	exec := &stubExecutor{outputs: []string{psOutput}}
	svc := NewService(exec)

	procs, err := svc.List(context.Background(), "srv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("process count = %d, want 2", len(procs))
	}
	if procs[0].PID != 1 || procs[1].PID != 2 {
		t.Errorf("unexpected PIDs: %d, %d", procs[0].PID, procs[1].PID)
	}
}

func TestServiceList_SSHError(t *testing.T) {
	exec := &stubExecutor{err: errors.New("ssh failed")}
	svc := NewService(exec)

	_, err := svc.List(context.Background(), "srv-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceKill_ValidSignal(t *testing.T) {
	exec := &stubExecutor{outputs: []string{"myapp", ""}}
	svc := NewService(exec)

	err := svc.Kill(context.Background(), "srv-1", 1234, "SIGTERM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceKill_ProtectedProcess(t *testing.T) {
	exec := &stubExecutor{outputs: []string{"sshd"}}
	svc := NewService(exec)

	err := svc.Kill(context.Background(), "srv-1", 5678, "SIGTERM")
	if !errors.Is(err, ErrProtectedProcess) {
		t.Fatalf("expected ErrProtectedProcess, got %v", err)
	}
}

func TestServiceKill_ProcessNotFound(t *testing.T) {
	exec := &stubExecutor{outputs: []string{""}}
	svc := NewService(exec)

	err := svc.Kill(context.Background(), "srv-1", 9999, "SIGTERM")
	if !errors.Is(err, ErrProcessNotFound) {
		t.Fatalf("expected ErrProcessNotFound, got %v", err)
	}
}

func TestServiceKill_InvalidSignal(t *testing.T) {
	exec := &stubExecutor{}
	svc := NewService(exec)

	err := svc.Kill(context.Background(), "srv-1", 1234, "SIGSTOP")
	if err == nil {
		t.Fatal("expected error for invalid signal")
	}
}
func TestIsProtected(t *testing.T) {
	tests := []struct {
		comm     string
		expected bool
	}{
		{"systemd", true},
		{"init", true},
		{"sshd", true},
		{"nginx", false},
		{"myapp", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := model.IsProtectedCommand(tc.comm); got != tc.expected {
			t.Errorf("IsProtectedCommand(%q) = %v, want %v", tc.comm, got, tc.expected)
		}
	}
}
