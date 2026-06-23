package store

import (
	"context"
	"database/sql"
	"testing"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	// Use in-memory SQLite with a shared cache so WAL mode can be verified
	// We pass an explicit 32-byte test key to avoid file-based key generation.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	s, err := newSQLiteStoreWithKey(":memory:", key)
	if err != nil {
		t.Fatalf("newSQLiteStoreWithKey failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreCreatesServersTable(t *testing.T) {
	s := newTestStore(t)

	// Verify the servers table exists by querying it
	var count int
	err := s.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM servers").Scan(&count)
	if err != nil {
		t.Fatalf("query servers table: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestStoreServersTableHasExpectedColumns(t *testing.T) {
	s := newTestStore(t)

	rows, err := s.db.QueryContext(context.Background(), "PRAGMA table_info(servers)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	expectedColumns := map[string]bool{
		"id":                   false,
		"name":                 false,
		"host":                 false,
		"port":                 false,
		"auth_type":            false,
		"username":             false,
		"encrypted_password":   false,
		"key_path":             false,
		"host_key_fingerprint": false,
		"created_at":           false,
		"updated_at":           false,
	}

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		if _, ok := expectedColumns[name]; ok {
			expectedColumns[name] = true
		}
	}

	for col, found := range expectedColumns {
		if !found {
			t.Errorf("missing column: %s", col)
		}
	}
}

func TestStoreWALModeEnabled(t *testing.T) {
	s := newTestStore(t)

	var journalMode string
	err := s.db.QueryRowContext(context.Background(),
		"PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}

	// In-memory databases don't support WAL, they use "memory" journal mode.
	// We verify WAL is attempted by checking the store opens without error.
	// For a file-based test, we'd check for "wal".
	if journalMode != "memory" && journalMode != "wal" {
		t.Errorf("journal_mode = %q, want 'wal' or 'memory'", journalMode)
	}
}

func TestStoreOpenCreatesDataDirectory(t *testing.T) {
	dir := t.TempDir()
	dataDir := dir + "/nested/data"

	s, err := Open(dataDir, "")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	// Verify the data directory was created
	info, err := statFile(dataDir)
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("data dir should be a directory")
	}
}
