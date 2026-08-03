package app

import (
	"context"
	"fmt"
	"log/slog"

	"belochka/internal/model"
)

// ServerLister lists servers without their passwords.
// Satisfied by *store.SQLiteStore.
type ServerLister interface {
	ListWithoutPasswords(ctx context.Context) ([]model.Server, error)
}

// PoolSyncer manages SSH pool membership for synchronisation.
// Satisfied by *ssh.Pool.
type PoolSyncer interface {
	Add(ctx context.Context, serverID string)
	Remove(serverID string)
}

// CollectorSyncer manages collector membership for synchronisation.
// Satisfied by *monitor.Manager.
type CollectorSyncer interface {
	Add(ctx context.Context, serverID string)
	Remove(serverID string)
	ServerIDs() []string
}

// ServerSynchronizer keeps the SSH pool and collector manager in sync
// with the current set of servers in the database. It encapsulates the
// diff-and-apply logic behind narrow interfaces so it can be unit-tested
// without a real database, SSH pool, or collector manager.
type ServerSynchronizer struct {
	servers   ServerLister
	pool      PoolSyncer
	collector CollectorSyncer
}

// NewServerSynchronizer creates a ServerSynchronizer wired to the given
// implementations.
func NewServerSynchronizer(servers ServerLister, pool PoolSyncer, collector CollectorSyncer) *ServerSynchronizer {
	return &ServerSynchronizer{
		servers:   servers,
		pool:      pool,
		collector: collector,
	}
}

// Sync ensures the SSH pool and collector manager contain exactly the
// servers in the database. Servers found in the store but not in the
// pool/collector are added; servers in the pool/collector but no longer
// in the store are removed.
func (s *ServerSynchronizer) Sync(ctx context.Context) error {
	servers, err := s.servers.ListWithoutPasswords(ctx)
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}

	currentIDs := make(map[string]bool, len(servers))
	for _, srv := range servers {
		currentIDs[srv.ID] = true
		s.pool.Add(ctx, srv.ID)
		s.collector.Add(ctx, srv.ID)
	}

	for _, id := range s.collector.ServerIDs() {
		if !currentIDs[id] {
			s.collector.Remove(id)
			s.pool.Remove(id)
		}
	}

	return nil
}

// SyncAndLog calls Sync and logs any error at ERROR level.
// Convenience wrapper for call sites that cannot propagate the error.
func (s *ServerSynchronizer) SyncAndLog(ctx context.Context) {
	if err := s.Sync(ctx); err != nil {
		slog.Error("server sync failed", "error", err)
	}
}
