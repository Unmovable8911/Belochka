package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateKeyCreatesNewKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "encryption.key")

	key, generated, err := loadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("loadOrGenerateKey failed: %v", err)
	}

	if !generated {
		t.Error("expected generated=true for new key")
	}

	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	// Key file should exist on disk
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("reading key file: %v", err)
	}

	if len(data) == 0 {
		t.Error("key file should not be empty")
	}
}

func TestLoadOrGenerateKeyLoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "encryption.key")

	// Generate key first
	key1, _, err := loadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Load same key
	key2, generated, err := loadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if generated {
		t.Error("expected generated=false for existing key")
	}

	if string(key1) != string(key2) {
		t.Error("loaded key should match generated key")
	}
}

func TestLoadOrGenerateKeyRoundTripWithEncryption(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "encryption.key")

	key, _, err := loadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("loadOrGenerateKey failed: %v", err)
	}

	plaintext := "test-password-123"
	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Reload key from disk and decrypt
	key2, _, err := loadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("reload key failed: %v", err)
	}

	decrypted, err := decrypt(key2, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}
