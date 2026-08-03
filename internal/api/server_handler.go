package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"belochka/internal/httpx"
	"belochka/internal/model"
	"belochka/internal/ssh"

	"github.com/go-chi/chi/v5"
)

// ServerStore defines the persistence operations required by the server handler.
type ServerStore interface {
	Create(ctx context.Context, srv model.Server) (model.Server, error)
	GetByID(ctx context.Context, id string) (model.Server, error)
	List(ctx context.Context) ([]model.Server, error)
	ListByGroup(ctx context.Context, groupID *string) ([]model.Server, error)
	Update(ctx context.Context, srv model.Server) (model.Server, error)
	Delete(ctx context.Context, id string) error
}

// SSHTester tests SSH connectivity to a server.
type SSHTester func(srv model.Server) (ssh.TestResult, error)

// Reconnecter triggers a reconnection for a server and returns its status.
type Reconnecter interface {
	Reconnect(ctx context.Context, serverID string) ssh.ConnStatus
}

// serverHandler handles server CRUD and test endpoints.
type serverHandler struct {
	store       ServerStore
	tester      SSHTester
	reconnecter Reconnecter
	onChange    func()
}

func (h *serverHandler) notifyChange() {
	if h.onChange != nil {
		h.onChange()
	}
}

// serverResponse is a Server without the Password field, used for JSON responses.
type serverResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Host               string          `json:"host"`
	Port               int             `json:"port"`
	AuthType           model.AuthType  `json:"auth_type"`
	Username           string          `json:"username"`
	GroupID            *string         `json:"group_id,omitempty"`
	KeyPath            string          `json:"key_path,omitempty"`
	HostKeyFingerprint string          `json:"host_key_fingerprint,omitempty"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
}

func toServerResponse(srv model.Server) serverResponse {
	return serverResponse{
		ID:                 srv.ID,
		Name:               srv.Name,
		Host:               srv.Host,
		Port:               srv.Port,
		AuthType:           srv.AuthType,
		Username:           srv.Username,
		GroupID:            srv.GroupID,
		KeyPath:            srv.KeyPath,
		HostKeyFingerprint: srv.HostKeyFingerprint,
		CreatedAt:          srv.CreatedAt.Format(timeFormat),
		UpdatedAt:          srv.UpdatedAt.Format(timeFormat),
	}
}

func (h *serverHandler) create(w http.ResponseWriter, r *http.Request) {
	var srv model.Server
	if err := httpx.DecodeJSON(r, &srv); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	if problems := srv.Validate(); len(problems) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", strings.Join(problems, "; "))
		return
	}

	created, err := h.store.Create(r.Context(), srv)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to create server")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toServerResponse(created))
	h.notifyChange()
}

func (h *serverHandler) list(w http.ResponseWriter, r *http.Request) {
	var servers []model.Server
	var err error

	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr != "" {
		servers, err = h.store.ListByGroup(r.Context(), &groupIDStr)
	} else {
		servers, err = h.store.List(r.Context())
	}

	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to list servers")
		return
	}

	resp := make([]serverResponse, len(servers))
	for i, srv := range servers {
		resp[i] = toServerResponse(srv)
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *serverHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrServerNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to get server")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toServerResponse(srv))
}

func (h *serverHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrServerNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to get server")
		return
	}

	// Decode into a map first to detect which keys are present (for GroupID
	// null-vs-absent disambiguation), then decode into the Server struct for
	// typed field access.
	bodyBytes, err := readBody(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	var srv model.Server
	if err := json.Unmarshal(bodyBytes, &srv); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	mergeServerFields(&existing, srv, raw)

	if problems := existing.Validate(); len(problems) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", strings.Join(problems, "; "))
		return
	}

	updated, err := h.store.Update(r.Context(), existing)
	if err != nil {
		if errors.Is(err, model.ErrServerNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to update server")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toServerResponse(updated))
	h.notifyChange()
}

func (h *serverHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, model.ErrServerNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to delete server")
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.notifyChange()
}

func (h *serverHandler) testConnection(w http.ResponseWriter, r *http.Request) {
	var srv model.Server
	if err := httpx.DecodeJSON(r, &srv); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	if problems := srv.Validate(); len(problems) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", strings.Join(problems, "; "))
		return
	}

	// When re-testing an existing server without re-entering its password,
	// reuse the stored secret. This is read-only and persists nothing.
	if srv.AuthType == model.AuthTypePassword && srv.Password == "" && srv.ID != "" {
		if stored, err := h.store.GetByID(r.Context(), srv.ID); err == nil {
			srv.Password = stored.Password
		}
	}

	result, err := h.tester(srv)
	if err != nil {
		var connErr *ssh.ConnectionError
		if errors.As(err, &connErr) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, string(connErr.Kind), connErr.Message)
			return
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "connection_failed", err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *serverHandler) reconnect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify the server exists in the store.
	_, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrServerNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to get server")
		return
	}

	status := h.reconnecter.Reconnect(r.Context(), id)
	httpx.WriteJSON(w, http.StatusOK, status)
}


// mergeServerFields applies fields from srv to existing only for keys
// present in the request body (raw).
func mergeServerFields(existing *model.Server, srv model.Server, raw map[string]json.RawMessage) {
	if _, ok := raw["name"]; ok {
		existing.Name = srv.Name
	}
	if _, ok := raw["host"]; ok {
		existing.Host = srv.Host
	}
	if _, ok := raw["port"]; ok {
		existing.Port = srv.Port
	}
	if _, ok := raw["username"]; ok {
		existing.Username = srv.Username
	}
	if _, ok := raw["auth_type"]; ok {
		existing.AuthType = srv.AuthType
	}
	if _, ok := raw["group_id"]; ok {
		existing.GroupID = srv.GroupID
	}
	// Password, KeyPath, and HostKeyFingerprint are optional; empty means "no change".
	if srv.Password != "" {
		existing.Password = srv.Password
	}
	if srv.KeyPath != "" {
		existing.KeyPath = srv.KeyPath
	}
	if srv.HostKeyFingerprint != "" {
		existing.HostKeyFingerprint = srv.HostKeyFingerprint
	}
}

// readBody reads and returns the full request body.
func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}
