package ssh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"belochka/internal/clock"
	"belochka/internal/model"

	gossh "golang.org/x/crypto/ssh"
)

// Executor executes a shell command on a remote server via SSH and returns
// its combined stdout+stderr. A non-zero exit code is returned as an error.
type Executor interface {
	Execute(ctx context.Context, serverID, cmd string) (string, error)
}

// ServerProvider fetches server configuration by ID.
type ServerProvider interface {
	GetByID(ctx context.Context, id string) (model.Server, error)
}

type managedConn struct {
	mu       sync.RWMutex
	client   *gossh.Client
	recon    *Reconnector
	kaCancel context.CancelFunc
	cancel   context.CancelFunc
}

// Pool manages persistent SSH connections to multiple servers.
type Pool struct {
	provider ServerProvider
	clock    clock.Clock

	mu    sync.RWMutex
	conns map[string]*managedConn
}

// NewPool creates a new SSH connection pool.
func NewPool(provider ServerProvider, clk clock.Clock) *Pool {
	return &Pool{
		provider: provider,
		clock:    clk,
		conns:    make(map[string]*managedConn),
	}
}

// Add starts a persistent SSH connection for the given server.
// If a connection already exists, it is a no-op.
func (p *Pool) Add(ctx context.Context, serverID string) {
	p.mu.Lock()
	if _, exists := p.conns[serverID]; exists {
		p.mu.Unlock()
		return
	}

	connCtx, connCancel := context.WithCancel(ctx)
	mc := &managedConn{cancel: connCancel}
	p.conns[serverID] = mc
	p.mu.Unlock()

	connectFn := func(ctx context.Context) error {
		srv, err := p.provider.GetByID(ctx, serverID)
		if err != nil {
			return &ConnectionError{Kind: ErrNetwork, Message: err.Error(), Cause: err}
		}

		config := &gossh.ClientConfig{
			User:    srv.Username,
			Timeout: dialTimeout,
		}

		auth, err := buildAuth(srv)
		if err != nil {
			return err
		}
		config.Auth = auth

		config.HostKeyCallback = func(hostname string, remote net.Addr, key gossh.PublicKey) error {
			fp := fingerprint(key)
			if srv.HostKeyFingerprint != "" && srv.HostKeyFingerprint != fp {
				return &ConnectionError{
					Kind:    ErrHostKey,
					Message: fmt.Sprintf("host key mismatch: expected %s, got %s", srv.HostKeyFingerprint, fp),
				}
			}
			return nil
		}

		addr := net.JoinHostPort(srv.Host, fmt.Sprintf("%d", srv.Port))
		client, err := gossh.Dial("tcp", addr, config)
		if err != nil {
			return classifyError(err)
		}

		mc.mu.Lock()
		mc.client = client
		mc.mu.Unlock()
		return nil
	}

	mc.recon = NewReconnector(connectFn, p.clock)

	go p.runConnection(connCtx, serverID, mc)
}

func (p *Pool) runConnection(ctx context.Context, serverID string, mc *managedConn) {
	for {
		if ctx.Err() != nil {
			return
		}

		mc.recon.Run(ctx)

		status := mc.recon.Status()
		if status.State != StateConnected {
			return
		}

		slog.Info("SSH connected", "server_id", serverID)

		kaCtx, kaCancel := context.WithCancel(ctx)
		mc.mu.Lock()
		mc.kaCancel = kaCancel
		mc.mu.Unlock()

		ka := NewKeepalive(
			func(ctx context.Context) error {
				mc.mu.RLock()
				client := mc.client
				mc.mu.RUnlock()
				if client == nil {
					return fmt.Errorf("not connected")
				}
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				return err
			},
			func() {
				mc.mu.Lock()
				if mc.client != nil {
					mc.client.Close()
					mc.client = nil
				}
				mc.mu.Unlock()
				kaCancel()
			},
			p.clock,
		)

		ka.Run(kaCtx)

		if ctx.Err() != nil {
			return
		}

		slog.Info("SSH connection lost, reconnecting", "server_id", serverID)
		mc.recon.Reset()
	}
}

// Remove stops and removes the connection for the given server.
func (p *Pool) Remove(serverID string) {
	p.mu.Lock()
	mc, ok := p.conns[serverID]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.conns, serverID)
	p.mu.Unlock()

	mc.cancel()
	mc.mu.Lock()
	if mc.client != nil {
		mc.client.Close()
	}
	mc.mu.Unlock()
}

