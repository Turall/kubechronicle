package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/model"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

// mockStore is a mock implementation of store.Store for testing
type mockStore struct {
	savedEvents []*model.ChangeEvent
	closeCalled bool
	saveError   error
}

func (m *mockStore) Save(event *model.ChangeEvent) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.savedEvents = append(m.savedEvents, event)
	return nil
}

func (m *mockStore) Close() error {
	m.closeCalled = true
	return nil
}

func (m *mockStore) QueryEvents(ctx context.Context, filters store.QueryFilters, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	// Simple mock implementation - return all saved events
	events := make([]*model.ChangeEvent, len(m.savedEvents))
	copy(events, m.savedEvents)
	return &store.QueryResult{
		Events: events,
		Total:  len(events),
	}, nil
}

func (m *mockStore) GetEventByID(ctx context.Context, id string) (*model.ChangeEvent, error) {
	// Simple mock implementation - search saved events
	for _, event := range m.savedEvents {
		if event.ID == id {
			return event, nil
		}
	}
	return nil, nil
}

func (m *mockStore) GetResourceHistory(ctx context.Context, kind, namespace, name string, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	// Simple mock implementation - filter saved events
	var events []*model.ChangeEvent
	for _, event := range m.savedEvents {
		if event.ResourceKind == kind && event.Namespace == namespace && event.Name == name {
			events = append(events, event)
		}
	}
	return &store.QueryResult{
		Events: events,
		Total:  len(events),
	}, nil
}

func (m *mockStore) GetUserActivity(ctx context.Context, username string, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	// Simple mock implementation - filter saved events by username
	var events []*model.ChangeEvent
	for _, event := range m.savedEvents {
		if event.Actor.Username == username {
			events = append(events, event)
		}
	}
	return &store.QueryResult{
		Events: events,
		Total:  len(events),
	}, nil
}

func TestNewHandler(t *testing.T) {
	store := &mockStore{}
	handler := NewHandler(store, nil, nil, nil)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if handler.decoder == nil {
		t.Error("Handler decoder should not be nil")
	}
	if handler.store != store {
		t.Error("Handler store should be set")
	}
	if handler.queue == nil {
		t.Error("Handler queue should not be nil")
	}
}

func TestNewHandler_NilStore(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if handler.store != nil {
		t.Error("Handler store should be nil")
	}
}

