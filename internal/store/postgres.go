package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// PostgreSQLStore implements the Store interface using PostgreSQL.
type PostgreSQLStore struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLStore creates a new PostgreSQL store and initializes the database schema.
func NewPostgreSQLStore(connectionString string) (*PostgreSQLStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgreSQLStore{pool: pool}

	// Initialize schema
	if err := store.initSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	klog.Info("PostgreSQL store initialized successfully")
	return store, nil
}

// initSchema creates the change_events table if it doesn't exist.
func (s *PostgreSQLStore) initSchema(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS change_events (
		id VARCHAR(255) PRIMARY KEY,
		timestamp TIMESTAMPTZ NOT NULL,
		operation VARCHAR(50) NOT NULL,
		resource_kind VARCHAR(100) NOT NULL,
		namespace VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		actor JSONB NOT NULL,
		source JSONB NOT NULL,
		diff JSONB,
		object_snapshot JSONB,
		allowed BOOLEAN NOT NULL DEFAULT true,
		block_pattern VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	-- Create indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_change_events_timestamp ON change_events(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_change_events_resource ON change_events(resource_kind, namespace, name);
	CREATE INDEX IF NOT EXISTS idx_change_events_operation ON change_events(operation);
	CREATE INDEX IF NOT EXISTS idx_change_events_actor_username ON change_events((actor->>'username'));
	CREATE INDEX IF NOT EXISTS idx_change_events_source_tool ON change_events((source->>'tool'));
	CREATE INDEX IF NOT EXISTS idx_change_events_allowed ON change_events(allowed);
	CREATE INDEX IF NOT EXISTS idx_change_events_block_pattern ON change_events(block_pattern) WHERE block_pattern IS NOT NULL;

	-- GIN index for JSONB fields to enable efficient queries
	CREATE INDEX IF NOT EXISTS idx_change_events_actor_gin ON change_events USING GIN (actor);
	CREATE INDEX IF NOT EXISTS idx_change_events_source_gin ON change_events USING GIN (source);
	`

	_, err := s.pool.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Run migrations to add new columns to existing tables
	// Add allowed column if it doesn't exist
	migrateAllowedSQL := `
	DO $$ 
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
		               WHERE table_name='change_events' AND column_name='allowed') THEN
			ALTER TABLE change_events ADD COLUMN allowed BOOLEAN NOT NULL DEFAULT true;
		END IF;
	END $$;
	`
	_, err = s.pool.Exec(ctx, migrateAllowedSQL)
	if err != nil {
		return fmt.Errorf("failed to migrate allowed column: %w", err)
	}

	// Add block_pattern column if it doesn't exist
	migrateBlockPatternSQL := `
	DO $$ 
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
		               WHERE table_name='change_events' AND column_name='block_pattern') THEN
			ALTER TABLE change_events ADD COLUMN block_pattern VARCHAR(255);
		END IF;
	END $$;
	`
	_, err = s.pool.Exec(ctx, migrateBlockPatternSQL)
	if err != nil {
		return fmt.Errorf("failed to migrate block_pattern column: %w", err)
	}

	// Create indexes if they don't exist (after columns are added)
	indexSQL := `
	CREATE INDEX IF NOT EXISTS idx_change_events_allowed ON change_events(allowed);
	CREATE INDEX IF NOT EXISTS idx_change_events_block_pattern ON change_events(block_pattern) WHERE block_pattern IS NOT NULL;
	`
	_, err = s.pool.Exec(ctx, indexSQL)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	klog.V(2).Info("Database schema initialized")
	return nil
}

// Save persists a change event to the database.
func (s *PostgreSQLStore) Save(event *model.ChangeEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Marshal JSONB fields
	actorJSON, err := json.Marshal(event.Actor)
	if err != nil {
		return fmt.Errorf("failed to marshal actor: %w", err)
	}

	sourceJSON, err := json.Marshal(event.Source)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}

	var diffJSON []byte
	if len(event.Diff) > 0 {
		diffJSON, err = json.Marshal(event.Diff)
		if err != nil {
			return fmt.Errorf("failed to marshal diff: %w", err)
		}
	}

	var snapshotJSON []byte
	if event.ObjectSnapshot != nil {
		snapshotJSON, err = json.Marshal(event.ObjectSnapshot)
		if err != nil {
			return fmt.Errorf("failed to marshal object snapshot: %w", err)
		}
	}

	insertSQL := `
		INSERT INTO change_events (
			id, timestamp, operation, resource_kind, namespace, name,
			actor, source, diff, object_snapshot, allowed, block_pattern
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (id) DO NOTHING
	`

	// Set default values if not set
	allowed := event.Allowed
	blockPattern := event.BlockPattern

	_, err = s.pool.Exec(ctx, insertSQL,
		event.ID,
		event.Timestamp,
		event.Operation,
		event.ResourceKind,
		event.Namespace,
		event.Name,
		actorJSON,
		sourceJSON,
		diffJSON,
		snapshotJSON,
		allowed,
		blockPattern,
	)

	if err != nil {
		return fmt.Errorf("failed to insert change event: %w", err)
	}

	return nil
}

// Close closes the database connection pool.
func (s *PostgreSQLStore) Close() error {
	if s.pool != nil {
		s.pool.Close()
		klog.Info("PostgreSQL store closed")
	}
	return nil
}

// HealthCheck verifies the database connection is healthy.
func (s *PostgreSQLStore) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// QueryEvents queries change events with filters, pagination, and sorting.
func (s *PostgreSQLStore) QueryEvents(ctx context.Context, filters QueryFilters, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error) {
	// Build WHERE clause
	whereClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if filters.ResourceKind != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("resource_kind = $%d", argIdx))
		args = append(args, filters.ResourceKind)
		argIdx++
	}

	if filters.Namespace != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("namespace = $%d", argIdx))
		args = append(args, filters.Namespace)
		argIdx++
	}

	if filters.Name != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, filters.Name)
		argIdx++
	}

	if filters.Username != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("actor->>'username' = $%d", argIdx))
		args = append(args, filters.Username)
		argIdx++
	}

	if filters.Operation != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("operation = $%d", argIdx))
		args = append(args, filters.Operation)
		argIdx++
	}

	if filters.StartTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp >= $%d", argIdx))
		args = append(args, *filters.StartTime)
		argIdx++
	}

	if filters.EndTime != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp <= $%d", argIdx))
		args = append(args, *filters.EndTime)
		argIdx++
	}

	if filters.Allowed != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("allowed = $%d", argIdx))
		args = append(args, *filters.Allowed)
		argIdx++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Determine sort order
	orderSQL := "DESC"
	if sortOrder == SortOrderAsc {
		orderSQL = "ASC"
	}

	// Count total matching records
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM change_events %s", whereSQL)
	var total int
	err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	// Query events with pagination
	limit := pagination.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}

	querySQL := fmt.Sprintf(`
		SELECT id, timestamp, operation, resource_kind, namespace, name,
		       actor, source, diff, object_snapshot, allowed, block_pattern
		FROM change_events
		%s
		ORDER BY timestamp %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderSQL, argIdx, argIdx+1)

	args = append(args, limit, pagination.Offset)

	rows, err := s.pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	events := []*model.ChangeEvent{}
	for rows.Next() {
		event, err := s.scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &QueryResult{
		Events: events,
		Total:  total,
	}, nil
}

// GetEventByID retrieves a single change event by ID.
func (s *PostgreSQLStore) GetEventByID(ctx context.Context, id string) (*model.ChangeEvent, error) {
	querySQL := `
		SELECT id, timestamp, operation, resource_kind, namespace, name,
		       actor, source, diff, object_snapshot, allowed, block_pattern
		FROM change_events
		WHERE id = $1
	`

	row := s.pool.QueryRow(ctx, querySQL, id)
	event, err := s.scanEventRow(row)
	if err != nil {
		return nil, fmt.Errorf("failed to get event by ID: %w", err)
	}

	return event, nil
}

// GetResourceHistory retrieves the change history for a specific resource.
func (s *PostgreSQLStore) GetResourceHistory(ctx context.Context, kind, namespace, name string, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error) {
	filters := QueryFilters{
		ResourceKind: kind,
		Namespace:    namespace,
		Name:         name,
	}
	return s.QueryEvents(ctx, filters, pagination, sortOrder)
}

// GetUserActivity retrieves change events for a specific user.
func (s *PostgreSQLStore) GetUserActivity(ctx context.Context, username string, pagination PaginationParams, sortOrder SortOrder) (*QueryResult, error) {
	filters := QueryFilters{
		Username: username,
	}
	return s.QueryEvents(ctx, filters, pagination, sortOrder)
}

// scanEvent scans a single event from pgx.Rows.
func (s *PostgreSQLStore) scanEvent(rows interface {
	Scan(dest ...interface{}) error
}) (*model.ChangeEvent, error) {
	var (
		id             string
		timestamp      time.Time
		operation      string
		resourceKind   string
		namespace      string
		name           string
		actorJSON      []byte
		sourceJSON     []byte
		diffJSON       []byte
		snapshotJSON   []byte
		allowed        bool
		blockPattern   *string
	)

	err := rows.Scan(
		&id, &timestamp, &operation, &resourceKind, &namespace, &name,
		&actorJSON, &sourceJSON, &diffJSON, &snapshotJSON, &allowed, &blockPattern,
	)
	if err != nil {
		return nil, err
	}

	event := &model.ChangeEvent{
		ID:           id,
		Timestamp:    timestamp,
		Operation:    operation,
		ResourceKind: resourceKind,
		Namespace:    namespace,
		Name:         name,
		Allowed:      allowed,
	}

	if blockPattern != nil {
		event.BlockPattern = *blockPattern
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(actorJSON, &event.Actor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actor: %w", err)
	}

	if err := json.Unmarshal(sourceJSON, &event.Source); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source: %w", err)
	}

	if len(diffJSON) > 0 {
		if err := json.Unmarshal(diffJSON, &event.Diff); err != nil {
			return nil, fmt.Errorf("failed to unmarshal diff: %w", err)
		}
	}

	if len(snapshotJSON) > 0 {
		if err := json.Unmarshal(snapshotJSON, &event.ObjectSnapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object snapshot: %w", err)
		}
	}

	return event, nil
}

// scanEventRow scans a single event from pgx.Row.
func (s *PostgreSQLStore) scanEventRow(row interface {
	Scan(dest ...interface{}) error
}) (*model.ChangeEvent, error) {
	return s.scanEvent(row)
}
