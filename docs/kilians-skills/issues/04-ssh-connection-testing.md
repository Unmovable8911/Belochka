## What to build

Implement the SSH connection testing endpoint and host key verification using Trust on First Use (TOFU). When a user wants to add or modify a server, they must first test the connection. The test endpoint establishes an SSH connection, returns the host key fingerprint, and reports success or failure.

The SSH module supports two auth methods: password and key file (without passphrase). Keys with passphrases are detected and rejected with a clear error message. The host key fingerprint is returned as part of the test response so the UI can display it for user confirmation before saving.

The server model's `HostKeyFingerprint` field stores the accepted fingerprint. On subsequent tests, the fingerprint is compared — a mismatch produces a specific error.

## Acceptance criteria

- [ ] `POST /api/servers/{id}/test` endpoint that tests SSH connectivity
- [ ] Returns host key fingerprint (SHA256 format) in response on success
- [ ] Password authentication works
- [ ] Key file authentication works (no passphrase)
- [ ] Key file with passphrase rejected with clear error message
- [ ] Host key mismatch detected and reported as specific error
- [ ] Connection timeout handling (5 second dial timeout)
- [ ] Error responses distinguish auth failure, host key mismatch, network error, passphrase-protected key
- [ ] HTTP 422 for connection test failures with descriptive error
- [ ] Tests using mock SSH server for connection state verification

## Blocked by

- #3 Server CRUD REST API
