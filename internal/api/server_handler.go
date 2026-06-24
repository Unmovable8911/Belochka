package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"belochka/internal/model"
	"belochka/internal/ssh"

	"github.com/go-chi/chi/v5"
)

// ServerStore defines the persistence operations required by the server handler.
type ServerStore interface {
	Create(ctx context.Context, srv model.Server) (model.Server, error)
	GetByID(ctx context.Context, id string) (model.Server, error)
	List(ctx context.Context) ([]model.Server, error)
	Update(ctx context.Context, srv model.Server) (model.Server, error)
	Delete(ctx context.Context, id string) error
}

// SSHTester tests SSH connectivity to a server.
type SSHTester interface {
	TestConnection(srv model.Server) (ssh.TestResult, error)
}

// serverHandler handles server CRUD and test endpoints.
type serverHandler struct {
	store    ServerStore
	tester   SSHTester
	onChange func()
}

func (h *serverHandler) notifyChange() {
	if h.onChange != nil {
		h.onChange()
	}
}

// errorBody is the unified error response format.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// serverResponse is a Server without the Password field, used for JSON responses.
type serverResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Host               string          `json:"host"`
	Port               int             `json:"port"`
	AuthType           model.AuthType  `json:"auth_type"`
	Username           string          `json:"username"`
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
		KeyPath:            srv.KeyPath,
		HostKeyFingerprint: srv.HostKeyFingerprint,
		CreatedAt:          srv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          srv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{
		Error: errorDetail{Code: code, Message: message},
	})
}

func (h *serverHandler) create(w http.ResponseWriter, r *http.Request) {
	var srv model.Server
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	if problems := validateServer(srv); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", strings.Join(problems, "; "))
		return
	}

	created, err := h.store.Create(r.Context(), srv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to create server")
		return
	}

	writeJSON(w, http.StatusCreated, toServerResponse(created))
	h.notifyChange()
}

func (h *serverHandler) list(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to list servers")
		return
	}

	resp := make([]serverResponse, len(servers))
	for i, srv := range servers {
		resp[i] = toServerResponse(srv)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *serverHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to get server")
		return
	}

	writeJSON(w, http.StatusOK, toServerResponse(srv))
}

func (h *serverHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var srv model.Server
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	srv.ID = id

	if problems := validateServer(srv); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", strings.Join(problems, "; "))
		return
	}

	updated, err := h.store.Update(r.Context(), srv)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to update server")
		return
	}

	writeJSON(w, http.StatusOK, toServerResponse(updated))
	h.notifyChange()
}

func (h *serverHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.store.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to delete server")
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.notifyChange()
}

func (h *serverHandler) testConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "Server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to get server")
		return
	}

	result, err := h.tester.TestConnection(srv)
	if err != nil {
		var connErr *ssh.ConnectionError
		if errors.As(err, &connErr) {
			writeError(w, http.StatusUnprocessableEntity, string(connErr.Kind), connErr.Message)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "connection_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func validateServer(srv model.Server) []string {
	var problems []string
	if strings.TrimSpace(srv.Name) == "" {
		problems = append(problems, "name is required")
	}
	if strings.TrimSpace(srv.Host) == "" {
		problems = append(problems, "host is required")
	}
	if strings.TrimSpace(srv.Username) == "" {
		problems = append(problems, "username is required")
	}
	return problems
}
