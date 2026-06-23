package broadcast

import (
	"encoding/json"

	"belochka/internal/model"
)

// ServerInfo holds the server identity and connection state needed
// for assembling a broadcast snapshot.
type ServerInfo struct {
	ID        string
	Name      string
	Host      string
	State     string
	Attempts  int
	LastError string
}

// Assemble builds a JSON-encoded snapshot from server info and metrics,
// ready for broadcasting to WebSocket clients.
func Assemble(servers []ServerInfo, snapshots map[string]*model.Snapshot) ([]byte, error) {
	infos := make([]wireServerInfo, len(servers))
	metrics := make(map[string]wireMetrics)

	for i, s := range servers {
		infos[i] = wireServerInfo{
			ID:        s.ID,
			Name:      s.Name,
			Host:      s.Host,
			Status:    s.State,
			Attempts:  s.Attempts,
			LastError: s.LastError,
		}

		if snap, ok := snapshots[s.ID]; ok && snap != nil {
			metrics[s.ID] = snapshotToWire(*snap)
		}
	}

	return json.Marshal(wireSnapshot{
		Servers: infos,
		Metrics: metrics,
	})
}
