package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"belochka/internal/httpx"
	"belochka/internal/model"

	"github.com/go-chi/chi/v5"
)

// GroupStore defines the persistence operations required by the group handler.
type GroupStore interface {
	CreateGroup(ctx context.Context, grp model.Group) (model.Group, error)
	GetGroupByID(ctx context.Context, id string) (model.Group, error)
	ListGroups(ctx context.Context) ([]model.Group, error)
	UpdateGroup(ctx context.Context, grp model.Group) (model.Group, error)
	DeleteGroup(ctx context.Context, id string) error
	GroupMemberCount(ctx context.Context, groupID string) (int, error)
}

// groupHandler handles group CRUD endpoints.
type groupHandler struct {
	store GroupStore
}

// groupResponse is the JSON shape for a single group in list/get responses.
type groupResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toGroupResponse(g model.Group, memberCount int) groupResponse {
	return groupResponse{
		ID:          g.ID,
		Name:        g.Name,
		MemberCount: memberCount,
		CreatedAt:   g.CreatedAt.Format(timeFormat),
		UpdatedAt:   g.UpdatedAt.Format(timeFormat),
	}
}

func (h *groupHandler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	if strings.TrimSpace(input.Name) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	grp := model.Group{
		Name: strings.TrimSpace(input.Name),
	}

	created, err := h.store.CreateGroup(r.Context(), grp)
	if err != nil {
		if errors.Is(err, model.ErrGroupDuplicateName) {
			httpx.WriteError(w, http.StatusConflict, "duplicate_name", err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to create group")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *groupHandler) list(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListGroups(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to list groups")
		return
	}

	resp := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		memberCount, _ := h.store.GroupMemberCount(r.Context(), g.ID)
		resp = append(resp, toGroupResponse(g, memberCount))
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *groupHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	grp, err := h.store.GetGroupByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrGroupNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to get group")
		return
	}

	memberCount, _ := h.store.GroupMemberCount(r.Context(), grp.ID)

	httpx.WriteJSON(w, http.StatusOK, toGroupResponse(grp, memberCount))
}

func (h *groupHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.store.GetGroupByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrGroupNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to get group")
		return
	}

	var input struct {
		Name *string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	// Merge partial update: nil means "keep existing value".
	name := existing.Name
	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "validation_failed", "name is required")
			return
		}
		name = strings.TrimSpace(*input.Name)
	}

	grp := model.Group{
		ID:   id,
		Name: name,
	}

	updated, err := h.store.UpdateGroup(r.Context(), grp)
	if err != nil {
		if errors.Is(err, model.ErrGroupNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		if errors.Is(err, model.ErrGroupDuplicateName) {
			httpx.WriteError(w, http.StatusConflict, "duplicate_name", err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to update group")
		return
	}

	memberCount, _ := h.store.GroupMemberCount(r.Context(), updated.ID)

	httpx.WriteJSON(w, http.StatusOK, toGroupResponse(updated, memberCount))
}

func (h *groupHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.store.DeleteGroup(r.Context(), id); err != nil {
		if errors.Is(err, model.ErrGroupNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Group not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "store_error", "Failed to delete group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
