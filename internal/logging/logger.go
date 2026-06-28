package logging

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// purgeInterval bounds how often retention cleanup runs, keeping it off the
// per-write hot path. A long-running process is still trimmed periodically.
const purgeInterval = time.Hour

// Logger writes to a log file. Optionally tees output to a second writer (stdout).
// Satisfies io.Writer for use with slog.NewTextHandler.
type Logger struct {
	file      *os.File
	out       io.Writer
	retention time.Duration
	lastPurge time.Time
}

// New opens (or creates) the log file at path.
// If tee is true, writes also go to os.Stdout.
// retention controls how long log lines are kept before being purged.
func New(path string, tee bool, retention time.Duration) (*Logger, error) {
	return open(path, tee, os.Stdout, retention)
}

// open is the internal constructor; stdout receives the secondary writer for tee mode.
func open(path string, tee bool, stdout io.Writer, retention time.Duration) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	var out io.Writer = f
	if tee && stdout != nil {
		out = io.MultiWriter(f, stdout)
	}
	l := &Logger{file: f, out: out, retention: retention}
	// Trim a stale file at startup; subsequent trims happen at most hourly.
	if err := l.purgeIfStale(); err != nil {
		return nil, err
	}
	l.lastPurge = time.Now()
	return l, nil
}

func (l *Logger) Write(p []byte) (int, error) {
	if time.Since(l.lastPurge) >= purgeInterval {
		l.lastPurge = time.Now()
		if err := l.purgeIfStale(); err != nil {
			return 0, err
		}
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
	cutoff := time.Now().Add(-l.retention)

	// Fast path: if the first line parses and is within the window, there is
	// nothing to purge. An unparseable first line falls through to a full scan
	// rather than disabling cleanup. bufio.Reader (unlike Scanner) has no line
	// length cap, so an oversized line cannot wedge the purge.
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReader(l.file)
	firstLine, err := r.ReadString('\n')
	if err != nil && firstLine == "" {
		return nil // empty file — O_APPEND handles positioning for the write
	}
	if t, perr := parseLogTime(firstLine); perr == nil && !t.Before(cutoff) {
		return nil // first line is within window; nothing to purge
	}

	// Collect lines within the retention window (and any unparseable lines).
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var keep []string
	r = bufio.NewReader(l.file)
	for {
		line, rerr := r.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimRight(line, "\n")
			t, perr := parseLogTime(trimmed)
			if perr != nil || !t.Before(cutoff) {
				keep = append(keep, trimmed)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return rerr
		}
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
