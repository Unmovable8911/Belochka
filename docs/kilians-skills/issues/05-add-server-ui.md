## What to build

Build the "Add Server" dialog in the React frontend that walks the user through the full server creation flow: fill form, test connection, confirm host key fingerprint, save.

The dialog contains fields for: display name, host, port (default 22), username, auth type selector (password or key file path), and the credential field. A "Test Connection" button triggers the test endpoint. On success, the host key fingerprint is displayed and the user must explicitly confirm trust before the "Save" button becomes enabled.

The flow enforces the lifecycle: form → test → confirm fingerprint → save. The test must pass before saving is allowed.

## Acceptance criteria

- [ ] Modal/dialog opens from a button in the dashboard header area
- [ ] Form fields: name, host, port (default 22), username, auth type (password/key), credential
- [ ] Auth type selector toggles between password input and key file path input
- [ ] "Test Connection" button calls `POST /api/servers/{id}/test`
- [ ] Loading state shown during connection test
- [ ] On test success: host key fingerprint displayed with explicit "Trust" confirmation
- [ ] On test failure: error message displayed (auth failure, network, passphrase key, etc.)
- [ ] "Save" button disabled until test passes AND fingerprint is confirmed
- [ ] Successful save closes dialog and adds server to the local state
- [ ] Form validation: non-empty name, host, username required before test
- [ ] Sonner toast notification on save success/failure

## Blocked by

- #4 SSH Connection Testing and Host Key Verification
