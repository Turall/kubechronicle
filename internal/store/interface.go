package store

import (
	"context"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// QueryFilters represents filters for querying change events.
type QueryFilters struct {
	ResourceKind string
	Namespace    string
	Name         string
	Username     string
	Operation    string
	StartTime    *time.Time
	EndTime      *time.Time
	Allowed      *bool // nil = all, true = allowed only, false = blocked only
}

// PaginationParams represents pagination parameters.
type PaginationParams struct {
	Limit  int // Number of results per page
	Offset int // Offset for pagination
}

// SortOrder represents sort order.
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// QueryResult represents a paginated query result.
type QueryResult struct {
	Events []*model.ChangeEvent
	Total  int // Total number of events matching the query (before pagination)
}

// Store defines the interface for persisting and querying change events.
type Store interface {
	// Save persists a change event.
	Save(event *model.ChangeEvent) error
	
	// Close closes the store connection.
	Close() error
	
	// QueryEvents queries change events with filters, pagination, and sorting.
	QueryEvents(ctx context.Context, filters QueryFilters, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error)
	
	// GetEventByID retrieves a single change event by ID.
	GetEventByID(ctx context.Context, id string) (*model.ChangeEvent, error)
	
	// GetResourceHistory retrieves the change history for a specific resource.
	GetResourceHistory(ctx context.Context, kind, namespace, name string, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error)
	
	// GetUserActivity retrieves change events for a specific user.
	GetUserActivity(ctx context.Context, username string, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error)
}