// clientFor returns the live SSH client for serverID, or an error if the
// server is not in the pool or not currently connected.
func (p *Pool) clientFor(serverID string) (*gossh.Client, error) {
	p.mu.RLock()
	mc, ok := p.conns[serverID]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no connection for server %s", serverID)
	}

	mc.mu.RLock()
	client := mc.client
	mc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("server %s not connected", serverID)
	}

	return client, nil
}

// Execute runs a command on the specified server's SSH connection.
func (p *Pool) Execute(ctx context.Context, serverID, cmd string) (string, error) {
	client, err := p.clientFor(serverID)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// RunCommand executes a command and returns combined stdout+stderr, the exit code,
// and any connection-level error. Unlike Execute, a non-zero exit code is not an
// error — it is returned as exitCode. Only SSH connection failures return a non-nil error.
func (p *Pool) RunCommand(ctx context.Context, serverID, cmd string) (string, int, error) {
	client, err := p.clientFor(serverID)
	if err != nil {
		return "", -1, err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		var exitErr *gossh.ExitError
		if errors.As(err, &exitErr) {
			return string(output), exitErr.ExitStatus(), nil
		}
		return "", -1, err
	}
	return string(output), 0, nil
}

// OpenSession creates a new SSH session on the existing connection for the given server.
func (p *Pool) OpenSession(serverID string) (*gossh.Session, error) {
	client, err := p.clientFor(serverID)
	if err != nil {
		return nil, err
	}

	return client.NewSession()
}

// Status returns the connection status for a server.
func (p *Pool) Status(serverID string) ConnStatus {
	p.mu.RLock()
	mc, ok := p.conns[serverID]
	p.mu.RUnlock()

	if !ok {
		return ConnStatus{State: StateReconnecting}
	}

	return mc.recon.Status()
}

// TriggerReconnect forces a reconnection for the given server.
// No-op if the server is already reconnecting or not in the pool.
func (p *Pool) TriggerReconnect(serverID string) {
	p.mu.RLock()
	mc, ok := p.conns[serverID]
	p.mu.RUnlock()
	if !ok {
		return
	}

	status := mc.recon.Status()
	if status.State != StateConnected {
		return
	}

	mc.mu.Lock()
	if mc.client != nil {
		mc.client.Close()
		mc.client = nil
	}
	if mc.kaCancel != nil {
		mc.kaCancel()
	}
	mc.mu.Unlock()
}

// Reconnect forcefully restarts the connection for the given server.
// It closes the old SSH client, cancels the old context and keepalive,
// resets the Reconnector, and starts a new connection goroutine.
// Works for any state (connected, reconnecting, failed) — idempotent.
// No-op if the server is not in the pool. Returns the connection status
// after initiating the reconnect.
func (p *Pool) Reconnect(ctx context.Context, serverID string) ConnStatus {
	p.mu.RLock()
	mc, ok := p.conns[serverID]
	p.mu.RUnlock()
	if !ok {
		return ConnStatus{State: StateReconnecting}
	}

	// Close existing client and keepalive.
	mc.mu.Lock()
	if mc.client != nil {
		mc.client.Close()
		mc.client = nil
	}
	if mc.kaCancel != nil {
		mc.kaCancel()
	}
	mc.mu.Unlock()

	// Cancel the old connection context (stops the old runConnection loop).
	mc.cancel()

	// Create a new context for the fresh connection loop.
	// Use context.Background() rather than the incoming ctx (which may be
	// an HTTP request context) so the connection goroutine outlives the
	// HTTP handler that initiated the reconnect.
	connCtx, connCancel := context.WithCancel(context.Background())
	mc.mu.Lock()
	mc.cancel = connCancel
	mc.mu.Unlock()

	mc.recon.Reset()
	go p.runConnection(connCtx, serverID, mc)

	return p.Status(serverID)
}

// CloseAll stops all connections and clears the pool.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, mc := range p.conns {
		mc.cancel()
		mc.mu.Lock()
		if mc.client != nil {
			mc.client.Close()
		}
		mc.mu.Unlock()
	}
	p.conns = make(map[string]*managedConn)
}