func TestHandler_HandleAdmissionReview_Success(t *testing.T) {
	mockStore := &mockStore{}
	handler := NewHandler(mockStore, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler.Start(ctx)

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Deployment",
			},
			Namespace: "default",
			Name:      "test-deployment",
			UserInfo: authenticationv1.UserInfo{
				Username: "user@example.com",
			},
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {"name": "test-deployment"}}`),
			},
		},
	}

	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Failed to marshal review: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Response == nil {
		t.Fatal("Response should not be nil")
	}
	if !response.Response.Allowed {
		t.Error("Response.Allowed should be true (observe-only)")
	}
	if response.Response.UID != "test-uid" {
		t.Errorf("Response.UID = %s, want test-uid", response.Response.UID)
	}
}

func TestHandler_HandleAdmissionReview_WrongMethod(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/validate", nil)
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandler_HandleAdmissionReview_InvalidJSON(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	// Should still allow (fail-open)
	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (fail-open)", w.Code, http.StatusOK)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Response.Allowed should be true (fail-open)")
	}
}

func TestHandler_HandleAdmissionReview_InvalidVersion(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1beta1", // Wrong version
		},
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
		},
	}

	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	// Should still allow (fail-open)
	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (fail-open)", w.Code, http.StatusOK)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Response.Allowed should be true (fail-open)")
	}
}

func TestHandler_ProcessEvents_WithStore(t *testing.T) {
	mockStore := &mockStore{}
	handler := NewHandler(mockStore, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	handler.Start(ctx)

	event := &model.ChangeEvent{
		ID:        "test-id",
		Operation: "CREATE",
		Name:      "test",
	}

	// Send event to queue
	select {
	case handler.queue <- event:
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to send event to queue")
	}

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	// Check if event was saved
	if len(mockStore.savedEvents) != 1 {
		t.Errorf("Expected 1 saved event, got %d", len(mockStore.savedEvents))
	}
	if mockStore.savedEvents[0].ID != "test-id" {
		t.Errorf("Saved event ID = %s, want test-id", mockStore.savedEvents[0].ID)
	}

	cancel()
	time.Sleep(50 * time.Millisecond) // Give worker time to stop
}

func TestHandler_ProcessEvents_WithoutStore(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	handler.Start(ctx)

	event := &model.ChangeEvent{
		ID:        "test-id",
		Operation: "CREATE",
	}

	// Send event to queue
	select {
	case handler.queue <- event:
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to send event to queue")
	}

	// Give worker time to process (should not crash)
	time.Sleep(100 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHandler_ProcessEvents_StoreError(t *testing.T) {
	mockStore := &mockStore{
		saveError: &mockError{message: "storage error"},
	}
	handler := NewHandler(mockStore, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	handler.Start(ctx)

	event := &model.ChangeEvent{
		ID: "test-id",
	}

	// Send event to queue
	select {
	case handler.queue <- event:
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to send event to queue")
	}

	// Give worker time to process (should handle error gracefully)
	time.Sleep(100 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHandler_ProcessEvents_ContextCancel(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	handler.Start(ctx)

	// Cancel context
	cancel()

	// Give worker time to stop
	time.Sleep(100 * time.Millisecond)

	// Worker should have stopped, so sending to queue should block or fail
	// This is expected behavior
}

func TestGenerateEventID(t *testing.T) {
	event := &model.ChangeEvent{
		Operation:    "CREATE",
		ResourceKind: "Deployment",
		Name:         "test",
		Timestamp:    time.Now(),
	}

	id1 := generateEventID(event)
	if id1 == "" {
		t.Error("generateEventID() returned empty string")
	}

	// Same event should produce same ID (if timestamp is same)
	event2 := &model.ChangeEvent{
		Operation:    "CREATE",
		ResourceKind: "Deployment",
		Name:         "test",
		Timestamp:    event.Timestamp,
	}
	id2 := generateEventID(event2)
	if id1 != id2 {
		t.Error("Same event should produce same ID")
	}

	// Different operation should produce different ID
	event3 := &model.ChangeEvent{
		Operation:    "UPDATE",
		ResourceKind: "Deployment",
		Name:         "test",
		Timestamp:    event.Timestamp,
	}
	id3 := generateEventID(event3)
	if id1 == id3 {
		t.Error("Different operations should produce different IDs")
	}
}

// mockError is a simple error implementation for testing
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

func TestHandler_HandleAdmissionReview_EmptyBody(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(nil))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	// Should still allow (fail-open)
	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (fail-open)", w.Code, http.StatusOK)
	}
}

func TestHandler_HandleAdmissionReview_QueueFull(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	// Fill the queue
	for i := 0; i < 1001; i++ {
		select {
		case handler.queue <- &model.ChangeEvent{ID: "test"}:
		default:
			// Queue is full
		}
	}

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Deployment",
			},
			UserInfo: authenticationv1.UserInfo{
				Username: "user@example.com",
			},
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {"name": "test"}}`),
			},
		},
	}

	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	// Should still allow even if queue is full
	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (fail-open)", w.Code, http.StatusOK)
	}
}

func TestReadBody(t *testing.T) {
	bodyContent := []byte("test body")
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(bodyContent))

	body, err := readBody(req)
	if err != nil {
		t.Fatalf("readBody() error = %v", err)
	}
	if !bytes.Equal(body, bodyContent) {
		t.Errorf("readBody() = %v, want %v", body, bodyContent)
	}

	// Body should be readable again
	body2, err := readBody(req)
	if err != nil {
		t.Fatalf("readBody() error on second read = %v", err)
	}
	if !bytes.Equal(body2, bodyContent) {
		t.Error("Body should be readable multiple times")
	}
}

func TestReadBody_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	body, err := readBody(req)
	if err != nil {
		t.Fatalf("readBody() with nil body error = %v", err)
	}
	if len(body) != 0 {
		t.Errorf("readBody() with nil body = %v, want empty", body)
	}
}

func TestHandler_SendErrorResponse(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	w := httptest.NewRecorder()

	handler.sendErrorResponse(w, &mockError{message: "test error"})

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (fail-open)", w.Code, http.StatusOK)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Response.Allowed should be true (fail-open)")
	}
	if response.Response.Result == nil {
		t.Error("Response.Result should be set for error response")
	}
}

func TestHandler_SendResponse(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)
	w := httptest.NewRecorder()

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     "test-uid",
			Allowed: true,
		},
	}

	err := handler.sendResponse(w, review)
	if err != nil {
		t.Fatalf("sendResponse() error = %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Response.Allowed should be true")
	}
}

