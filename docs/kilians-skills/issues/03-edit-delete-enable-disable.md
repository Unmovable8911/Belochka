## What to build

Add edit, delete, and enable/disable operations to the Cron Jobs table.

**Backend:**

- `PUT /api/servers/{id}/crons/{index}` — replaces the cron entry at the given zero-based index (among cron entries only, not passthroughs). Accepts `{ minute, hour, dayOfMonth, month, dayOfWeek, command, enabled }`. If `enabled` is false, writes the line as `#[disabled] <schedule> <command>`; otherwise writes a plain cron line. Reads current crontab, replaces the target entry, writes back. Passthrough lines are preserved.
- `DELETE /api/servers/{id}/crons/{index}` — removes the entry at the given index. Reads, removes, writes back. Passthrough lines are preserved.

The index in both endpoints refers to the position within the `entries` array returned by `GET /api/servers/{id}/crons`, not the raw file line number.

**Frontend:**

- **Edit:** Each table row has an edit (pencil) icon button. Clicking it opens the same dialog used for Add (pre-populated with the entry's current values). Saving calls PUT. On success, refresh the list.
- **Delete:** Each table row has a delete (trash) icon button. Clicking it shows a confirmation dialog ("Delete this cron job?") before calling DELETE. On success, refresh the list.
- **Enable/Disable:** Each table row has a toggle switch in the enabled column. Toggling it calls PUT with `enabled` flipped and all other fields unchanged. No confirmation dialog needed. On failure, revert the toggle and show an inline error in the table row.

All write failures (PUT, DELETE) show an inline error in the table, not a toast.

**i18n:** Add all new strings to en, zh, fr, ru: edit/delete button labels, confirmation dialog text, enable/disable labels, error messages.

## Acceptance criteria

- [ ] `PUT /api/servers/{id}/crons/{index}` correctly replaces the entry and preserves passthrough lines
- [ ] `DELETE /api/servers/{id}/crons/{index}` correctly removes the entry and preserves passthrough lines
- [ ] Disabled entries are written as `#[disabled] <schedule> <command>`; enabled entries as plain cron lines
- [ ] Edit button opens the Add dialog pre-populated; saving updates the entry in the table
- [ ] Delete button shows confirmation dialog before deleting
- [ ] Toggle switch enables/disables inline without a dialog; reverts and shows error on failure
- [ ] All new strings present in en, zh, fr, ru translation files

## Blocked by

- [Add cron job](02-add-cron-job.md)
