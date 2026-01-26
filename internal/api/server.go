package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/model"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

// Server handles HTTP API requests for change events.
type Server struct {
	store store.Store
}

// NewServer creates a new API server.
func NewServer(store store.Store) *Server {
	return &Server{
		store: store,
	}
}

// ListChangesResponse represents the response for listing changes.
type ListChangesResponse struct {
	Events []*model.ChangeEvent `json:"events"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandleListChanges handles GET /api/changes requests.
func (s *Server) HandleListChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	filters := store.QueryFilters{}
	pagination := store.PaginationParams{
		Limit:  50, // Default limit
		Offset: 0,
	}
	sortOrder := store.SortOrderDesc // Default: newest first

	if resourceKind := r.URL.Query().Get("resource_kind"); resourceKind != "" {
		filters.ResourceKind = resourceKind
	}

	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filters.Namespace = namespace
	}

	if name := r.URL.Query().Get("name"); name != "" {
		filters.Name = name
	}

	if username := r.URL.Query().Get("user"); username != "" {
		filters.Username = username
	}

	if operation := r.URL.Query().Get("operation"); operation != "" {
		filters.Operation = operation
	}

	// Parse time range
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filters.StartTime = &startTime
		}
	}

	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filters.EndTime = &endTime
		}
	}

	// Parse allowed filter
	if allowedStr := r.URL.Query().Get("allowed"); allowedStr != "" {
		if allowed, err := strconv.ParseBool(allowedStr); err == nil {
			filters.Allowed = &allowed
		}
	}

	// Parse pagination
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			pagination.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			pagination.Offset = offset
		}
	}

	// Parse sort order
	if sort := r.URL.Query().Get("sort"); sort != "" {
		if sort == "asc" {
			sortOrder = store.SortOrderAsc
		}
	}

	// Query events
	ctx := r.Context()
	result, err := s.store.QueryEvents(ctx, filters, pagination, sortOrder)
	if err != nil {
		klog.Errorf("Failed to query events: %v", err)
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query events: %v", err))
		return
	}

	// Send response
	response := ListChangesResponse{
		Events: result.Events,
		Total:  result.Total,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
	}

	s.sendJSON(w, http.StatusOK, response)
}

// HandleGetChange handles GET /api/changes/{id} requests.
func (s *Server) HandleGetChange(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/changes/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/changes/")
	if path == "" || strings.Contains(path, "/") {
		s.sendError(w, http.StatusBadRequest, "Missing or invalid change ID")
		return
	}

	id, err := url.PathUnescape(path)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid change ID: %v", err))
		return
	}

	// Get event by ID
	ctx := r.Context()
	event, err := s.store.GetEventByID(ctx, id)
	if err != nil {
		klog.Errorf("Failed to get event by ID: %v", err)
		s.sendError(w, http.StatusNotFound, fmt.Sprintf("Change event not found: %v", err))
		return
	}

	s.sendJSON(w, http.StatusOK, event)
}

// HandleResourceHistory handles GET /api/resources/{kind}/{namespace}/{name}/history requests.
func (s *Server) HandleResourceHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract path: /api/resources/{kind}/{namespace}/{name}/history
	path := strings.TrimPrefix(r.URL.Path, "/api/resources/")
	if !strings.HasSuffix(path, "/history") {
		s.sendError(w, http.StatusBadRequest, "Invalid resource path. Expected: /api/resources/{kind}/{namespace}/{name}/history")
		return
	}

	path = strings.TrimSuffix(path, "/history")
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 3 {
		s.sendError(w, http.StatusBadRequest, "Invalid resource path. Expected: /api/resources/{kind}/{namespace}/{name}/history")
		return
	}

	kind, err1 := url.PathUnescape(pathParts[0])
	namespace, err2 := url.PathUnescape(pathParts[1])
	name, err3 := url.PathUnescape(pathParts[2])

	if err1 != nil || err2 != nil || err3 != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid URL encoding in resource path")
		return
	}

	// Parse pagination
	pagination := store.PaginationParams{
		Limit:  50,
		Offset: 0,
	}
	sortOrder := store.SortOrderDesc

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			pagination.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			pagination.Offset = offset
		}
	}

	if sort := r.URL.Query().Get("sort"); sort == "asc" {
		sortOrder = store.SortOrderAsc
	}

	// Get resource history
	ctx := r.Context()
	result, err := s.store.GetResourceHistory(ctx, kind, namespace, name, pagination, sortOrder)
	if err != nil {
		klog.Errorf("Failed to get resource history: %v", err)
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get resource history: %v", err))
		return
	}

	response := ListChangesResponse{
		Events: result.Events,
		Total:  result.Total,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
	}

	s.sendJSON(w, http.StatusOK, response)
}

// HandleUserActivity handles GET /api/users/{username}/activity requests.
func (s *Server) HandleUserActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract username from path: /api/users/{username}/activity
	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	if !strings.HasSuffix(path, "/activity") {
		s.sendError(w, http.StatusBadRequest, "Invalid user path. Expected: /api/users/{username}/activity")
		return
	}

	path = strings.TrimSuffix(path, "/activity")
	if path == "" {
		s.sendError(w, http.StatusBadRequest, "Missing username")
		return
	}

	username, err := url.PathUnescape(path)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid username encoding: %v", err))
		return
	}

	// Parse pagination
	pagination := store.PaginationParams{
		Limit:  50,
		Offset: 0,
	}
	sortOrder := store.SortOrderDesc

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			pagination.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			pagination.Offset = offset
		}
	}

	if sort := r.URL.Query().Get("sort"); sort == "asc" {
		sortOrder = store.SortOrderAsc
	}

	// Get user activity
	ctx := r.Context()
	result, err := s.store.GetUserActivity(ctx, username, pagination, sortOrder)
	if err != nil {
		klog.Errorf("Failed to get user activity: %v", err)
		s.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get user activity: %v", err))
		return
	}

	response := ListChangesResponse{
		Events: result.Events,
		Total:  result.Total,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
	}

	s.sendJSON(w, http.StatusOK, response)
}

// sendJSON sends a JSON response.
func (s *Server) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.Errorf("Failed to encode JSON response: %v", err)
	}
}

// handleOptions handles CORS preflight requests.
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

// sendError sends an error response.
func (s *Server) sendError(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Error: message,
	}
	s.sendJSON(w, statusCode, response)
}