func TestHandler_GetIgnoreConfig(t *testing.T) {
	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*"},
		NamePatterns:     []string{"test-*"},
	}
	handler := NewHandler(nil, nil, ignoreConfig, nil)

	// Test thread-safe getter
	retrieved := handler.getIgnoreConfig()
	if retrieved == nil {
		t.Fatal("getIgnoreConfig() returned nil")
	}
	if len(retrieved.NamespacePatterns) != 1 || retrieved.NamespacePatterns[0] != "kube-*" {
		t.Errorf("getIgnoreConfig() = %v, want namespace_patterns=[kube-*]", retrieved.NamespacePatterns)
	}
	if len(retrieved.NamePatterns) != 1 || retrieved.NamePatterns[0] != "test-*" {
		t.Errorf("getIgnoreConfig() = %v, want name_patterns=[test-*]", retrieved.NamePatterns)
	}
}

func TestHandler_GetBlockConfig(t *testing.T) {
	blockConfig := &config.BlockConfig{
		NamePatterns:      []string{"block-*"},
		OperationPatterns: []string{"DELETE"},
		Message:           "Custom block message",
	}
	handler := NewHandler(nil, nil, nil, blockConfig)

	// Test thread-safe getter
	retrieved := handler.getBlockConfig()
	if retrieved == nil {
		t.Fatal("getBlockConfig() returned nil")
	}
	if len(retrieved.NamePatterns) != 1 || retrieved.NamePatterns[0] != "block-*" {
		t.Errorf("getBlockConfig() = %v, want name_patterns=[block-*]", retrieved.NamePatterns)
	}
	if len(retrieved.OperationPatterns) != 1 || retrieved.OperationPatterns[0] != "DELETE" {
		t.Errorf("getBlockConfig() = %v, want operation_patterns=[DELETE]", retrieved.OperationPatterns)
	}
	if retrieved.Message != "Custom block message" {
		t.Errorf("getBlockConfig() message = %q, want %q", retrieved.Message, "Custom block message")
	}
}

func TestHandler_ReloadConfig_FromFiles(t *testing.T) {
	// Create temporary directory for config files
	tmpDir, err := os.MkdirTemp("", "kubechronicle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial config files
	ignoreConfigPath := filepath.Join(tmpDir, "IGNORE_CONFIG")
	blockConfigPath := filepath.Join(tmpDir, "BLOCK_CONFIG")

	initialIgnoreConfig := config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*"},
		NamePatterns:      []string{"test-*"},
	}
	initialBlockConfig := config.BlockConfig{
		NamePatterns:      []string{"block-*"},
		OperationPatterns: []string{"DELETE"},
		Message:           "Initial message",
	}

	ignoreJSON, _ := json.Marshal(initialIgnoreConfig)
	blockJSON, _ := json.Marshal(initialBlockConfig)

	if err := os.WriteFile(ignoreConfigPath, ignoreJSON, 0644); err != nil {
		t.Fatalf("Failed to write ignore config: %v", err)
	}
	if err := os.WriteFile(blockConfigPath, blockJSON, 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	// Create handler with config path
	handler := NewHandler(nil, nil, nil, nil)
	handler.configPath = tmpDir

	// Reload config
	handler.reloadConfig()

	// Verify config was loaded
	ignoreConfig := handler.getIgnoreConfig()
	if ignoreConfig == nil {
		t.Fatal("Ignore config should be loaded")
	}
	if len(ignoreConfig.NamespacePatterns) != 1 || ignoreConfig.NamespacePatterns[0] != "kube-*" {
		t.Errorf("Reloaded ignore config namespace_patterns = %v, want [kube-*]", ignoreConfig.NamespacePatterns)
	}

	blockConfig := handler.getBlockConfig()
	if blockConfig == nil {
		t.Fatal("Block config should be loaded")
	}
	if len(blockConfig.NamePatterns) != 1 || blockConfig.NamePatterns[0] != "block-*" {
		t.Errorf("Reloaded block config name_patterns = %v, want [block-*]", blockConfig.NamePatterns)
	}
	if blockConfig.Message != "Initial message" {
		t.Errorf("Reloaded block config message = %q, want %q", blockConfig.Message, "Initial message")
	}

	// Update config files
	updatedIgnoreConfig := config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*", "cert-manager"},
		NamePatterns:      []string{"test-*", "updated-*"},
	}
	updatedBlockConfig := config.BlockConfig{
		NamePatterns:      []string{"block-*", "critical-*"},
		OperationPatterns: []string{"DELETE", "UPDATE"},
		Message:           "Updated message",
	}

	updatedIgnoreJSON, _ := json.Marshal(updatedIgnoreConfig)
	updatedBlockJSON, _ := json.Marshal(updatedBlockConfig)

	if err := os.WriteFile(ignoreConfigPath, updatedIgnoreJSON, 0644); err != nil {
		t.Fatalf("Failed to write updated ignore config: %v", err)
	}
	if err := os.WriteFile(blockConfigPath, updatedBlockJSON, 0644); err != nil {
		t.Fatalf("Failed to write updated block config: %v", err)
	}

	// Reload again
	handler.reloadConfig()

	// Verify updated config
	ignoreConfig = handler.getIgnoreConfig()
	if len(ignoreConfig.NamespacePatterns) != 2 {
		t.Errorf("Updated ignore config namespace_patterns = %v, want 2 items", ignoreConfig.NamespacePatterns)
	}
	if len(ignoreConfig.NamePatterns) != 2 {
		t.Errorf("Updated ignore config name_patterns = %v, want 2 items", ignoreConfig.NamePatterns)
	}

	blockConfig = handler.getBlockConfig()
	if len(blockConfig.NamePatterns) != 2 {
		t.Errorf("Updated block config name_patterns = %v, want 2 items", blockConfig.NamePatterns)
	}
	if len(blockConfig.OperationPatterns) != 2 {
		t.Errorf("Updated block config operation_patterns = %v, want 2 items", blockConfig.OperationPatterns)
	}
	if blockConfig.Message != "Updated message" {
		t.Errorf("Updated block config message = %q, want %q", blockConfig.Message, "Updated message")
	}
}

