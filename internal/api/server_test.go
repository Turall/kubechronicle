package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

// mockStore implements store.Store for handler testing without a real DB.
type mockStore struct {
	lastFilters    store.QueryFilters
	lastPagination store.PaginationParams
	lastSort       store.SortOrder

	queryResult     *store.QueryResult
	queryErr        error
	eventByID       *model.ChangeEvent
	eventByIDErr    error
	resourceHistory *store.QueryResult
	resourceHistErr error
	userActivity    *store.QueryResult
	userActivityErr error
}

func (m *mockStore) Save(event *model.ChangeEvent) error { return nil }
func (m *mockStore) Close() error                        { return nil }

func (m *mockStore) QueryEvents(ctx context.Context, filters store.QueryFilters, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	m.lastFilters = filters
	m.lastPagination = pagination
	m.lastSort = sortOrder
	return m.queryResult, m.queryErr
}

func (m *mockStore) GetEventByID(ctx context.Context, id string) (*model.ChangeEvent, error) {
	return m.eventByID, m.eventByIDErr
}

func (m *mockStore) GetResourceHistory(ctx context.Context, kind, namespace, name string, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	m.lastFilters = store.QueryFilters{ResourceKind: kind, Namespace: namespace, Name: name}
	m.lastPagination = pagination
	m.lastSort = sortOrder
	return m.resourceHistory, m.resourceHistErr
}

func (m *mockStore) GetUserActivity(ctx context.Context, username string, pagination store.PaginationParams, sortOrder store.SortOrder) (*store.QueryResult, error) {
	m.lastFilters = store.QueryFilters{Username: username}
	m.lastPagination = pagination
	m.lastSort = sortOrder
	return m.userActivity, m.userActivityErr
}

func sampleEvent() *model.ChangeEvent {
	return &model.ChangeEvent{
		ID:           "CREATE-Deployment-my-app-123",
		Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Operation:    "CREATE",
		ResourceKind: "Deployment",
		Namespace:    "default",
		Name:         "my-app",
		Actor: model.Actor{
			Username: "user@example.com",
			Groups:   []string{"system:authenticated"},
		},
		Source: model.Source{
			Tool: "kubectl",
		},
		Allowed: true,
	}
}

func decodeResponse[T any](t *testing.T, resp *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return out
}

func TestHandleListChanges_Success(t *testing.T) {
	mock := &mockStore{
		queryResult: &store.QueryResult{
			Events: []*model.ChangeEvent{sampleEvent()},
			Total:  1,
		},
	}
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/changes?resource_kind=Deployment&namespace=default&name=my-app&user=user@example.com&operation=CREATE&start_time=2024-01-01T00:00:00Z&end_time=2024-01-02T00:00:00Z&allowed=true&limit=10&offset=5&sort=asc", nil)
	rec := httptest.NewRecorder()

	server.HandleListChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Validate response body
	resp := decodeResponse[ListChangesResponse](t, rec)
	if resp.Total != 1 || len(resp.Events) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// Validate filters were passed to store
	if mock.lastFilters.ResourceKind != "Deployment" || mock.lastFilters.Namespace != "default" || mock.lastFilters.Name != "my-app" || mock.lastFilters.Username != "user@example.com" || mock.lastFilters.Operation != "CREATE" {
		t.Fatalf("unexpected filters: %+v", mock.lastFilters)
	}
	if mock.lastPagination.Limit != 10 || mock.lastPagination.Offset != 5 {
		t.Fatalf("unexpected pagination: %+v", mock.lastPagination)
	}
	if mock.lastSort != store.SortOrderAsc {
		t.Fatalf("unexpected sort order: %s", mock.lastSort)
	}
}

func TestHandleListChanges_Options(t *testing.T) {
	server := NewServer(&mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}})
	req := httptest.NewRequest(http.MethodOptions, "/api/changes", nil)
	rec := httptest.NewRecorder()

	server.HandleListChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for OPTIONS, got %d", rec.Code)
	}
}

