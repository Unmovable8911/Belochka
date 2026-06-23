package store

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32) // 256-bit zero key for testing
	plaintext := "my-secret-password"

	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if ciphertext == plaintext {
		t.Fatal("ciphertext should not equal plaintext")
	}

	decrypted, err := decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	plaintext := "same-password"

	ct1, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}

	ct2, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // different key

	ciphertext, err := encrypt(key1, "secret")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = decrypt(key2, ciphertext)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestEncryptEmptyPassword(t *testing.T) {
	key := make([]byte, 32)

	ciphertext, err := encrypt(key, "")
	if err != nil {
		t.Fatalf("encrypt empty string failed: %v", err)
	}

	decrypted, err := decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != "" {
		t.Errorf("decrypted = %q, want empty string", decrypted)
	}
}
