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
func (s *SQLiteStore) List(ctx context.Context) ([]model.Server, error) {
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

		if encPassword != "" {
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

	var result sql.Result
	var err error

	if srv.Password != "" {
		result, err = s.db.ExecContext(ctx,
			`UPDATE servers SET name=?, host=?, port=?, auth_type=?, username=?, encrypted_password=?, key_path=?, host_key_fingerprint=?, updated_at=?
			 WHERE id=?`,
			srv.Name, srv.Host, srv.Port, string(srv.AuthType),
			srv.Username, encPassword, srv.KeyPath, srv.HostKeyFingerprint,
			srv.UpdatedAt, srv.ID,
		)
	} else {
		// Keep existing password
		result, err = s.db.ExecContext(ctx,
			`UPDATE servers SET name=?, host=?, port=?, auth_type=?, username=?, key_path=?, host_key_fingerprint=?, updated_at=?
			 WHERE id=?`,
			srv.Name, srv.Host, srv.Port, string(srv.AuthType),
			srv.Username, srv.KeyPath, srv.HostKeyFingerprint,
			srv.UpdatedAt, srv.ID,
		)
	}

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

// encryptPassword encrypts a password, returning empty string for empty input.
func (s *SQLiteStore) encryptPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	return encrypt(s.key, password)
}

// statFile wraps os.Stat for testability.
var statFile = os.Stat

// deriveKey derives a 32-byte key from a passphrase using SHA-256.
func deriveKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}
