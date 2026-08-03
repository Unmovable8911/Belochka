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

// Cipher encrypts and decrypts opaque strings. A single implementation
// (AESCipher) is provided; callers that need a different cipher can
// implement this interface and inject it via newSQLiteStoreWithKey.
type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertextHex string) (string, error)
}

// AESCipher implements Cipher using AES-256-GCM with a fixed 32-byte key.
type AESCipher struct {
	key []byte
}

// NewAESCipher creates an AESCipher with the given 32-byte key.
func NewAESCipher(key []byte) *AESCipher {
	return &AESCipher{key: key}
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns hex(nonce || ciphertext || authentication-tag).
func (c *AESCipher) Encrypt(plaintext string) (string, error) {
	return encrypt(c.key, plaintext)
}

// Decrypt decrypts a hex-encoded ciphertext produced by Encrypt.
func (c *AESCipher) Decrypt(ciphertextHex string) (string, error) {
	return decrypt(c.key, ciphertextHex)
}

// encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// Returns hex(nonce || ciphertext || 16-byte-authentication-tag) where the
// nonce is 12 random bytes and the tag is appended by GCM Seal.
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
