# Storage Layer

The storage layer provides persistence for change events.

## PostgreSQL Backend

The PostgreSQL store uses JSONB columns for flexible storage of structured data.

### Database Schema

The `change_events` table stores all Kubernetes resource change events with the following structure:

- `id`: Unique identifier (VARCHAR)
- `timestamp`: When the change occurred (TIMESTAMPTZ)
- `operation`: CREATE, UPDATE, or DELETE (VARCHAR)
- `resource_kind`: Kubernetes resource type (VARCHAR)
- `namespace`: Resource namespace (VARCHAR)
- `name`: Resource name (VARCHAR)
- `actor`: JSONB containing username, groups, serviceAccount, sourceIP
- `source`: JSONB containing tool (kubectl, helm, controller, unknown)
- `diff`: JSONB array of RFC 6902 patch operations (for UPDATE)
- `object_snapshot`: JSONB full object (for DELETE)
- `created_at`: Record creation timestamp (TIMESTAMPTZ)

### Indexes

The schema includes several indexes for efficient querying:

- Timestamp (DESC) - for chronological queries
- Resource (kind, namespace, name) - for resource history
- Operation - for filtering by operation type
- Actor username - for user activity queries
- Source tool - for tool-based filtering
- GIN indexes on JSONB fields - for efficient JSON queries

### Connection String Format

PostgreSQL connection string format:
```
postgres://user:password@host:port/database?sslmode=disable
```

Or set via environment variable:
```bash
export DATABASE_URL="postgres://user:password@localhost:5432/kubechronicle?sslmode=disable"
```

### Automatic Schema Initialization

The store automatically creates the schema on first connection if it doesn't exist. No manual migration needed.

### Idempotency

The store uses `ON CONFLICT (id) DO NOTHING` to ensure idempotent inserts. Duplicate events with the same ID are silently ignored.
