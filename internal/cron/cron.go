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
