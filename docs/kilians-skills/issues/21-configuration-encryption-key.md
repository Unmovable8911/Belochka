## What to build

Implement the configuration system: optional `belochka.yaml` file loading, environment variable support for the encryption key, and startup behavior.

Configuration load priority: `--config` CLI flag → `./belochka.yaml` in CWD → built-in defaults. All settings have sensible defaults for zero-configuration startup: port 53136, data_dir `./data`.

The encryption key can be set via the config file (`encryption_key` field) or `BELOCHKA_ENCRYPTION_KEY` environment variable. If neither is set, the key is auto-generated and stored in the data directory. When the auto-generated key is co-located with the database, a slog warning is emitted at startup explaining the security implication.

## Acceptance criteria

- [ ] `belochka.yaml` parsed when present (optional, not required)
- [ ] `--config` CLI flag specifies config file path
- [ ] Load order: CLI flag → CWD file → defaults
- [ ] Config fields: port, data_dir, encryption_key
- [ ] Default port: 53136, default data_dir: `./data`
- [ ] `BELOCHKA_ENCRYPTION_KEY` env var overrides config file value
- [ ] Auto-generated key stored at `{data_dir}/encryption.key`
- [ ] slog warning when auto-generated key is co-located with database
- [ ] Zero-configuration startup works (no config file, no env vars)
- [ ] Config values accessible throughout the application via config package

## Blocked by

- #2 SQLite Store Layer
