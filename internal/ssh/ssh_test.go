package ssh_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"belochka/internal/model"
	"belochka/internal/ssh"

	gossh "golang.org/x/crypto/ssh"
)

// testSSHServer starts a minimal SSH server for testing.
// It returns the listener address and a cleanup function.
func testSSHServer(t *testing.T, opts ...func(*gossh.ServerConfig)) (addr string, hostFingerprint string, cleanup func()) {
	t.Helper()

	// Generate host key
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := gossh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("create host signer: %v", err)
	}

	// Compute expected fingerprint
	pubKey := hostSigner.PublicKey()
	hash := sha256.Sum256(pubKey.Marshal())
	hostFingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])

	config := &gossh.ServerConfig{
		PasswordCallback: func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if conn.User() == "testuser" && string(password) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("auth failed")
		},
	}

	for _, opt := range opts {
		opt(config)
	}

	config.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			go handleTestConnection(conn, config)
		}
	}()

	return listener.Addr().String(), hostFingerprint, func() { listener.Close() }
}

func handleTestConnection(conn net.Conn, config *gossh.ServerConfig) {
	defer conn.Close()
	_, _, _, err := gossh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	// Connection established successfully; keep alive briefly for test to complete
	// The client will close when done
	select {}
}

func TestTestConnection_PasswordAuth_ReturnsFingerprint(t *testing.T) {
	addr, expectedFP, cleanup := testSSHServer(t)
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:     host,
		Port:     port,
		AuthType: model.AuthTypePassword,
		Username: "testuser",
		Password: "testpass",
	}

	result, err := ssh.TestConnection(srv)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if result.Fingerprint != expectedFP {
		t.Fatalf("expected fingerprint %q, got %q", expectedFP, result.Fingerprint)
	}
}

func TestTestConnection_WrongPassword_ReturnsAuthError(t *testing.T) {
	addr, _, cleanup := testSSHServer(t)
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:     host,
		Port:     port,
		AuthType: model.AuthTypePassword,
		Username: "testuser",
		Password: "wrongpass",
	}

	_, err := ssh.TestConnection(srv)
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	var connErr *ssh.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *ssh.ConnectionError, got %T: %v", err, err)
	}
	if connErr.Kind != ssh.ErrAuth {
		t.Fatalf("expected ErrAuth, got %v", connErr.Kind)
	}
}

func TestTestConnection_HostKeyMismatch_ReturnsError(t *testing.T) {
	addr, _, cleanup := testSSHServer(t)
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:               host,
		Port:               port,
		AuthType:           model.AuthTypePassword,
		Username:           "testuser",
		Password:           "testpass",
		HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}

	_, err := ssh.TestConnection(srv)
	if err == nil {
		t.Fatal("expected error for host key mismatch")
	}

	var connErr *ssh.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *ssh.ConnectionError, got %T: %v", err, err)
	}
	if connErr.Kind != ssh.ErrHostKey {
		t.Fatalf("expected ErrHostKey, got %v", connErr.Kind)
	}
}

func TestTestConnection_HostKeyMatch_Succeeds(t *testing.T) {
	addr, expectedFP, cleanup := testSSHServer(t)
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:               host,
		Port:               port,
		AuthType:           model.AuthTypePassword,
		Username:           "testuser",
		Password:           "testpass",
		HostKeyFingerprint: expectedFP,
	}

	result, err := ssh.TestConnection(srv)
	if err != nil {
		t.Fatalf("expected success with matching fingerprint, got error: %v", err)
	}

	if result.Fingerprint != expectedFP {
		t.Fatalf("expected fingerprint %q, got %q", expectedFP, result.Fingerprint)
	}
}

func TestTestConnection_KeyAuth_Succeeds(t *testing.T) {
	// Generate a test key pair
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientSigner, err := gossh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatalf("create client signer: %v", err)
	}

	// Write key to temp file
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "id_rsa")
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	// Start server that accepts this public key
	addr, expectedFP, cleanup := testSSHServer(t, func(cfg *gossh.ServerConfig) {
		cfg.PublicKeyCallback = func(conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			if conn.User() == "testuser" &&
				string(key.Marshal()) == string(clientSigner.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("public key rejected")
		}
	})
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:     host,
		Port:     port,
		AuthType: model.AuthTypeKey,
		Username: "testuser",
		KeyPath:  keyPath,
	}

	result, err := ssh.TestConnection(srv)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if result.Fingerprint != expectedFP {
		t.Fatalf("expected fingerprint %q, got %q", expectedFP, result.Fingerprint)
	}
}

func TestTestConnection_PassphraseProtectedKey_ReturnsError(t *testing.T) {
	// Generate a key and encrypt it with a passphrase
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}

	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "id_rsa_enc")

	// Use OpenSSH format with passphrase via ssh.MarshalPrivateKeyWithPassphrase
	encPEM, err := gossh.MarshalPrivateKeyWithPassphrase(clientKey, "", []byte("testpassphrase"))
	if err != nil {
		t.Fatalf("marshal encrypted key: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(encPEM), 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	addr, _, cleanup := testSSHServer(t)
	defer cleanup()

	host, port := splitHostPort(t, addr)

	srv := model.Server{
		Host:     host,
		Port:     port,
		AuthType: model.AuthTypeKey,
		Username: "testuser",
		KeyPath:  keyPath,
	}

	_, err = ssh.TestConnection(srv)
	if err == nil {
		t.Fatal("expected error for passphrase-protected key")
	}

	var connErr *ssh.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *ssh.ConnectionError, got %T: %v", err, err)
	}
	if connErr.Kind != ssh.ErrPassphrase {
		t.Fatalf("expected ErrPassphrase, got %v", connErr.Kind)
	}
}

func TestTestConnection_NetworkError_ReturnsNetworkError(t *testing.T) {
	// Use a port that's not listening
	srv := model.Server{
		Host:     "127.0.0.1",
		Port:     1, // unlikely to be open
		AuthType: model.AuthTypePassword,
		Username: "testuser",
		Password: "testpass",
	}

	_, err := ssh.TestConnection(srv)
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}

	var connErr *ssh.ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *ssh.ConnectionError, got %T: %v", err, err)
	}
	if connErr.Kind != ssh.ErrNetwork {
		t.Fatalf("expected ErrNetwork, got %v", connErr.Kind)
	}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}
