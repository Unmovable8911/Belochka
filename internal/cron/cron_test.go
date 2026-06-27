package cron

import (
	"testing"
)

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
