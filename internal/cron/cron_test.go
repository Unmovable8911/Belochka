package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubExecutor returns a fixed crontab output for List.
type stubExecutor struct {
	output string
	err    error
}

func (s *stubExecutor) Execute(_ context.Context, _, _ string) (string, error) {
	return s.output, s.err
}

// stubRunner records the command it was asked to run and returns a fixed result.
type stubRunner struct {
	gotCmd   string
	output   string
	exitCode int
	err      error
}

func (r *stubRunner) RunCommand(_ context.Context, _, cmd string) (string, int, error) {
	r.gotCmd = cmd
	return r.output, r.exitCode, r.err
}

func TestServiceRun_ValidIndex_RunsEntryCommand(t *testing.T) {
	exec := &stubExecutor{output: "0 * * * * /usr/bin/hourly.sh\n"}
	runner := &stubRunner{output: "done\n", exitCode: 0}
	svc := NewService(exec, runner)

	output, exitCode, err := svc.Run(context.Background(), "srv-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.gotCmd != "/usr/bin/hourly.sh" {
		t.Errorf("expected command /usr/bin/hourly.sh, got %q", runner.gotCmd)
	}
	if output != "done\n" || exitCode != 0 {
		t.Errorf("unexpected result: output=%q exitCode=%d", output, exitCode)
	}
}

func TestServiceRun_OutOfRange_ReturnsErr(t *testing.T) {
	exec := &stubExecutor{output: "0 * * * * /usr/bin/hourly.sh\n"}
	runner := &stubRunner{}
	svc := NewService(exec, runner)

	_, _, err := svc.Run(context.Background(), "srv-1", 5)
	if !errors.Is(err, ErrCronIndexOutOfRange) {
		t.Fatalf("expected ErrCronIndexOutOfRange, got %v", err)
	}
	if runner.gotCmd != "" {
		t.Errorf("runner should not be called for out-of-range index")
	}
}

func TestServiceRun_ReadError_Propagates(t *testing.T) {
	exec := &stubExecutor{err: errors.New("ssh failed")}
	runner := &stubRunner{}
	svc := NewService(exec, runner)

	_, _, err := svc.Run(context.Background(), "srv-1", 0)
	if err == nil {
		t.Fatal("expected error from read failure")
	}
}

func TestParseCrontab_EnabledEntry(t *testing.T) {
	output := "0 * * * * /usr/bin/backup.sh"
	result := ParseCrontab(output)

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	e := result.Entries[0]
	if !e.Enabled {
		t.Error("expected enabled=true")
	}
	if e.Minute != "0" || e.Hour != "*" || e.DayOfMonth != "*" || e.Month != "*" || e.DayOfWeek != "*" {
		t.Errorf("wrong schedule fields: %q %q %q %q %q", e.Minute, e.Hour, e.DayOfMonth, e.Month, e.DayOfWeek)
	}
	if e.Command != "/usr/bin/backup.sh" {
		t.Errorf("expected command /usr/bin/backup.sh, got %q", e.Command)
	}
	if e.Raw != "0 * * * * /usr/bin/backup.sh" {
		t.Errorf("expected raw to be original line, got %q", e.Raw)
	}
	if len(result.Passthroughs) != 0 {
		t.Errorf("expected 0 passthroughs, got %d", len(result.Passthroughs))
	}
}

func TestParseCrontab_DisabledEntry(t *testing.T) {
	output := "#[disabled] 30 2 * * 0 /usr/bin/weekly.sh arg1"
	result := ParseCrontab(output)

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	e := result.Entries[0]
	if e.Enabled {
		t.Error("expected enabled=false for disabled entry")
	}
	if e.Minute != "30" || e.Hour != "2" || e.DayOfMonth != "*" || e.Month != "*" || e.DayOfWeek != "0" {
		t.Errorf("wrong schedule fields: %q %q %q %q %q", e.Minute, e.Hour, e.DayOfMonth, e.Month, e.DayOfWeek)
	}
	if e.Command != "/usr/bin/weekly.sh arg1" {
		t.Errorf("expected command with args, got %q", e.Command)
	}
	if e.Raw != "#[disabled] 30 2 * * 0 /usr/bin/weekly.sh arg1" {
		t.Errorf("raw should be the original line including disabled marker, got %q", e.Raw)
	}
	if len(result.Passthroughs) != 0 {
		t.Errorf("expected 0 passthroughs, got %d", len(result.Passthroughs))
	}
}

