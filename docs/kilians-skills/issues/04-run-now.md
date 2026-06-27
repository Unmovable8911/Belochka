## What to build

Add a "Run now" button to each cron job row that executes the job's command immediately over SSH and displays the full output in a dialog.

**Backend:** Add `POST /api/servers/{id}/crons/{index}/run`. SSH into the server, execute the command from the cron entry at the given index, capture stdout and stderr (merged), and return `{ exitCode: number, output: string }`. The command runs as the SSH user (same as crontab owner). No timeout is imposed by the server — the HTTP response returns when the command exits.

**Frontend:** Each table row has a "Run" (play) icon button. Clicking it:
1. Disables the button and shows a spinner (command is running).
2. On completion, opens a dialog showing the command that was run, the exit code, and the full stdout/stderr output in a monospace scrollable text area.
3. The dialog has a "Close" button. Exit code 0 is shown in green; non-zero in red.
4. On network/SSH failure (before the command even runs), show an inline error in the table row without opening the dialog.

**Testing:** To verify this feature end-to-end, add a test cron entry with a safe, non-destructive command of your own choice (e.g. `echo "belochka-run-test-$(date)"`) to the test server, trigger "Run now", and confirm the output dialog shows the expected output. Remove the test entry afterwards. Test server connection details (host, port, username, password) will be provided at dispatch time.

**i18n:** Add all new strings to en, zh, fr, ru: run button label, dialog title, command label, exit code label, output label, close button, error messages.

## Acceptance criteria

- [ ] `POST /api/servers/{id}/crons/{index}/run` executes the command and returns `{ exitCode, output }`
- [ ] "Run" button shows a spinner while the command is in flight
- [ ] On completion, dialog opens showing command, exit code (green/red), and full output in monospace text area
- [ ] Exit code 0 shown in green; non-zero in red
- [ ] SSH/network failure before execution shows inline error in the table row (no dialog)
- [ ] Manual end-to-end test on the test server passes using a safe command written by the implementing agent
- [ ] All new strings present in en, zh, fr, ru translation files

## Blocked by

- [Edit, delete & enable/disable cron job](03-edit-delete-enable-disable.md)
