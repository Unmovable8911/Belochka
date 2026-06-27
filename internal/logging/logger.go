package logging

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultRetentionDays = 3

// Logger writes to a log file. Optionally tees output to a second writer (stdout).
// Satisfies io.Writer for use with slog.NewTextHandler.
type Logger struct {
	file      *os.File
	out       io.Writer
	retention time.Duration
}

// New opens (or creates) the log file at path.
// If tee is true, writes also go to os.Stdout.
func New(path string, tee bool) (*Logger, error) {
	return open(path, tee, os.Stdout)
}

// open is the internal constructor; out receives the secondary writer for tee mode.
func open(path string, tee bool, stdout io.Writer) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	var out io.Writer = f
	if tee && stdout != nil {
		out = io.MultiWriter(f, stdout)
	}
	return &Logger{file: f, out: out, retention: retentionFromEnv()}, nil
}

func (l *Logger) Write(p []byte) (int, error) {
	if err := l.purgeIfStale(); err != nil {
		return 0, err
	}
	return l.out.Write(p)
}

func (l *Logger) Close() error {
	return l.file.Close()
}

// purgeIfStale checks the first line of the log file. If its timestamp is older
// than the retention window, it rewrites the file keeping only recent lines.
// The common case (first line within window) is a single seek + read.
func (l *Logger) purgeIfStale() error {
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	scanner := bufio.NewScanner(l.file)
	if !scanner.Scan() {
		return nil // empty file — O_APPEND handles positioning for the write
	}
	firstLine := scanner.Text()
	t, err := parseLogTime(firstLine)
	if err != nil {
		return nil // unparseable first line; leave file as-is
	}
	cutoff := time.Now().Add(-l.retention)
	if !t.Before(cutoff) {
		return nil // first line is within window; nothing to purge
	}

	// Collect lines within the retention window.
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var keep []string
	scanner = bufio.NewScanner(l.file)
	for scanner.Scan() {
		line := scanner.Text()
		t, err := parseLogTime(line)
		if err != nil || !t.Before(cutoff) {
			keep = append(keep, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if err := l.file.Truncate(0); err != nil {
		return err
	}
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	for _, line := range keep {
		if _, err := fmt.Fprintln(l.file, line); err != nil {
			return err
		}
	}
	return nil
}

// parseLogTime extracts the timestamp from a slog text-format line.
// Format: time=<RFC3339Nano> level=... msg=...
func parseLogTime(line string) (time.Time, error) {
	const prefix = "time="
	idx := strings.Index(line, prefix)
	if idx < 0 {
		return time.Time{}, fmt.Errorf("no time= field")
	}
	rest := line[idx+len(prefix):]
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		end = len(rest)
	}
	return time.Parse(time.RFC3339Nano, rest[:end])
}

func retentionFromEnv() time.Duration {
	s := os.Getenv("BELOCHKA_LOG_RETENTION_DAYS")
	if s == "" {
		return defaultRetentionDays * 24 * time.Hour
	}
	days, err := strconv.Atoi(s)
	if err != nil || days <= 0 {
		return defaultRetentionDays * 24 * time.Hour
	}
	return time.Duration(days) * 24 * time.Hour
}
