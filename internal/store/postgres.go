package store

import (
	"context"
	"encoding/json"
	"fmt"
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
