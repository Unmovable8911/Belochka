package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"belochka/internal/model"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements server persistence using SQLite.
type SQLiteStore struct {
	db  *sql.DB
	key []byte
}

const createServersTable = `
CREATE TABLE IF NOT EXISTS servers (
	id                   TEXT PRIMARY KEY,
	name                 TEXT NOT NULL,
	host                 TEXT NOT NULL,
	port                 INTEGER NOT NULL,
	auth_type            TEXT NOT NULL,
	username             TEXT NOT NULL,
	encrypted_password   TEXT NOT NULL DEFAULT '',
	key_path             TEXT NOT NULL DEFAULT '',
	host_key_fingerprint TEXT NOT NULL DEFAULT '',
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL
);`

// Open creates a new SQLiteStore. It ensures the data directory exists,
// loads or generates an encryption key, opens the database with WAL mode,
// and creates the schema.
//
// encryptionKey may be empty, in which case a key is auto-generated
// and saved to dataDir/encryption.key with an slog warning.
func Open(dataDir string, encryptionKey string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	var key []byte
	if encryptionKey != "" {
		// Use the provided key, padded/truncated to 32 bytes via SHA-256
		k := deriveKey(encryptionKey)
		key = k[:]
	} else {
		keyPath := filepath.Join(dataDir, "encryption.key")
		var generated bool
		var err error
		key, generated, err = loadOrGenerateKey(keyPath)
		if err != nil {
			return nil, fmt.Errorf("load encryption key: %w", err)
		}
		if generated {
			slog.Warn("encryption key auto-generated and stored alongside database; consider providing an explicit key via config or BELOCHKA_ENCRYPTION_KEY env var",
				"path", keyPath)
		}
	}

	dbPath := filepath.Join(dataDir, "belochka.db")
	return newSQLiteStoreWithKey(dbPath, key)
}

// newSQLiteStoreWithKey opens a SQLite database and initializes the schema.
// Used by Open and by tests (with ":memory:").
func newSQLiteStoreWithKey(dbPath string, key []byte) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Single-writer serialization
	db.SetMaxOpenConns(1)

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Create schema
	if _, err := db.Exec(createServersTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &SQLiteStore{db: db, key: key}, nil
}

// Close checkpoints the WAL and closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		slog.Warn("WAL checkpoint failed", "error", err)
	}
	return s.db.Close()
}

