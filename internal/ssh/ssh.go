package ssh

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"belochka/internal/model"

	"golang.org/x/crypto/ssh"
)

const dialTimeout = 5 * time.Second

// ErrorKind classifies SSH connection errors.
type ErrorKind string

const (
	ErrAuth        ErrorKind = "auth_failed"
	ErrHostKey     ErrorKind = "host_key_mismatch"
	ErrNetwork     ErrorKind = "network_error"
	ErrPassphrase  ErrorKind = "passphrase_protected_key"
)

// ConnectionError is a classified SSH connection error.
type ConnectionError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *ConnectionError) Error() string {
	return e.Message
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// TestResult holds the outcome of a connection test.
type TestResult struct {
	Fingerprint string `json:"fingerprint"`
}

// TestConnection tests SSH connectivity to the server described by srv.
// On success it returns the host key fingerprint in SHA256 format.
func TestConnection(srv model.Server) (TestResult, error) {
	config := &ssh.ClientConfig{
		User:    srv.Username,
		Timeout: dialTimeout,
	}

	auth, err := buildAuth(srv)
	if err != nil {
		return TestResult{}, err
	}
	config.Auth = auth

	addr := net.JoinHostPort(srv.Host, fmt.Sprintf("%d", srv.Port))

	// Capture the host key fingerprint via callback
	var hostKeyFP string
	config.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostKeyFP = fingerprint(key)

		// Check stored fingerprint if present
		if srv.HostKeyFingerprint != "" && srv.HostKeyFingerprint != hostKeyFP {
			return &ConnectionError{
				Kind:    ErrHostKey,
				Message: fmt.Sprintf("host key mismatch: expected %s, got %s", srv.HostKeyFingerprint, hostKeyFP),
			}
		}
		return nil
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return TestResult{}, classifyError(err)
	}
	client.Close()

	return TestResult{Fingerprint: hostKeyFP}, nil
}

// fingerprint computes the SHA256 fingerprint of an SSH public key.
func fingerprint(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}

// buildAuth constructs the SSH auth methods for the given server config.
func buildAuth(srv model.Server) ([]ssh.AuthMethod, error) {
	switch srv.AuthType {
	case model.AuthTypePassword:
		return []ssh.AuthMethod{ssh.Password(srv.Password)}, nil

	case model.AuthTypeKey:
		keyBytes, err := os.ReadFile(srv.KeyPath)
		if err != nil {
			return nil, &ConnectionError{
				Kind:    ErrNetwork,
				Message: fmt.Sprintf("failed to read key file: %v", err),
				Cause:   err,
			}
		}

		// Detect passphrase-protected keys
		block, _ := pem.Decode(keyBytes)
		if block != nil && strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return nil, &ConnectionError{
				Kind:    ErrPassphrase,
				Message: "key file is passphrase-protected; passphrase-protected keys are not supported",
			}
		}

		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			// ParsePrivateKey returns a *ssh.PassphraseMissingError for encrypted keys
			var ppErr *ssh.PassphraseMissingError
			if errors.As(err, &ppErr) {
				return nil, &ConnectionError{
					Kind:    ErrPassphrase,
					Message: "key file is passphrase-protected; passphrase-protected keys are not supported",
					Cause:   err,
				}
			}
			return nil, &ConnectionError{
				Kind:    ErrAuth,
				Message: fmt.Sprintf("failed to parse key file: %v", err),
				Cause:   err,
			}
		}

		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", srv.AuthType)
	}
}

// classifyError wraps a raw SSH/network error into a ConnectionError.
func classifyError(err error) error {
	// Check if it's already a ConnectionError (e.g. from host key callback)
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return connErr
	}

	// Check for network-level errors (timeout, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return &ConnectionError{
			Kind:    ErrNetwork,
			Message: fmt.Sprintf("network error: %v", err),
			Cause:   err,
		}
	}

	// Default: treat as auth failure for SSH-level errors
	return &ConnectionError{
		Kind:    ErrAuth,
		Message: fmt.Sprintf("authentication failed: %v", err),
		Cause:   err,
	}
}
