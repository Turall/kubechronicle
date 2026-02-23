-- kubechronicle database schema
-- This schema is automatically created by the application on startup.
-- You can also run this manually to initialize the database.

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
	exec_metadata JSONB,
	created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_change_events_timestamp ON change_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_change_events_resource ON change_events(resource_kind, namespace, name);
CREATE INDEX IF NOT EXISTS idx_change_events_operation ON change_events(operation);
CREATE INDEX IF NOT EXISTS idx_change_events_actor_username ON change_events((actor->>'username'));
CREATE INDEX IF NOT EXISTS idx_change_events_source_tool ON change_events((source->>'tool'));
CREATE INDEX IF NOT EXISTS idx_change_events_allowed ON change_events(allowed);
CREATE INDEX IF NOT EXISTS idx_change_events_block_pattern ON change_events(block_pattern) WHERE block_pattern IS NOT NULL;

-- GIN indexes for JSONB fields to enable efficient queries
CREATE INDEX IF NOT EXISTS idx_change_events_actor_gin ON change_events USING GIN (actor);
CREATE INDEX IF NOT EXISTS idx_change_events_source_gin ON change_events USING GIN (source);
CREATE INDEX IF NOT EXISTS idx_change_events_exec_metadata_gin ON change_events USING GIN (exec_metadata) WHERE exec_metadata IS NOT NULL;

-- Example queries:
-- 
-- Get all changes for a specific resource:
-- SELECT * FROM change_events 
-- WHERE resource_kind = 'Deployment' AND namespace = 'default' AND name = 'my-app'
-- ORDER BY timestamp DESC;
--
-- Get all changes by a specific user:
-- SELECT * FROM change_events 
-- WHERE actor->>'username' = 'user@example.com'
-- ORDER BY timestamp DESC;
--
-- Get all changes made via kubectl:
-- SELECT * FROM change_events 
-- WHERE source->>'tool' = 'kubectl'
-- ORDER BY timestamp DESC;
--
-- Get all blocked events:
-- SELECT * FROM change_events 
-- WHERE allowed = false
-- ORDER BY timestamp DESC;
--
-- Get all blocked events by pattern:
-- SELECT * FROM change_events 
-- WHERE block_pattern = 'production'
-- ORDER BY timestamp DESC;
--
-- Get all allowed events:
-- SELECT * FROM change_events 
-- WHERE allowed = true
-- ORDER BY timestamp DESC;
--
-- Get all exec operations:
-- SELECT * FROM change_events 
-- WHERE operation = 'EXEC'
-- ORDER BY timestamp DESC;
--
-- Get exec operations for a specific pod:
-- SELECT * FROM change_events 
-- WHERE operation = 'EXEC' 
--   AND resource_kind = 'Pod' 
--   AND namespace = 'default' 
--   AND name = 'my-pod'
-- ORDER BY timestamp DESC;
--
-- Get exec operations by a specific user:
-- SELECT * FROM change_events 
-- WHERE operation = 'EXEC' 
--   AND actor->>'username' = 'user@example.com'
-- ORDER BY timestamp DESC;