## What to build

Add the ability to create a new cron job on a remote server via an "Add" button in the Cron Jobs tab.

**Backend:** Add `POST /api/servers/{id}/crons` accepting `{ minute, hour, dayOfMonth, month, dayOfWeek, command }`. Read the current crontab via `crontab -l`, append the new entry as a standard cron line, and write the full crontab back via `crontab -`. Passthrough lines (env vars, plain comments) must be preserved in their original positions. Return the created entry. Return 400 on structurally invalid input (e.g. empty command).

**Frontend:** Add an "Add" button (top-right of the Cron Jobs tab). Clicking it opens a dialog with five text inputs (Minute, Hour, Day of Month, Month, Day of Week) and a Command field. Below the fields, show a real-time human-readable description of the schedule (e.g. "Every 5 minutes", "At 02:00 on Monday"). Each schedule field is validated client-side against basic cron syntax (digits, `*`, `/`, `-`, `,`); the Save button is disabled and fields show a red border when any field is invalid. On success, close the dialog and refresh the cron list. On API error, show an inline error inside the dialog (not a toast).

**i18n:** Add all new strings to en, zh, fr, ru: dialog title, field labels, save/cancel buttons, validation messages, human-readable schedule descriptions.

## Acceptance criteria

- [ ] `POST /api/servers/{id}/crons` appends the new entry and writes the crontab back without losing passthrough lines
- [ ] "Add" button opens a dialog with six fields (five schedule + command)
- [ ] Human-readable schedule preview updates in real time as the user types
- [ ] Invalid cron field syntax disables Save and shows per-field error styling
- [ ] Successful submission closes the dialog and the new entry appears in the table
- [ ] API failure shows an inline error in the dialog
- [ ] All new strings present in en, zh, fr, ru translation files

## Blocked by

- [List cron jobs](01-list-cron-jobs.md)