func TestHandler_ReloadConfig_MissingFiles(t *testing.T) {
	// Create temporary directory without config files
	tmpDir, err := os.MkdirTemp("", "kubechronicle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create handler with initial config
	initialIgnoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"initial-*"},
	}
	initialBlockConfig := &config.BlockConfig{
		NamePatterns: []string{"initial-*"},
		Message:      "Initial",
	}

	handler := NewHandler(nil, nil, initialIgnoreConfig, initialBlockConfig)
	handler.configPath = tmpDir

	// Reload config (files don't exist)
	handler.reloadConfig()

	// Config should remain unchanged (not nil)
	ignoreConfig := handler.getIgnoreConfig()
	if ignoreConfig == nil {
		t.Fatal("Ignore config should not be nil (should keep initial config)")
	}
	if len(ignoreConfig.NamespacePatterns) != 1 || ignoreConfig.NamespacePatterns[0] != "initial-*" {
		t.Errorf("Ignore config should remain unchanged: %v", ignoreConfig.NamespacePatterns)
	}

	blockConfig := handler.getBlockConfig()
	if blockConfig == nil {
		t.Fatal("Block config should not be nil (should keep initial config)")
	}
	if len(blockConfig.NamePatterns) != 1 || blockConfig.NamePatterns[0] != "initial-*" {
		t.Errorf("Block config should remain unchanged: %v", blockConfig.NamePatterns)
	}
}

func TestHandler_ReloadConfig_InvalidJSON(t *testing.T) {
	// Create temporary directory with invalid JSON files
	tmpDir, err := os.MkdirTemp("", "kubechronicle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ignoreConfigPath := filepath.Join(tmpDir, "IGNORE_CONFIG")
	blockConfigPath := filepath.Join(tmpDir, "BLOCK_CONFIG")

	// Write invalid JSON
	if err := os.WriteFile(ignoreConfigPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid ignore config: %v", err)
	}
	if err := os.WriteFile(blockConfigPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid block config: %v", err)
	}

	// Create handler with initial config
	initialIgnoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"initial-*"},
	}
	initialBlockConfig := &config.BlockConfig{
		NamePatterns: []string{"initial-*"},
	}

	handler := NewHandler(nil, nil, initialIgnoreConfig, initialBlockConfig)
	handler.configPath = tmpDir

	// Reload config (should fail to parse but not crash)
	handler.reloadConfig()

	// Config should remain unchanged
	ignoreConfig := handler.getIgnoreConfig()
	if ignoreConfig == nil || len(ignoreConfig.NamespacePatterns) != 1 {
		t.Error("Ignore config should remain unchanged after invalid JSON")
	}

	blockConfig := handler.getBlockConfig()
	if blockConfig == nil || len(blockConfig.NamePatterns) != 1 {
		t.Error("Block config should remain unchanged after invalid JSON")
	}
}

