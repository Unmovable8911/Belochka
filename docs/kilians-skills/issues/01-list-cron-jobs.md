## What to build

Add a Cron Jobs tab to the ServerDetail page, backed by a REST endpoint that reads and parses the remote server's crontab.

**Backend:** Add `GET /api/servers/{id}/crons` that SSHes into the server and runs `crontab -l`. Parse the output into structured entries: `{ minute, hour, dayOfMonth, month, dayOfWeek, command, enabled, raw }`. Lines starting with `#[disabled] ` are parsed as disabled cron entries (strip the marker to recover the schedule). Plain comment lines and environment variable declarations (e.g. `MAILTO=root`, `PATH=...`) are preserved as opaque "passthrough" lines and returned separately — they must survive a round-trip without modification. Return a JSON object: `{ entries: CronEntry[], passthroughs: string[] }`.

**Frontend:** Replace the flat content area in `ServerDetail` with two tabs below the server name + action buttons row: **Overview** (all existing content unchanged) and **Cron Jobs**. The Cron Jobs tab shows a table with columns: enabled status, schedule (`* * * * *`), command, and an actions column (placeholder for future slices). While loading, show a spinner. On error (SSH failure, server offline), show an inline error message rather than a toast. On empty crontab, show an empty state message.

**i18n:** Add all new UI strings to all four translation files (en, zh, fr, ru): tab labels, table column headers, loading/empty/error states.

## Acceptance criteria

- [ ] `GET /api/servers/{id}/crons` returns structured cron entries and passthrough lines
- [ ] Disabled entries (prefixed `#[disabled]`) are parsed correctly with `enabled: false`
- [ ] Plain comments and env var lines are preserved in `passthroughs`, not treated as cron entries
- [ ] ServerDetail shows Overview and Cron Jobs tabs; Overview tab renders existing content identically
- [ ] Cron Jobs tab renders a table of entries with enabled status, schedule, and command columns
- [ ] Loading spinner shown while fetching; inline error shown on failure; empty state shown for empty crontab
- [ ] All new strings present in en, zh, fr, ru translation files

## Blocked by

None — can start immediately.
