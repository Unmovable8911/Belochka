package app_test

import (
	"context"
	"errors"
	"testing"

	"belochka/internal/app"
	"belochka/internal/model"
)

// stubServerLister implements app.ServerLister for tests.
type stubServerLister struct {
	servers []model.Server
	err     error
}

func (s *stubServerLister) ListWithoutPasswords(_ context.Context) ([]model.Server, error) {
	return s.servers, s.err
}

// stubPoolSyncer implements app.PoolSyncer for tests.
type stubPoolSyncer struct {
	added   []string
	removed []string
}

func (s *stubPoolSyncer) Add(_ context.Context, serverID string) {
	s.added = append(s.added, serverID)
}

func (s *stubPoolSyncer) Remove(serverID string) {
	s.removed = append(s.removed, serverID)
}

// stubCollectorSyncer implements app.CollectorSyncer for tests.
type stubCollectorSyncer struct {
	added     []string
	removed   []string
	serverIDs []string
}

func (s *stubCollectorSyncer) Add(_ context.Context, serverID string) {
	s.added = append(s.added, serverID)
}

func (s *stubCollectorSyncer) Remove(serverID string) {
	s.removed = append(s.removed, serverID)
}

func (s *stubCollectorSyncer) ServerIDs() []string {
	return s.serverIDs
}

func TestServerSynchronizer_Sync_AddsNewServers(t *testing.T) {
	lister := &stubServerLister{
		servers: []model.Server{
			{ID: "s1", Name: "alpha"},
			{ID: "s2", Name: "beta"},
		},
	}
	pool := &stubPoolSyncer{}
	collector := &stubCollectorSyncer{}

	sync := app.NewServerSynchronizer(lister, pool, collector)
	if err := sync.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(pool.added) != 2 {
		t.Errorf("pool added: want 2, got %d: %v", len(pool.added), pool.added)
	}
	if len(collector.added) != 2 {
		t.Errorf("collector added: want 2, got %d: %v", len(collector.added), collector.added)
	}
}

func TestServerSynchronizer_Sync_RemovesStaleServers(t *testing.T) {
	lister := &stubServerLister{
		servers: []model.Server{
			{ID: "s1", Name: "alpha"},
		},
	}
	pool := &stubPoolSyncer{}
	collector := &stubCollectorSyncer{
		// s2 is in the collector but not in the lister — should be removed.
		serverIDs: []string{"s1", "s2"},
	}

	sync := app.NewServerSynchronizer(lister, pool, collector)
	if err := sync.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(pool.removed) != 1 || pool.removed[0] != "s2" {
		t.Errorf("pool removed: want [s2], got %v", pool.removed)
	}
	if len(collector.removed) != 1 || collector.removed[0] != "s2" {
		t.Errorf("collector removed: want [s2], got %v", collector.removed)
	}
}

func TestServerSynchronizer_Sync_NoOpWhenInSync(t *testing.T) {
	lister := &stubServerLister{
		servers: []model.Server{
			{ID: "s1", Name: "alpha"},
		},
	}
	pool := &stubPoolSyncer{}
	collector := &stubCollectorSyncer{
		serverIDs: []string{"s1"},
	}

	sync := app.NewServerSynchronizer(lister, pool, collector)
	if err := sync.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// s1 was already known — Add is idempotent so it's called but not harmful.
	// No removes should happen.
	if len(pool.removed) != 0 {
		t.Errorf("pool removed: want 0, got %v", pool.removed)
	}
	if len(collector.removed) != 0 {
		t.Errorf("collector removed: want 0, got %v", collector.removed)
	}
}

func TestServerSynchronizer_Sync_EmptyServersClearsAll(t *testing.T) {
	lister := &stubServerLister{
		servers: []model.Server{},
	}
	pool := &stubPoolSyncer{}
	collector := &stubCollectorSyncer{
		serverIDs: []string{"s1", "s2", "s3"},
	}

	sync := app.NewServerSynchronizer(lister, pool, collector)
	if err := sync.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(pool.removed) != 3 {
		t.Errorf("pool removed: want 3, got %d", len(pool.removed))
	}
	if len(collector.removed) != 3 {
		t.Errorf("collector removed: want 3, got %d", len(collector.removed))
	}
}

func TestServerSynchronizer_Sync_ListError(t *testing.T) {
	wantErr := errors.New("db down")
	lister := &stubServerLister{err: wantErr}
	pool := &stubPoolSyncer{}
	collector := &stubCollectorSyncer{}

	sync := app.NewServerSynchronizer(lister, pool, collector)
	err := sync.Sync(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}
