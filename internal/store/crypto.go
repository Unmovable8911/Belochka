package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// loadOrGenerateKey loads an encryption key from the given path, or generates
// a new random 32-byte key and saves it if the file doesn't exist.
// Returns the key, whether it was newly generated, and any error.
func loadOrGenerateKey(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		key, err := hex.DecodeString(string(data))
		if err != nil {
			return nil, false, fmt.Errorf("decode key file: %w", err)
		}
		if len(key) != 32 {
			return nil, false, fmt.Errorf("key file has wrong length: %d bytes, want 32", len(key))
		}
		return key, false, nil
	}

	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read key file: %w", err)
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, false, fmt.Errorf("generate key: %w", err)
	}

	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0600); err != nil {
		return nil, false, fmt.Errorf("write key file: %w", err)
	}

	return key, true, nil
}

// encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// Returns hex-encoded ciphertext (nonce + sealed data).
func encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// decrypt decrypts hex-encoded ciphertext using AES-256-GCM with the given 32-byte key.
func decrypt(key []byte, ciphertextHex string) (string, error) {
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
