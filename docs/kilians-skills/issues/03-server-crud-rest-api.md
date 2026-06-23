## What to build

Expose RESTful endpoints for server configuration management. The API layer sits between the HTTP router and the store, handling request validation, response formatting, and password redaction.

Endpoints: `POST /api/servers` (create), `GET /api/servers` (list all), `GET /api/servers/{id}` (get one), `PUT /api/servers/{id}` (update), `DELETE /api/servers/{id}` (delete). All responses use a consistent JSON format. Error responses follow the unified format: `{"error": {"code": "machine_readable", "message": "Human readable"}}`.

Passwords are never included in API responses. On update, an empty password string means "keep current password." Validation requires non-empty host, username, and name fields.

## Acceptance criteria

- [ ] `POST /api/servers` creates a server, returns 201 with server JSON (no password)
- [ ] `GET /api/servers` returns list of all servers (no passwords)
- [ ] `GET /api/servers/{id}` returns single server or 404
- [ ] `PUT /api/servers/{id}` updates server, empty password means keep current
- [ ] `DELETE /api/servers/{id}` removes server or returns 404
- [ ] Validation: 400 for missing required fields (name, host, username)
- [ ] Unified error response format with machine-readable code and human message
- [ ] Password fields never appear in any response body
- [ ] httptest-based tests with mocked store for all endpoints and error cases

## Blocked by

- #2 SQLite Store Layer
