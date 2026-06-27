package logging

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func slogLine(t time.Time, msg string) string {
	return fmt.Sprintf("time=%s level=INFO msg=%q\n", t.UTC().Format(time.RFC3339Nano), msg)
}

func writeRaw(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l)
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Cycle 1: written line appears in file.
func TestLogger_WritesToFile(t *testing.T) {
	path := t.TempDir() + "/test.log"
	l, err := open(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	line := slogLine(time.Now(), "hello")
	if _, err := l.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}

	if got := readAll(t, path); !strings.Contains(got, "hello") {
		t.Errorf("expected file to contain 'hello', got: %s", got)
	}
}

// Cycle 2: tee mode writes to file AND provided writer.
func TestLogger_TeeMode(t *testing.T) {
	path := t.TempDir() + "/test.log"
	buf := &bytes.Buffer{}
	l, err := open(path, true, buf)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	line := slogLine(time.Now(), "tee-me")
	if _, err := l.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}

	if got := readAll(t, path); !strings.Contains(got, "tee-me") {
		t.Errorf("expected file to contain 'tee-me', got: %s", got)
	}
	if !strings.Contains(buf.String(), "tee-me") {
		t.Errorf("expected stdout buf to contain 'tee-me', got: %s", buf.String())
	}
}

// Cycle 3: lines within retention window are kept.
func TestLogger_RetainsRecentLines(t *testing.T) {
	path := t.TempDir() + "/test.log"
	recent := slogLine(time.Now().Add(-1*24*time.Hour), "recent")
	writeRaw(t, path, []string{recent})

	l, err := open(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	newLine := slogLine(time.Now(), "new")
	if _, err := l.Write([]byte(newLine)); err != nil {
		t.Fatal(err)
	}

	got := readAll(t, path)
	if !strings.Contains(got, "recent") {
		t.Errorf("expected recent line to be kept, got:\n%s", got)
	}
	if !strings.Contains(got, "new") {
		t.Errorf("expected new line to be present, got:\n%s", got)
	}
}

// Cycle 4: lines older than retention window are purged.
func TestLogger_PurgesOldLines(t *testing.T) {
	path := t.TempDir() + "/test.log"
	old := slogLine(time.Now().Add(-4*24*time.Hour), "old-message")
	writeRaw(t, path, []string{old})

	l, err := open(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	newLine := slogLine(time.Now(), "new-message")
	if _, err := l.Write([]byte(newLine)); err != nil {
		t.Fatal(err)
	}

	got := readAll(t, path)
	if strings.Contains(got, "old-message") {
		t.Errorf("expected old line to be purged, got:\n%s", got)
	}
	if !strings.Contains(got, "new-message") {
		t.Errorf("expected new line to be present, got:\n%s", got)
	}
}

// Cycle 5: BELOCHKA_LOG_RETENTION_DAYS overrides default retention.
func TestLogger_RetentionEnvVar(t *testing.T) {
	t.Setenv("BELOCHKA_LOG_RETENTION_DAYS", "1")

	path := t.TempDir() + "/test.log"
	old := slogLine(time.Now().Add(-2*24*time.Hour), "two-days-old")
	writeRaw(t, path, []string{old})

	l, err := open(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	newLine := slogLine(time.Now(), "today")
	if _, err := l.Write([]byte(newLine)); err != nil {
		t.Fatal(err)
	}

	got := readAll(t, path)
	if strings.Contains(got, "two-days-old") {
		t.Errorf("expected 2-day-old line purged with 1-day retention, got:\n%s", got)
	}
	if !strings.Contains(got, "today") {
		t.Errorf("expected new line present, got:\n%s", got)
	}
}

// Cycle 6: an unparseable first line must not disable retention cleanup.
func TestLogger_UnparseableFirstLineStillPurges(t *testing.T) {
	path := t.TempDir() + "/test.log"
	old := slogLine(time.Now().Add(-4*24*time.Hour), "old-message")
	writeRaw(t, path, []string{"garbage without a timestamp\n", old})

	// open performs the initial purge.
	l, err := open(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	got := readAll(t, path)
	if strings.Contains(got, "old-message") {
		t.Errorf("expected old line purged despite unparseable first line, got:\n%s", got)
	}
	if !strings.Contains(got, "garbage without a timestamp") {
		t.Errorf("expected unparseable line to be kept, got:\n%s", got)
	}
}
