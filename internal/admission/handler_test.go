package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubechronicle/kubechronicle/internal/model"
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
	if body != nil && len(body) != 0 {
		t.Errorf("readBody() with nil body = %v, want nil or empty", body)
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
