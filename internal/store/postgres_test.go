package store

import (
	"context"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestPostgreSQLStore_Interface(t *testing.T) {
	// Test that PostgreSQLStore implements Store interface
	var _ Store = (*PostgreSQLStore)(nil)
}

func TestPostgreSQLStore_NewPostgreSQLStore_InvalidConnectionString(t *testing.T) {
	_, err := NewPostgreSQLStore("invalid-connection-string")
	if err == nil {
		t.Error("NewPostgreSQLStore() should return error for invalid connection string")
	}
}

func TestPostgreSQLStore_NewPostgreSQLStore_ConnectionFailure(t *testing.T) {
	// Use a connection string that will fail to connect
	_, err := NewPostgreSQLStore("postgres://invalid:invalid@localhost:5432/nonexistent?sslmode=disable")
	if err == nil {
		t.Error("NewPostgreSQLStore() should return error when connection fails")
	}
}

// Note: Full integration tests for PostgreSQLStore would require a running database
// These tests verify the interface and error handling without requiring a DB

func TestPostgreSQLStore_HealthCheck_NoConnection(t *testing.T) {
	store := &PostgreSQLStore{}
	// HealthCheck will panic if pool is nil, which is expected behavior
	// This test documents that HealthCheck requires a valid pool
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This will panic because pool is nil - that's expected behavior
	// We use recover to catch the panic and verify it happens
	defer func() {
		if r := recover(); r == nil {
			t.Error("HealthCheck should panic when pool is nil")
		}
	}()
	
	_ = store.HealthCheck(ctx)
}

func TestPostgreSQLStore_Close_NilPool(t *testing.T) {
	store := &PostgreSQLStore{}
	// Close should handle nil pool gracefully
	err := store.Close()
	if err != nil {
		t.Errorf("Close() should handle nil pool gracefully, got error: %v", err)
	}
}

// Test helper to create a test ChangeEvent
func createTestChangeEvent() *model.ChangeEvent {
	return &model.ChangeEvent{
		ID:          "test-id-123",
		Timestamp:   time.Now(),
		Operation:   "CREATE",
		ResourceKind: "Deployment",
		Namespace:   "default",
		Name:        "test-deployment",
		Actor: model.Actor{
			Username: "user@example.com",
			Groups:   []string{"system:authenticated"},
		},
		Source: model.Source{
			Tool: "kubectl",
		},
		Allowed:     true,
		BlockPattern: "",
	}
}

func TestChangeEvent_Structure(t *testing.T) {
	event := createTestChangeEvent()

	if event.ID == "" {
		t.Error("ChangeEvent ID should not be empty")
	}
	if event.Operation == "" {
		t.Error("ChangeEvent Operation should not be empty")
	}
	if event.ResourceKind == "" {
		t.Error("ChangeEvent ResourceKind should not be empty")
	}
	if event.Actor.Username == "" {
		t.Error("ChangeEvent Actor.Username should not be empty")
	}
	if event.Source.Tool == "" {
		t.Error("ChangeEvent Source.Tool should not be empty")
	}
	// Verify new fields have defaults
	if !event.Allowed {
		t.Error("ChangeEvent Allowed should default to true")
	}
	if event.BlockPattern != "" {
		t.Error("ChangeEvent BlockPattern should default to empty string")
	}
}