func TestHandleListChanges_MethodNotAllowed(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/changes", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()

	server.HandleListChanges(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleGetChange_Success(t *testing.T) {
	mock := &mockStore{eventByID: sampleEvent()}
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/changes/CREATE-Deployment-my-app-123", nil)
	rec := httptest.NewRecorder()

	server.HandleGetChange(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var event model.ChangeEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if event.ID != "CREATE-Deployment-my-app-123" {
		t.Errorf("unexpected event ID: %s", event.ID)
	}
}

func TestHandleGetChange_BadRequest(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/changes/", nil)
	rec := httptest.NewRecorder()

	server.HandleGetChange(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleResourceHistory_Success(t *testing.T) {
	mock := &mockStore{
		resourceHistory: &store.QueryResult{
			Events: []*model.ChangeEvent{sampleEvent()},
			Total:  1,
		},
	}
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/my-app/history?limit=2&offset=1&sort=asc", nil)
	rec := httptest.NewRecorder()

	server.HandleResourceHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mock.lastFilters.ResourceKind != "Deployment" || mock.lastFilters.Namespace != "default" || mock.lastFilters.Name != "my-app" {
		t.Fatalf("unexpected filters: %+v", mock.lastFilters)
	}
	if mock.lastPagination.Limit != 2 || mock.lastPagination.Offset != 1 {
		t.Fatalf("unexpected pagination: %+v", mock.lastPagination)
	}
	if mock.lastSort != store.SortOrderAsc {
		t.Fatalf("unexpected sort: %s", mock.lastSort)
	}
}

func TestHandleResourceHistory_BadPath(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/history", nil)
	rec := httptest.NewRecorder()

	server.HandleResourceHistory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUserActivity_Success(t *testing.T) {
	mock := &mockStore{
		userActivity: &store.QueryResult{
			Events: []*model.ChangeEvent{sampleEvent()},
			Total:  1,
		},
	}
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/users/user%40example.com/activity?limit=1&offset=0&sort=desc", nil)
	rec := httptest.NewRecorder()

	server.HandleUserActivity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mock.lastFilters.Username != "user@example.com" {
		t.Fatalf("unexpected username filter: %+v", mock.lastFilters)
	}
}

func TestHandleUserActivity_BadPath(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/users//activity", nil)
	rec := httptest.NewRecorder()

	server.HandleUserActivity(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleGetChange_InvalidURLEncoding(t *testing.T) {
	server := NewServer(&mockStore{})
	// Create request with valid URL first, then manually set invalid path
	req := httptest.NewRequest(http.MethodGet, "/api/changes/test-id", nil)
	// Manually set an invalid percent-encoded path that PathUnescape will fail on
	req.URL.Path = "/api/changes/%ZZ"
	rec := httptest.NewRecorder()
	server.HandleGetChange(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL encoding, got %d", rec.Code)
	}
}

func TestHandleResourceHistory_InvalidURLEncoding(t *testing.T) {
	server := NewServer(&mockStore{})
	// Create request with valid URL first, then manually set invalid path
	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/my-app/history", nil)
	// Manually set an invalid percent-encoded path that PathUnescape will fail on
	req.URL.Path = "/api/resources/%ZZ/default/my-app/history"
	rec := httptest.NewRecorder()
	server.HandleResourceHistory(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL encoding, got %d", rec.Code)
	}
}

func TestHandleUserActivity_InvalidURLEncoding(t *testing.T) {
	server := NewServer(&mockStore{})
	// Create request with valid URL first, then manually set invalid path
	req := httptest.NewRequest(http.MethodGet, "/api/users/testuser/activity", nil)
	// Manually set an invalid percent-encoded path that PathUnescape will fail on
	req.URL.Path = "/api/users/%ZZ/activity"
	rec := httptest.NewRecorder()
	server.HandleUserActivity(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL encoding, got %d", rec.Code)
	}
}

func TestHandleListChanges_ValidTimeParsing(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/changes?start_time=%s&end_time=%s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)), nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastFilters.StartTime == nil || mock.lastFilters.EndTime == nil {
		t.Error("time filters should be set")
	}
}

func TestHandleListChanges_ValidAllowedParsing(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/changes?allowed=true", nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastFilters.Allowed == nil || !*mock.lastFilters.Allowed {
		t.Error("allowed filter should be set to true")
	}
}

func TestHandleListChanges_NegativeLimit(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/changes?limit=-5", nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastPagination.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", mock.lastPagination.Limit)
	}
}

func TestHandleListChanges_ZeroLimit(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/changes?limit=0", nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastPagination.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", mock.lastPagination.Limit)
	}
}

func TestHandleResourceHistory_InvalidURLEncoding_Namespace(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/my-app/history", nil)
	req.URL.Path = "/api/resources/Deployment/%ZZ/my-app/history"
	rec := httptest.NewRecorder()
	server.HandleResourceHistory(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL encoding in namespace, got %d", rec.Code)
	}
}

func TestHandleResourceHistory_InvalidURLEncoding_Name(t *testing.T) {
	server := NewServer(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/my-app/history", nil)
	req.URL.Path = "/api/resources/Deployment/default/%ZZ/history"
	rec := httptest.NewRecorder()
	server.HandleResourceHistory(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL encoding in name, got %d", rec.Code)
	}
}

func TestHandleListChanges_InvalidAllowedFalse(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/changes?allowed=false", nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if mock.lastFilters.Allowed == nil || *mock.lastFilters.Allowed {
		t.Error("allowed filter should be set to false")
	}
}

func TestHandleListChanges_InvalidSort(t *testing.T) {
	mock := &mockStore{queryResult: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/changes?sort=invalid", nil)
	rec := httptest.NewRecorder()
	server.HandleListChanges(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Should default to desc for invalid sort
	if mock.lastSort != store.SortOrderDesc {
		t.Errorf("expected default desc sort, got %s", mock.lastSort)
	}
}

func TestHandleResourceHistory_InvalidSort(t *testing.T) {
	mock := &mockStore{resourceHistory: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/resources/Deployment/default/my-app/history?sort=invalid", nil)
	rec := httptest.NewRecorder()
	server.HandleResourceHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Should default to desc for invalid sort
	if mock.lastSort != store.SortOrderDesc {
		t.Errorf("expected default desc sort, got %s", mock.lastSort)
	}
}

func TestHandleUserActivity_InvalidSort(t *testing.T) {
	mock := &mockStore{userActivity: &store.QueryResult{Events: []*model.ChangeEvent{}, Total: 0}}
	server := NewServer(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/users/testuser/activity?sort=invalid", nil)
	rec := httptest.NewRecorder()
	server.HandleUserActivity(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Should default to desc for invalid sort
	if mock.lastSort != store.SortOrderDesc {
		t.Errorf("expected default desc sort, got %s", mock.lastSort)
	}
}
