package cron

import (
	"fmt"
	"strings"
)

// CronEntry represents a single parsed cron job entry.
type CronEntry struct {
	Minute     string `json:"minute"`
	Hour       string `json:"hour"`
	DayOfMonth string `json:"dayOfMonth"`
	Month      string `json:"month"`
	DayOfWeek  string `json:"dayOfWeek"`
	Command    string `json:"command"`
	Enabled    bool   `json:"enabled"`
	Raw        string `json:"raw"`
}

// CronResult is the parsed result of a crontab.
type CronResult struct {
	Entries      []CronEntry `json:"entries"`
	Passthroughs []string    `json:"passthroughs"`
}

const disabledPrefix = "#[disabled] "

// ParseCrontab parses the output of `crontab -l` into structured entries and
// passthrough lines. Disabled entries use the prefix "#[disabled] " and are
// parsed with Enabled: false. Plain comment lines and env var declarations
// (NAME=value) are preserved as passthroughs unchanged.
func ParseCrontab(output string) CronResult {
	result := CronResult{
		Entries:      []CronEntry{},
		Passthroughs: []string{},
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, disabledPrefix) {
			inner := strings.TrimPrefix(line, disabledPrefix)
			if entry, ok := parseCronLine(inner); ok {
				entry.Enabled = false
				entry.Raw = line
				result.Entries = append(result.Entries, entry)
			} else {
				result.Passthroughs = append(result.Passthroughs, line)
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			result.Passthroughs = append(result.Passthroughs, line)
			continue
		}

		if isEnvVar(line) {
			result.Passthroughs = append(result.Passthroughs, line)
			continue
		}

		if entry, ok := parseCronLine(line); ok {
			entry.Enabled = true
			entry.Raw = line
			result.Entries = append(result.Entries, entry)
		}
	}

	return result
}

// parseCronLine parses a 5-field + command cron schedule line.
func parseCronLine(line string) (CronEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return CronEntry{}, false
	}
	return CronEntry{
		Minute:     fields[0],
		Hour:       fields[1],
		DayOfMonth: fields[2],
		Month:      fields[3],
		DayOfWeek:  fields[4],
		Command:    strings.Join(fields[5:], " "),
	}, true
}

// BuildCronLine constructs a standard cron line from the given entry's schedule
// fields and command.
func BuildCronLine(entry CronEntry) string {
	return fmt.Sprintf("%s %s %s %s %s %s",
		entry.Minute, entry.Hour, entry.DayOfMonth, entry.Month, entry.DayOfWeek, entry.Command)
}

// BuildLine constructs a cron line respecting the Enabled flag. Disabled entries
// are written with the "#[disabled] " prefix; enabled entries are plain lines.
func BuildLine(entry CronEntry) string {
	plain := BuildCronLine(entry)
	if !entry.Enabled {
		return disabledPrefix + plain
	}
	return plain
}

// ReplaceCronEntry rebuilds the crontab string, replacing the cron entry at
// entryIndex (zero-based position in the entries array) with newEntry, or
// removing it when newEntry is nil. Passthrough lines are preserved in their
// original positions. Returns the new crontab content.
func ReplaceCronEntry(crontab string, entryIndex int, newEntry *CronEntry) string {
	var out []string
	idx := 0
	for _, line := range strings.Split(crontab, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		if isCronEntry(line) {
			if idx == entryIndex {
				if newEntry != nil {
					out = append(out, BuildLine(*newEntry))
				}
				// nil → delete: skip this line
			} else {
				out = append(out, line)
			}
			idx++
		} else {
			out = append(out, line)
		}
	}

	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// isCronEntry reports whether a raw crontab line is a cron schedule entry
// (either enabled or the "#[disabled] " prefixed form).
func isCronEntry(line string) bool {
	if strings.HasPrefix(line, disabledPrefix) {
		inner := strings.TrimPrefix(line, disabledPrefix)
		_, ok := parseCronLine(inner)
		return ok
	}
	if strings.HasPrefix(line, "#") || isEnvVar(line) {
		return false
	}
	_, ok := parseCronLine(line)
	return ok
}

// isEnvVar reports whether line looks like a shell env var assignment (NAME=value).
func isEnvVar(line string) bool {
	eqIdx := strings.Index(line, "=")
	if eqIdx <= 0 {
		return false
	}
	name := line[:eqIdx]
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
