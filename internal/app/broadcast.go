package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"belochka/internal/hub"
	"belochka/internal/model"
	"belochka/internal/ssh"
)

// PoolStatusProvider provides connection status for individual servers.
// Satisfied by *ssh.Pool.
type PoolStatusProvider interface {
	Status(serverID string) ssh.ConnStatus
}

// SnapshotProvider provides the latest snapshot for a server.
// Satisfied by *monitor.Manager.
type SnapshotProvider interface {
	Latest(serverID string) *model.Snapshot
}

// serverInfo holds the server identity and connection state needed
// for assembling a broadcast snapshot.
type serverInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	State     string  `json:"status"`
	Attempts  int     `json:"attempts,omitempty"`
	LastError string  `json:"lastError,omitempty"`
	GroupID   *string `json:"group_id,omitempty"`
}

// broadcastMsg is the top-level JSON envelope sent to WebSocket clients.
type broadcastMsg struct {
	Servers []serverInfo               `json:"servers"`
	Metrics map[string]*model.Snapshot `json:"metrics"`
}

// assemble builds a JSON-encoded snapshot from server info and metrics,
// ready for broadcasting to WebSocket clients.
func assemble(servers []serverInfo, snapshots map[string]*model.Snapshot) ([]byte, error) {
	metrics := make(map[string]*model.Snapshot)

	for _, s := range servers {
		if snap, ok := snapshots[s.ID]; ok && snap != nil {
			metrics[s.ID] = snap
		}
	}

	srvList := servers
	if srvList == nil {
		srvList = []serverInfo{}
	}

	return json.Marshal(broadcastMsg{
		Servers: srvList,
		Metrics: metrics,
	})
}

// BroadcastService assembles and broadcasts server+metrics snapshots
// to connected WebSocket clients. It depends on narrow interfaces rather
// than concrete types, making it independently testable.
type BroadcastService struct {
	servers   ServerLister
	pool      PoolStatusProvider
	snapshots SnapshotProvider
	hub       hub.Broadcaster
}

// NewBroadcastService creates a BroadcastService.
func NewBroadcastService(
	servers ServerLister,
	pool PoolStatusProvider,
	snapshots SnapshotProvider,
	hub hub.Broadcaster,
) *BroadcastService {
	return &BroadcastService{
		servers:   servers,
		pool:      pool,
		snapshots: snapshots,
		hub:       hub,
	}
}

// Broadcast assembles the current server list, connection states, and
// metrics snapshots into a JSON envelope and sends it to all connected
// WebSocket clients. Does nothing if no clients are connected.
func (b *BroadcastService) Broadcast(ctx context.Context) {
	if b.hub.ClientCount() == 0 {
		return
	}

	servers, err := b.servers.ListWithoutPasswords(ctx)
	if err != nil {
		slog.Error("failed to list servers for broadcast", "error", err)
		return
	}

	infos := make([]serverInfo, len(servers))
	snapshots := make(map[string]*model.Snapshot)

	for i, s := range servers {
		status := b.pool.Status(s.ID)
		infos[i] = serverInfo{
			ID:        s.ID,
			Name:      s.Name,
			Host:      s.Host,
			State:     string(status.State),
			Attempts:  status.Attempts,
			LastError: status.LastError,
			GroupID:   s.GroupID,
		}

		if snap := b.snapshots.Latest(s.ID); snap != nil {
			snapshots[s.ID] = snap
		}
	}

	data, err := assemble(infos, snapshots)
	if err != nil {
		slog.Error("failed to assemble broadcast", "error", err)
		return
	}
	b.hub.SetSnapshot(data)
	b.hub.BroadcastMsg("snapshot", data)
}