// Create inserts a new server and returns it with generated ID and timestamps.
func (s *SQLiteStore) Create(ctx context.Context, srv model.Server) (model.Server, error) {
	srv.ID = uuid.New().String()
	now := time.Now().UTC()
	srv.CreatedAt = now
	srv.UpdatedAt = now

	encPassword, err := s.encryptPassword(srv.Password)
	if err != nil {
		return model.Server{}, fmt.Errorf("encrypt password: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO servers (id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Host, srv.Port, string(srv.AuthType),
		srv.Username, encPassword, srv.KeyPath, srv.HostKeyFingerprint,
		srv.CreatedAt, srv.UpdatedAt,
	)
	if err != nil {
		return model.Server{}, fmt.Errorf("insert server: %w", err)
	}

	// Clear password from returned value (API should never expose it)
	srv.Password = ""
	return srv, nil
}

// GetByID retrieves a server by its UUID. Password is decrypted transparently.
func (s *SQLiteStore) GetByID(ctx context.Context, id string) (model.Server, error) {
	var srv model.Server
	var encPassword string
	var authType string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, created_at, updated_at
		 FROM servers WHERE id = ?`, id,
	).Scan(&srv.ID, &srv.Name, &srv.Host, &srv.Port, &authType,
		&srv.Username, &encPassword, &srv.KeyPath, &srv.HostKeyFingerprint,
		&srv.CreatedAt, &srv.UpdatedAt)

	if err == sql.ErrNoRows {
		return model.Server{}, fmt.Errorf("%w: %s", model.ErrServerNotFound, id)
	}
	if err != nil {
		return model.Server{}, fmt.Errorf("query server: %w", err)
	}

	srv.AuthType = model.AuthType(authType)

	if encPassword != "" {
		pwd, err := decrypt(s.key, encPassword)
		if err != nil {
			return model.Server{}, fmt.Errorf("decrypt password: %w", err)
		}
		srv.Password = pwd
	}

	return srv, nil
}

// List returns all servers ordered by creation time. Passwords are decrypted.
// Prefer ListWithoutPasswords on hot paths where secrets are not needed.
func (s *SQLiteStore) List(ctx context.Context) ([]model.Server, error) {
	return s.list(ctx, true)
}

// ListWithoutPasswords returns all servers without decrypting passwords.
// Use on hot paths (e.g., broadcast loop) where secrets are not required.
func (s *SQLiteStore) ListWithoutPasswords(ctx context.Context) ([]model.Server, error) {
	return s.list(ctx, false)
}

func (s *SQLiteStore) list(ctx context.Context, decryptPasswords bool) ([]model.Server, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, created_at, updated_at
		 FROM servers ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query servers: %w", err)
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		var srv model.Server
		var encPassword string
		var authType string

		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Host, &srv.Port, &authType,
			&srv.Username, &encPassword, &srv.KeyPath, &srv.HostKeyFingerprint,
			&srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}

		srv.AuthType = model.AuthType(authType)

	if decryptPasswords && encPassword != "" {
			pwd, err := decrypt(s.key, encPassword)
			if err != nil {
				return nil, fmt.Errorf("decrypt password: %w", err)
			}
			srv.Password = pwd
		}

		servers = append(servers, srv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}

	return servers, nil
}

// Update modifies an existing server's fields. If Password is empty, the
// existing encrypted password is preserved (allows name-only edits without
// re-supplying the password).
func (s *SQLiteStore) Update(ctx context.Context, srv model.Server) (model.Server, error) {
	srv.UpdatedAt = time.Now().UTC()

	var encPassword string
	if srv.Password != "" {
		var err error
		encPassword, err = s.encryptPassword(srv.Password)
		if err != nil {
			return model.Server{}, fmt.Errorf("encrypt password: %w", err)
		}
	}

	// COALESCE(NULLIF(?, ''), encrypted_password) preserves the existing
	// encrypted password when an empty string is passed (no-op password change).
	result, err := s.db.ExecContext(ctx,
		`UPDATE servers SET name=?, host=?, port=?, auth_type=?, username=?,
		 encrypted_password=COALESCE(NULLIF(?, ''), encrypted_password),
		 key_path=?, host_key_fingerprint=?, updated_at=?
		 WHERE id=?`,
		srv.Name, srv.Host, srv.Port, string(srv.AuthType),
		srv.Username, encPassword, srv.KeyPath, srv.HostKeyFingerprint,
		srv.UpdatedAt, srv.ID,
	)

	if err != nil {
		return model.Server{}, fmt.Errorf("update server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return model.Server{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return model.Server{}, fmt.Errorf("%w: %s", model.ErrServerNotFound, srv.ID)
	}

	// Re-read from DB to get the complete, consistent state
	return s.GetByID(ctx, srv.ID)
}

// Delete removes a server by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", model.ErrServerNotFound, id)
	}

	return nil
}

// CleanupOrphanKeys removes key files in dataDir/keys that are no longer
// referenced by any server's key_path.
func (s *SQLiteStore) CleanupOrphanKeys(dataDir string) error {
	keysDir := filepath.Join(dataDir, "keys")

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read keys dir: %w", err)
	}

	// Build set of referenced key paths.
	rows, err := s.db.Query(`SELECT key_path FROM servers WHERE key_path != ''`)
	if err != nil {
		return fmt.Errorf("query key paths: %w", err)
	}
	defer rows.Close()

	referenced := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return fmt.Errorf("scan key path: %w", err)
		}
		referenced[p] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate key paths: %w", err)
	}

	// Remove unreferenced .key files.
	var removed int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".key" {
			continue
		}
		fullPath := filepath.Join(keysDir, entry.Name())
		if referenced[fullPath] {
			continue
		}
		if err := os.Remove(fullPath); err != nil {
			slog.Warn("failed to remove orphan key file", "path", fullPath, "error", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("cleaned up orphan key files", "count", removed)
	}
	return nil
}

// encryptPassword encrypts a password, returning empty string for empty input.
func (s *SQLiteStore) encryptPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	return encrypt(s.key, password)
}

// deriveKey derives a 32-byte key from a passphrase using SHA-256.
func deriveKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}
