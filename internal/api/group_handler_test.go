package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"belochka/internal/api"
	"belochka/internal/hub"
	"belochka/internal/model"
)

// mockGroupStore implements api.GroupStore for testing.
type mockGroupStore struct {
	mu      sync.Mutex
	groups  map[string]model.Group
	nextID  int
}

func newMockGroupStore() *mockGroupStore {
	return &mockGroupStore{groups: make(map[string]model.Group)}
}

func (m *mockGroupStore) CreateGroup(_ context.Context, grp model.Group) (model.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check global name uniqueness
	for _, existing := range m.groups {
		if existing.Name == grp.Name {
			return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupDuplicateName, grp.Name)
		}
	}

	m.nextID++
	grp.ID = fmt.Sprintf("grp-%d", m.nextID)
	m.groups[grp.ID] = grp
	return grp, nil
}

func (m *mockGroupStore) GetGroupByID(_ context.Context, id string) (model.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	grp, ok := m.groups[id]
	if !ok {
		return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupNotFound, id)
	}
	return grp, nil
}

func (m *mockGroupStore) ListGroups(_ context.Context) ([]model.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []model.Group
	for _, grp := range m.groups {
		result = append(result, grp)
	}
	return result, nil
}

func (m *mockGroupStore) UpdateGroup(_ context.Context, grp model.Group) (model.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.groups[grp.ID]
	if !ok {
		return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupNotFound, grp.ID)
	}

	// Check global name uniqueness (excluding self)
	for _, other := range m.groups {
		if other.ID == grp.ID {
			continue
		}
		if other.Name == grp.Name {
			return model.Group{}, fmt.Errorf("%w: %s", model.ErrGroupDuplicateName, grp.Name)
		}
	}

	grp.CreatedAt = existing.CreatedAt
	m.groups[grp.ID] = grp
	return grp, nil
}

func (m *mockGroupStore) DeleteGroup(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[id]; !ok {
		return fmt.Errorf("%w: %s", model.ErrGroupNotFound, id)
	}

	delete(m.groups, id)
	return nil
}

func (m *mockGroupStore) GroupMemberCount(_ context.Context, groupID string) (int, error) {
	return 0, nil
}

func setupGroupRouter(store api.GroupStore) http.Handler {
	h := hub.New()
	return api.NewRouter(h, api.WithGroupStore(store))
}

func TestCreateGroup_Returns201(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "Production",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var grp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&grp)
	if grp["name"] != "Production" {
		t.Fatalf("expected name Production, got %v", grp["name"])
	}
	if grp["id"] == nil || grp["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestCreateGroup_EmptyName_Returns400(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{"name": "  "})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateGroup_DuplicateName_Returns409(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{"name": "Production"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first create, got %d", rec.Code)
	}

	// Second create with the same name
	req = httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListGroups_ReturnsFlatArray(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	// Create a few groups
	for _, name := range []string{"A", "B", "C"} {
		body, _ := json.Marshal(map[string]interface{}{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var groups []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&groups)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
}

func TestGetGroup_ReturnsGroup(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{"name": "Production"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	req = httptest.NewRequest(http.MethodGet, "/api/groups/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var grp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&grp)
	if grp["name"] != "Production" {
		t.Fatalf("expected name Production, got %v", grp["name"])
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/groups/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateGroup_Rename(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	// Create
	body, _ := json.Marshal(map[string]interface{}{"name": "Prod"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Update
	body, _ = json.Marshal(map[string]interface{}{"name": "Production"})
	req = httptest.NewRequest(http.MethodPut, "/api/groups/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&updated)
	if updated["name"] != "Production" {
		t.Fatalf("expected name Production, got %v", updated["name"])
	}
}

func TestUpdateGroup_NotFound(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	body, _ := json.Marshal(map[string]interface{}{"name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/api/groups/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteGroup_Returns204(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	// Create group
	body, _ := json.Marshal(map[string]interface{}{"name": "Production"})
	req := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Delete it
	req = httptest.NewRequest(http.MethodDelete, "/api/groups/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it no longer exists
	req = httptest.NewRequest(http.MethodGet, "/api/groups/"+id, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestDeleteGroup_NotFound(t *testing.T) {
	store := newMockGroupStore()
	router := setupGroupRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

