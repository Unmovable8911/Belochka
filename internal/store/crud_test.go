package store

import (
	"context"
	"strings"
	"testing"

	"belochka/internal/model"
)

func testServer() model.Server {
	return model.Server{
		Name:     "web-prod-01",
		Host:     "192.168.1.10",
		Port:     22,
		AuthType: model.AuthTypePassword,
		Username: "admin",
		Password: "s3cret!",
	}
}

func TestCreateAndGetByID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	input := testServer()

	created, err := s.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Error("created server should have an ID")
	}
	if created.Name != input.Name {
		t.Errorf("name = %q, want %q", created.Name, input.Name)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
	if created.Password != "" {
		t.Error("Create should not return password in result")
	}

	// Retrieve by ID
	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.Name != input.Name {
		t.Errorf("Name = %q, want %q", got.Name, input.Name)
	}
	if got.Host != input.Host {
		t.Errorf("Host = %q, want %q", got.Host, input.Host)
	}
	if got.Port != input.Port {
		t.Errorf("Port = %d, want %d", got.Port, input.Port)
	}
	if got.AuthType != input.AuthType {
		t.Errorf("AuthType = %q, want %q", got.AuthType, input.AuthType)
	}
	if got.Username != input.Username {
		t.Errorf("Username = %q, want %q", got.Username, input.Username)
	}
	// Password should be decrypted on read
	if got.Password != input.Password {
		t.Errorf("Password = %q, want %q", got.Password, input.Password)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("GetByID should fail for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Empty list
	servers, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}

	// Add two servers
	srv1 := testServer()
	srv1.Name = "server-1"
	created1, err := s.Create(ctx, srv1)
	if err != nil {
		t.Fatalf("Create server-1 failed: %v", err)
	}

	srv2 := testServer()
	srv2.Name = "server-2"
	srv2.Host = "192.168.1.11"
	_, err = s.Create(ctx, srv2)
	if err != nil {
		t.Fatalf("Create server-2 failed: %v", err)
	}

	servers, err = s.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	// Should be ordered by created_at (server-1 first)
	if servers[0].ID != created1.ID {
		t.Error("first server should be the one created first")
	}
	if servers[0].Name != "server-1" {
		t.Errorf("first server name = %q, want %q", servers[0].Name, "server-1")
	}
	if servers[1].Name != "server-2" {
		t.Errorf("second server name = %q, want %q", servers[1].Name, "server-2")
	}
}

func TestUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, testServer())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Fetch the full server (with password)
	fetched, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Update name and host
	fetched.Name = "web-prod-02"
	fetched.Host = "10.0.0.5"
	fetched.Password = "new-password"

	updated, err := s.Update(ctx, fetched)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Name != "web-prod-02" {
		t.Errorf("Name = %q, want %q", updated.Name, "web-prod-02")
	}
	if updated.Host != "10.0.0.5" {
		t.Errorf("Host = %q, want %q", updated.Host, "10.0.0.5")
	}
	if updated.Password != "new-password" {
		t.Errorf("Password = %q, want %q", updated.Password, "new-password")
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Error("UpdatedAt should be after original")
	}
}

func TestUpdatePreservesPasswordWhenEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, testServer())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update with empty password (name-only edit)
	update := model.Server{
		ID:       created.ID,
		Name:     "renamed-server",
		Host:     created.Host,
		Port:     created.Port,
		AuthType: created.AuthType,
		Username: created.Username,
		Password: "", // empty = keep existing
	}

	updated, err := s.Update(ctx, update)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Name != "renamed-server" {
		t.Errorf("Name = %q, want %q", updated.Name, "renamed-server")
	}

	// Original password should be preserved
	if updated.Password != "s3cret!" {
		t.Errorf("Password = %q, want original password preserved", updated.Password)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	srv := testServer()
	srv.ID = "nonexistent-id"

	_, err := s.Update(ctx, srv)
	if err == nil {
		t.Fatal("Update should fail for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, testServer())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = s.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not be found after deletion
	_, err = s.GetByID(ctx, created.ID)
	if err == nil {
		t.Fatal("server should not exist after deletion")
	}

	// List should be empty
	servers, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers after delete, got %d", len(servers))
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Delete(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("Delete should fail for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestCreateWithKeyAuth(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	srv := model.Server{
		Name:     "key-server",
		Host:     "10.0.0.1",
		Port:     2222,
		AuthType: model.AuthTypeKey,
		Username: "deploy",
		KeyPath:  "/home/deploy/.ssh/id_ed25519",
	}

	created, err := s.Create(ctx, srv)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.AuthType != model.AuthTypeKey {
		t.Errorf("AuthType = %q, want %q", got.AuthType, model.AuthTypeKey)
	}
	if got.KeyPath != srv.KeyPath {
		t.Errorf("KeyPath = %q, want %q", got.KeyPath, srv.KeyPath)
	}
	if got.Password != "" {
		t.Errorf("Password should be empty for key auth, got %q", got.Password)
	}
}

func TestCreateWithHostKeyFingerprint(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	srv := testServer()
	srv.HostKeyFingerprint = "SHA256:abcdef1234567890"

	created, err := s.Create(ctx, srv)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.HostKeyFingerprint != "SHA256:abcdef1234567890" {
		t.Errorf("HostKeyFingerprint = %q, want %q", got.HostKeyFingerprint, "SHA256:abcdef1234567890")
	}
}

func TestPasswordEncryptedAtRest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	input := testServer()
	created, err := s.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Query the raw encrypted_password column directly
	var encPassword string
	err = s.db.QueryRowContext(ctx,
		"SELECT encrypted_password FROM servers WHERE id = ?", created.ID,
	).Scan(&encPassword)
	if err != nil {
		t.Fatalf("query encrypted_password: %v", err)
	}

	// The stored value should not be the plaintext password
	if encPassword == input.Password {
		t.Error("password should be encrypted at rest, not stored as plaintext")
	}

	// It should be non-empty (encrypted)
	if encPassword == "" {
		t.Error("encrypted_password should not be empty for a password-auth server")
	}
}