func TestHandler_ReloadConfig_EmptyBlockMessage(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "kubechronicle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	blockConfigPath := filepath.Join(tmpDir, "BLOCK_CONFIG")

	// Create block config without message
	blockConfig := config.BlockConfig{
		NamePatterns: []string{"test-*"},
	}
	blockJSON, _ := json.Marshal(blockConfig)

	if err := os.WriteFile(blockConfigPath, blockJSON, 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	handler := NewHandler(nil, nil, nil, nil)
	handler.configPath = tmpDir

	// Reload config
	handler.reloadConfig()

	// Verify default message was set
	reloaded := handler.getBlockConfig()
	if reloaded == nil {
		t.Fatal("Block config should be loaded")
	}
	if reloaded.Message == "" {
		t.Error("Block config message should have default value")
	}
	if reloaded.Message != "Resource blocked by kubechronicle policy" {
		t.Errorf("Block config message = %q, want default message", reloaded.Message)
	}
}

func TestHandler_ReloadConfigPeriodically(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "kubechronicle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ignoreConfigPath := filepath.Join(tmpDir, "IGNORE_CONFIG")
	blockConfigPath := filepath.Join(tmpDir, "BLOCK_CONFIG")

	// Create initial config files
	initialIgnoreConfig := config.IgnoreConfig{
		NamespacePatterns: []string{"initial-*"},
	}
	initialBlockConfig := config.BlockConfig{
		NamePatterns: []string{"initial-*"},
	}

	ignoreJSON, _ := json.Marshal(initialIgnoreConfig)
	blockJSON, _ := json.Marshal(initialBlockConfig)

	if err := os.WriteFile(ignoreConfigPath, ignoreJSON, 0644); err != nil {
		t.Fatalf("Failed to write ignore config: %v", err)
	}
	if err := os.WriteFile(blockConfigPath, blockJSON, 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	handler := NewHandler(nil, nil, nil, nil)
	handler.configPath = tmpDir

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start periodic reloader with shorter interval for testing
	// We'll manually trigger reloads instead of waiting 30 seconds
	go handler.reloadConfigPeriodically(ctx)

	// Initial reload
	handler.reloadConfig()

	// Verify initial config
	ignoreConfig := handler.getIgnoreConfig()
	if ignoreConfig == nil || len(ignoreConfig.NamespacePatterns) != 1 {
		t.Error("Initial ignore config should be loaded")
	}

	// Update config files
	updatedIgnoreConfig := config.IgnoreConfig{
		NamespacePatterns: []string{"updated-*"},
	}
	updatedIgnoreJSON, _ := json.Marshal(updatedIgnoreConfig)
	if err := os.WriteFile(ignoreConfigPath, updatedIgnoreJSON, 0644); err != nil {
		t.Fatalf("Failed to write updated ignore config: %v", err)
	}

	// Manually trigger reload (simulating periodic reload)
	handler.reloadConfig()

	// Verify updated config
	ignoreConfig = handler.getIgnoreConfig()
	if ignoreConfig == nil || len(ignoreConfig.NamespacePatterns) != 1 || ignoreConfig.NamespacePatterns[0] != "updated-*" {
		t.Errorf("Updated ignore config = %v, want [updated-*]", ignoreConfig.NamespacePatterns)
	}

	// Cancel context to stop reloader
	cancel()
	time.Sleep(50 * time.Millisecond) // Give goroutine time to exit
}

func TestHandler_GetConfig_ThreadSafety(t *testing.T) {
	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"test-*"},
	}
	blockConfig := &config.BlockConfig{
		NamePatterns: []string{"block-*"},
	}

	handler := NewHandler(nil, nil, ignoreConfig, blockConfig)

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = handler.getIgnoreConfig()
				_ = handler.getBlockConfig()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify config is still accessible
	ignoreConfig = handler.getIgnoreConfig()
	if ignoreConfig == nil {
		t.Fatal("Ignore config should still be accessible after concurrent access")
	}

	blockConfig = handler.getBlockConfig()
	if blockConfig == nil {
		t.Fatal("Block config should still be accessible after concurrent access")
	}
}