func TestParseCrontab_PlainCommentIsPassthrough(t *testing.T) {
	output := "# this is a comment\n0 * * * * /usr/bin/job.sh"
	result := ParseCrontab(output)

	if len(result.Passthroughs) != 1 {
		t.Fatalf("expected 1 passthrough, got %d", len(result.Passthroughs))
	}
	if result.Passthroughs[0] != "# this is a comment" {
		t.Errorf("expected comment passthrough, got %q", result.Passthroughs[0])
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
}

func TestParseCrontab_EnvVarIsPassthrough(t *testing.T) {
	output := "MAILTO=root\nPATH=/usr/bin:/bin\n0 * * * * /usr/bin/job.sh"
	result := ParseCrontab(output)

	if len(result.Passthroughs) != 2 {
		t.Fatalf("expected 2 passthroughs, got %d: %v", len(result.Passthroughs), result.Passthroughs)
	}
	if result.Passthroughs[0] != "MAILTO=root" {
		t.Errorf("expected MAILTO=root passthrough, got %q", result.Passthroughs[0])
	}
	if result.Passthroughs[1] != "PATH=/usr/bin:/bin" {
		t.Errorf("expected PATH passthrough, got %q", result.Passthroughs[1])
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
}

func TestParseCrontab_EmptyOutputReturnsEmptyResult(t *testing.T) {
	result := ParseCrontab("")

	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Entries))
	}
	if len(result.Passthroughs) != 0 {
		t.Errorf("expected 0 passthroughs, got %d", len(result.Passthroughs))
	}
}

func TestBuildLine_Enabled(t *testing.T) {
	entry := CronEntry{
		Minute: "0", Hour: "2", DayOfMonth: "*", Month: "*", DayOfWeek: "1",
		Command: "/usr/bin/weekly.sh", Enabled: true,
	}
	got := BuildLine(entry)
	want := "0 2 * * 1 /usr/bin/weekly.sh"
	if got != want {
		t.Errorf("BuildLine(enabled) = %q, want %q", got, want)
	}
}

func TestBuildLine_Disabled(t *testing.T) {
	entry := CronEntry{
		Minute: "*/5", Hour: "*", DayOfMonth: "*", Month: "*", DayOfWeek: "*",
		Command: "/usr/bin/check.sh", Enabled: false,
	}
	got := BuildLine(entry)
	want := "#[disabled] */5 * * * * /usr/bin/check.sh"
	if got != want {
		t.Errorf("BuildLine(disabled) = %q, want %q", got, want)
	}
}

func TestReplaceCronEntry_ReplaceEnabled(t *testing.T) {
	original := "MAILTO=root\n0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"

	updated := CronEntry{
		Minute: "15", Hour: "3", DayOfMonth: "*", Month: "*", DayOfWeek: "0",
		Command: "/usr/bin/daily.sh", Enabled: true,
	}
	got := ReplaceCronEntry(original, 1, &updated)

	if !strings.Contains(got, "15 3 * * 0 /usr/bin/daily.sh") {
		t.Errorf("missing updated entry in output: %q", got)
	}
	if strings.Contains(got, "30 2 * * 0 /usr/bin/weekly.sh") {
		t.Errorf("old entry should be gone: %q", got)
	}
	if !strings.Contains(got, "0 * * * * /usr/bin/hourly.sh") {
		t.Errorf("other entry should be preserved: %q", got)
	}
	if !strings.Contains(got, "MAILTO=root") {
		t.Errorf("passthrough should be preserved: %q", got)
	}
}

func TestReplaceCronEntry_ReplaceWithDisabled(t *testing.T) {
	original := "0 * * * * /usr/bin/hourly.sh\n"

	updated := CronEntry{
		Minute: "0", Hour: "*", DayOfMonth: "*", Month: "*", DayOfWeek: "*",
		Command: "/usr/bin/hourly.sh", Enabled: false,
	}
	got := ReplaceCronEntry(original, 0, &updated)

	if !strings.Contains(got, "#[disabled] 0 * * * * /usr/bin/hourly.sh") {
		t.Errorf("expected disabled line, got: %q", got)
	}
}

