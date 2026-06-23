## What to build

Implement the SQLite persistence layer for server configurations. The store manages a `servers` table with fields for connection details, display name, host key fingerprint, and timestamps. Passwords are encrypted at rest using AES-256-GCM with a key derived from a configurable encryption key (for now, auto-generated and stored alongside the database in `./data/`).

The store opens SQLite with WAL mode enabled and single-writer serialization. It provides CRUD operations: create, get by ID, list all, update, and delete. Password fields are transparently encrypted on write and decrypted on read within the store layer. The auto-generated encryption key triggers a warning via slog at startup.

The data directory defaults to `./data` relative to CWD, created automatically if absent.

## Acceptance criteria

- [ ] SQLite database created at `./data/belochka.db` with WAL mode enabled
- [ ] `servers` table schema with: id (UUID), name, host, port, auth_type (password/key), username, encrypted_password, key_path, host_key_fingerprint, created_at, updated_at
- [ ] AES-256-GCM encryption/decryption for password field
- [ ] Encryption key auto-generated and saved to `./data/encryption.key` if not provided
- [ ] slog warning at startup when using auto-generated co-located key
- [ ] Store interface with Create, GetByID, List, Update, Delete operations
- [ ] In-memory SQLite tests for all CRUD operations
- [ ] Password encryption/decryption round-trip test
- [ ] Data directory auto-created if missing

## Blocked by

- #1 Project Scaffold
