## What to build

Add the ability to edit an existing server's configuration. The edit dialog pre-fills with current values and enforces conditional re-testing: changing connection-related fields (host, port, username, auth type, credentials) resets the validation state and requires a new connection test before saving. Changing only the display name allows direct save without re-testing.

The edit flow reuses the same form layout as the add dialog. When the host changes, the fingerprint confirmation is required again. Empty password on submit means "keep current password."

## Acceptance criteria

- [ ] Edit button/action accessible from server card or server list
- [ ] Dialog pre-fills with current server values (password field empty, placeholder indicates "unchanged")
- [ ] Changing host, port, username, auth type, or credential resets validation → requires re-test
- [ ] Changing only display name → save enabled immediately without re-test
- [ ] Re-test on host change requires new fingerprint confirmation
- [ ] Empty password field on save means keep current password
- [ ] `PUT /api/servers/{id}` called on save
- [ ] Sonner toast on success/failure
- [ ] Updated server reflected in local state immediately

## Blocked by

- #5 Add Server UI