func TestReplaceCronEntry_Delete(t *testing.T) {
	original := "MAILTO=root\n# comment\n0 * * * * /usr/bin/hourly.sh\n30 2 * * 0 /usr/bin/weekly.sh\n"

	got := ReplaceCronEntry(original, 0, nil)

	if strings.Contains(got, "hourly.sh") {
		t.Errorf("deleted entry should be gone: %q", got)
	}
	if !strings.Contains(got, "30 2 * * 0 /usr/bin/weekly.sh") {
		t.Errorf("remaining entry should be preserved: %q", got)
	}
	if !strings.Contains(got, "MAILTO=root") {
		t.Errorf("passthrough should be preserved: %q", got)
	}
	if !strings.Contains(got, "# comment") {
		t.Errorf("comment passthrough should be preserved: %q", got)
	}
}

func TestReplaceCronEntry_PreservesPassthroughOrder(t *testing.T) {
	original := "# header\nMAILTO=root\n0 * * * * /usr/bin/hourly.sh\n# footer\n30 2 * * 0 /usr/bin/weekly.sh\n"

	got := ReplaceCronEntry(original, 0, nil)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// After deleting index 0: header, MAILTO, footer, weekly
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "# header" {
		t.Errorf("line 0 wrong: %q", lines[0])
	}
	if lines[1] != "MAILTO=root" {
		t.Errorf("line 1 wrong: %q", lines[1])
	}
	if lines[2] != "# footer" {
		t.Errorf("line 2 wrong: %q", lines[2])
	}
	if lines[3] != "30 2 * * 0 /usr/bin/weekly.sh" {
		t.Errorf("line 3 wrong: %q", lines[3])
	}
}

func TestBuildCronLine(t *testing.T) {
	entry := CronEntry{
		Minute:     "0",
		Hour:       "2",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "1",
		Command:    "/usr/bin/weekly.sh arg1 arg2",
	}
	got := BuildCronLine(entry)
	want := "0 2 * * 1 /usr/bin/weekly.sh arg1 arg2"
	if got != want {
		t.Errorf("BuildCronLine() = %q, want %q", got, want)
	}
}

func TestParseCrontab_MixedCrontab(t *testing.T) {
	output := `# cron config
MAILTO=admin
0 * * * * /usr/bin/hourly.sh
#[disabled] */5 * * * * /usr/bin/check.sh
# another comment
30 2 * * 0 /usr/bin/weekly.sh`

	result := ParseCrontab(output)

	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	if len(result.Passthroughs) != 3 {
		t.Fatalf("expected 3 passthroughs, got %d: %v", len(result.Passthroughs), result.Passthroughs)
	}

	// Check enabled entries
	if !result.Entries[0].Enabled || result.Entries[0].Command != "/usr/bin/hourly.sh" {
		t.Errorf("first entry wrong: %+v", result.Entries[0])
	}
	if result.Entries[1].Enabled || result.Entries[1].Command != "/usr/bin/check.sh" {
		t.Errorf("second entry should be disabled: %+v", result.Entries[1])
	}
	if result.Entries[1].Minute != "*/5" {
		t.Errorf("expected minute */5, got %q", result.Entries[1].Minute)
	}
	if !result.Entries[2].Enabled || result.Entries[2].Command != "/usr/bin/weekly.sh" {
		t.Errorf("third entry wrong: %+v", result.Entries[2])
	}

	// Check passthroughs preserve order
	if result.Passthroughs[0] != "# cron config" {
		t.Errorf("first passthrough wrong: %q", result.Passthroughs[0])
	}
	if result.Passthroughs[1] != "MAILTO=admin" {
		t.Errorf("second passthrough wrong: %q", result.Passthroughs[1])
	}
	if result.Passthroughs[2] != "# another comment" {
		t.Errorf("third passthrough wrong: %q", result.Passthroughs[2])
	}
}
