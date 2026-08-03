package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"belochka/internal/model"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements server persistence using SQLite.
type SQLiteStore struct {
	db     *sql.DB
	cipher Cipher
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
	group_id             TEXT,
	created_at           DATETIME NOT NULL,
	updated_at           DATETIME NOT NULL,
	FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL
);`

const createGroupsTable = `
CREATE TABLE IF NOT EXISTS groups (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);`

// KeysSubDir is the subdirectory name for SSH key files.
const KeysSubDir = "keys"

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
	return newSQLiteStoreWithKey(dbPath, NewAESCipher(key))
}

// newSQLiteStoreWithKey opens a SQLite database, initializes the schema,
// and returns a store that encrypts/decrypts with the given Cipher.
// Used by Open (with AESCipher) and by tests (with AESCipher or a stub).
func newSQLiteStoreWithKey(dbPath string, cipher Cipher) (*SQLiteStore, error) {
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

	if _, err := db.Exec(createGroupsTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create groups table: %w", err)
	}

	if _, err := db.Exec(createBatchRunsTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create batch runs table: %w", err)
	}

	if _, err := db.Exec(createBatchRunResultsTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create batch run results table: %w", err)
	}

	return &SQLiteStore{db: db, cipher: cipher}, nil
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
		`INSERT INTO servers (id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, group_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Host, srv.Port, string(srv.AuthType),
		srv.Username, encPassword, srv.KeyPath, srv.HostKeyFingerprint, srv.GroupID,
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
		`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, group_id, created_at, updated_at
		 FROM servers WHERE id = ?`, id,
	).Scan(&srv.ID, &srv.Name, &srv.Host, &srv.Port, &authType,
		&srv.Username, &encPassword, &srv.KeyPath, &srv.HostKeyFingerprint, &srv.GroupID,
		&srv.CreatedAt, &srv.UpdatedAt)

	if err == sql.ErrNoRows {
		return model.Server{}, fmt.Errorf("%w: %s", model.ErrServerNotFound, id)
	}
	if err != nil {
		return model.Server{}, fmt.Errorf("query server: %w", err)
	}

	srv.AuthType = model.AuthType(authType)

	if encPassword != "" {
		pwd, err := s.cipher.Decrypt(encPassword)
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
		`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, group_id, created_at, updated_at
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
			&srv.Username, &encPassword, &srv.KeyPath, &srv.HostKeyFingerprint, &srv.GroupID,
			&srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}

		srv.AuthType = model.AuthType(authType)

		if decryptPasswords && encPassword != "" {
			pwd, err := s.cipher.Decrypt(encPassword)
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
		 key_path=?, host_key_fingerprint=?, group_id=?, updated_at=?
		 WHERE id=?`,
		srv.Name, srv.Host, srv.Port, string(srv.AuthType),
		srv.Username, encPassword, srv.KeyPath, srv.HostKeyFingerprint, srv.GroupID,
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

// ListByGroup returns servers filtered by group ID, or all servers when
// groupID is nil. Passwords are decrypted. When groupID is non-nil, servers
// matching that group_id are returned (direct members only, not recursive).
func (s *SQLiteStore) ListByGroup(ctx context.Context, groupID *string) ([]model.Server, error) {
	return s.listByGroup(ctx, groupID, true)
}

// listWithoutPasswordsByGroup is like ListByGroup but without decrypting passwords.
func (s *SQLiteStore) listWithoutPasswordsByGroup(ctx context.Context, groupID *string) ([]model.Server, error) {
	return s.listByGroup(ctx, groupID, false)
}

func (s *SQLiteStore) listByGroup(ctx context.Context, groupID *string, decryptPasswords bool) ([]model.Server, error) {
	var rows *sql.Rows
	var err error

	if groupID == nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, group_id, created_at, updated_at
			 FROM servers ORDER BY created_at ASC`)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, host, port, auth_type, username, encrypted_password, key_path, host_key_fingerprint, group_id, created_at, updated_at
			 FROM servers WHERE group_id = ? ORDER BY created_at ASC`, *groupID)
	}
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
			&srv.Username, &encPassword, &srv.KeyPath, &srv.HostKeyFingerprint, &srv.GroupID,
			&srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}

		srv.AuthType = model.AuthType(authType)

		if decryptPasswords && encPassword != "" {
			pwd, err := s.cipher.Decrypt(encPassword)
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

// --- Group CRUD ---

// CreateGroup inserts a new group and returns it with generated ID and timestamps.
func (s *SQLiteStore) CreateGroup(ctx context.Context, grp model.Group) (model.Group, error) {
	grp.ID = uuid.New().String()
	now := time.Now().UTC()
	grp.CreatedAt = now
	grp.UpdatedAt = now

	// Validate name uniqueness across all groups
	if err := s.checkGroupNameUnique(ctx, grp.Name, ""); err != nil {
		return model.Group{}, err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO groups (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		grp.ID, grp.Name, grp.CreatedAt, grp.UpdatedAt,
	)
	if err != nil {
		return model.Group{}, fmt.Errorf("insert group: %w", err)
	}

	return grp, nil
}

// GetGroupByID retrieves a group by its UUID.
func (s *SQLiteStore) GetGroupByID(ctx context.Context, id string) (model.Group, error) {
	var grp model.Group

	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM groups WHERE id = ?`, id,
	).Scan(&grp.ID, &grp.Name, &grp.CreatedAt, &grp.UpdatedAt)

	if err == sql.ErrNoRows {
		return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupNotFound, id)
	}
	if err != nil {
		return model.Group{}, fmt.Errorf("query group: %w", err)
	}

	return grp, nil
}

// ListGroups returns all groups ordered by creation time.
func (s *SQLiteStore) ListGroups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at, updated_at FROM groups ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query groups: %w", err)
	}
	defer rows.Close()

	var groups []model.Group
	for rows.Next() {
		var grp model.Group
		if err := rows.Scan(&grp.ID, &grp.Name, &grp.CreatedAt, &grp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, grp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}

	return groups, nil
}

// UpdateGroup modifies an existing group's name.
func (s *SQLiteStore) UpdateGroup(ctx context.Context, grp model.Group) (model.Group, error) {
	grp.UpdatedAt = time.Now().UTC()

	// Verify the group exists (preserves created_at for the response).
	existing, err := s.GetGroupByID(ctx, grp.ID)
	if err != nil {
		return model.Group{}, err
	}

	// Validate name uniqueness across all groups (excluding itself)
	if err := s.checkGroupNameUnique(ctx, grp.Name, grp.ID); err != nil {
		return model.Group{}, err
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE groups SET name=?, updated_at=? WHERE id=?`,
		grp.Name, grp.UpdatedAt, grp.ID,
	)
	if err != nil {
		return model.Group{}, fmt.Errorf("update group: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return model.Group{}, fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupNotFound, grp.ID)
	}

	// Preserve created_at from existing record
	grp.CreatedAt = existing.CreatedAt
	return grp, nil
}

// DeleteGroup removes a group by ID. The group's member servers are ungrouped
// (group_id set to NULL); there are no child groups to reassign.
func (s *SQLiteStore) DeleteGroup(ctx context.Context, id string) error {
	// Ungroup the group's member servers
	_, err := s.db.ExecContext(ctx,
		`UPDATE servers SET group_id = NULL WHERE group_id = ?`, id)
	if err != nil {
		return fmt.Errorf("ungroup servers: %w", err)
	}

	// Delete the group
	result, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", model.ErrGroupNotFound, id)
	}

	return nil
}

// GroupMemberCount returns the number of servers assigned to a group.
func (s *SQLiteStore) GroupMemberCount(ctx context.Context, groupID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM servers WHERE group_id = ?`, groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count group members: %w", err)
	}
	return count, nil
}

// checkGroupNameUnique checks that no other group has the given name.
// If excludeID is non-empty, that group is excluded from the check (used
// during rename).
func (s *SQLiteStore) checkGroupNameUnique(ctx context.Context, name string, excludeID string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("group name must not be empty")
	}

	query := `SELECT COUNT(*) FROM groups WHERE name = ?`
	args := []interface{}{name}

	if excludeID != "" {
		query += " AND id != ?"
		args = append(args, excludeID)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return fmt.Errorf("check name uniqueness: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %s", model.ErrGroupDuplicateName, name)
	}
	return nil
}

// CleanupOrphanKeys removes key files in dataDir/keys that are no longer
// referenced by any server's key_path.
func (s *SQLiteStore) CleanupOrphanKeys(dataDir string) error {
	keysDir := filepath.Join(dataDir, KeysSubDir)

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
	return s.cipher.Encrypt(password)
}

// deriveKey derives a 32-byte key from a passphrase using SHA-256.
func deriveKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}
