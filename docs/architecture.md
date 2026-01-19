# kubechronicle Architecture

## Overview

kubechronicle is a **Kubernetes Change Chronicle** that provides a human-readable, queryable history of Kubernetes resource changes. It tracks **who** changed **what**, **when**, and **how** resources were modified, without blocking or mutating any Kubernetes API requests.

The system is designed for:
- SRE teams conducting incident post-mortems
- Platform engineers auditing infrastructure changes
- Security teams tracking compliance and access patterns
- Teams understanding resource change timelines

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes API Server                     │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  ValidatingAdmissionWebhook (admission.k8s.io/v1)     │  │
│  └───────────────────┬───────────────────────────────────┘  │
└──────────────────────┼──────────────────────────────────────┘
                       │
                       │ HTTPS/TLS
                       │ AdmissionReview Request
                       ▼
         ┌─────────────────────────────┐
         │  kubechronicle-webhook       │
         │  (Validating Webhook)        │
         │                              │
         │  ┌────────────────────────┐  │
         │  │  Admission Handler     │  │
         │  │  - Decode Request      │  │
         │  │  - Extract Metadata    │  │
         │  │  - Compute Diff        │  │
         │  │  - Queue Event         │  │
         │  │  - Always Allow        │  │
         │  └───────────┬────────────┘  │
         │              │                │
         │  ┌───────────▼────────────┐  │
         │  │  Event Queue           │  │
         │  │  (Buffered Channel)    │  │
         │  └───────────┬────────────┘  │
         │              │                │
         │  ┌───────────▼────────────┐  │
         │  │  Async Processor       │  │
         │  │  - Dequeue Events      │  │
         │  │  - Persist to DB       │  │
         │  └───────────┬────────────┘  │
         └──────────────┼────────────────┘
                        │
                        │ Async Write
                        ▼
         ┌─────────────────────────────┐
         │      PostgreSQL Database     │
         │  - change_events table       │
         │  - JSONB for flexibility     │
         │  - Indexed for queries       │
         └─────────────────────────────┘
```

## Core Components

### 1. Admission Webhook (`internal/admission`)

The admission webhook is the entry point for observing all Kubernetes resource changes.

#### Handler (`handler.go`)
- **Type**: HTTP handler for `/validate` endpoint
- **Responsibility**: Process `AdmissionReview` requests from Kubernetes API server
- **Behavior**: 
  - Always returns `Allowed: true` (fail-open, observe-only)
  - Decodes incoming requests
  - Queues change events for async processing
  - Responds within <100ms target

#### Decoder (`decoder.go`)
- **Responsibility**: Extract relevant information from `AdmissionRequest`
- **Extracts**:
  - Operation (CREATE, UPDATE, DELETE)
  - Resource metadata (kind, namespace, name)
  - Actor information (username, groups, service account, source IP)
  - Source tool detection (kubectl, helm, controller, unknown)
  - Object snapshots (oldObject, object)
  - Computes RFC 6902 JSON Patch diffs for UPDATE operations

**Key Features**:
- Source tool detection via heuristics:
  - Helm: Checks for `app.kubernetes.io/managed-by: Helm` label
  - Controller: Detects system controllers (kube-controller-manager, kube-scheduler, service accounts)
  - kubectl: Human users (non-system usernames)
  - Unknown: Fallback for unrecognized patterns

### 2. Diff Engine (`internal/diff`)

Computes structural diffs between Kubernetes resource versions.

#### Core Functions

**`ComputeDiff(oldObj, newObj, resourceKind)`**
- Generates RFC 6902 JSON Patch operations
- Filters ignored fields (metadata noise)
- Hashes Secret values for security
- Returns array of `PatchOp` (add, remove, replace)

**`filterIgnoredFields(obj, pathPrefix)`**
- Recursively removes Kubernetes noise fields:
  - `metadata.managedFields`
  - `metadata.resourceVersion`
  - `metadata.generation`
  - `metadata.creationTimestamp`
  - `status` (entire subtree)
  - `metadata.annotations[kubectl.kubernetes.io/*]`
- Preserves only user-relevant changes

**`hashSecretValues(obj)`**
- Detects Secret resources by kind
- Hashes values in `data` and `stringData` fields
- Uses SHA-256 with hex encoding
- Format: `sha256:<hex-hash>`
- Prevents storing plaintext secrets

**`computePatchOperations(oldObj, newObj, path)`**
- Recursively compares objects
- Generates patch operations for:
  - Added fields → `{"op": "add", ...}`
  - Removed fields → `{"op": "remove", ...}`
  - Modified fields → `{"op": "replace", ...}`
- Handles arrays by replacing entire array if different
- Uses JSON Pointer paths (RFC 6901)

### 3. Storage Layer (`internal/store`)

Persists change events to PostgreSQL for querying and analysis.

#### Store Interface
```go
type Store interface {
    Save(event *model.ChangeEvent) error
    Close() error
}
```

#### PostgreSQL Implementation

**Schema** (`schema.sql`):
- `change_events` table with JSONB columns for flexible schema
- Indexes on:
  - `timestamp` (DESC) - for timeline queries
  - `(resource_kind, namespace, name)` - for resource history
  - `operation` - for filtering by operation type
  - `actor->>'username'` - for user-based queries
  - `source->>'tool'` - for tool-based analysis
  - GIN indexes on JSONB fields for efficient JSON queries

**Features**:
- Automatic schema initialization on startup
- Connection pooling via `pgxpool`
- Health check endpoint
- Graceful error handling

**Query Examples**:
```sql
-- Get all changes for a resource
SELECT * FROM change_events 
WHERE resource_kind = 'Deployment' 
  AND namespace = 'default' 
  AND name = 'my-app'
ORDER BY timestamp DESC;

-- Find changes by user
SELECT * FROM change_events 
WHERE actor->>'username' = 'user@example.com'
ORDER BY timestamp DESC;

-- Analyze tool usage
SELECT source->>'tool', COUNT(*) 
FROM change_events 
GROUP BY source->>'tool';
```

### 4. Configuration (`internal/config`)

Centralized configuration management via environment variables:

- `DATABASE_URL`: PostgreSQL connection string
- `WEBHOOK_PORT`: HTTP server port (default: 8443)
- `TLS_CERT_PATH`: Path to TLS certificate (default: /etc/tls/tls.crt)
- `TLS_KEY_PATH`: Path to TLS private key (default: /etc/tls/tls.key)

## Data Flow

### 1. Request Flow (Synchronous)

```
Kubernetes API Server
    │
    │ 1. User creates/updates/deletes resource
    │    POST /api/v1/namespaces/{ns}/deployments/{name}
    ▼
ValidatingAdmissionWebhook
    │
    │ 2. Webhook configured to intercept operation
    │    AdmissionReview request sent to webhook
    ▼
kubechronicle-webhook /validate
    │
    │ 3. Handler receives request
    │    - Decodes AdmissionReview
    │    - Extracts metadata (who, what, when)
    │    - Computes diff (for UPDATE)
    │    - Creates ChangeEvent
    │
    │ 4. Event queued (non-blocking)
    │    - Buffered channel (capacity: 1000)
    │    - If queue full, event dropped (logged)
    │
    │ 5. Response sent (<100ms target)
    │    - Always: Allowed: true
    │    - Fail-open behavior
    ▼
Kubernetes API Server
    │
    │ 6. Request continues processing
    │    (webhook did not block)
    ▼
Resource created/updated/deleted
```

### 2. Persistence Flow (Asynchronous)

```
Event Queue (Buffered Channel)
    │
    │ Worker goroutine continuously processes
    ▼
Async Processor
    │
    │ 1. Dequeue event from channel
    │ 2. Validate event structure
    │ 3. Save to PostgreSQL
    │    - JSONB serialization
    │    - Indexed storage
    │ 4. Error handling
    │    - Log errors
    │    - Continue processing
    ▼
PostgreSQL Database
    └─ change_events table
```

## Design Decisions

### Fail-Open Behavior

**Decision**: Webhook always returns `Allowed: true`, even on errors.

**Rationale**:
- kubechronicle is observe-only, not a security control
- Kubernetes operations must never be blocked by observation
- Better to lose some events than impact cluster operations
- Aligns with "never block" requirement

**Implementation**:
- All error paths in handler return `Allowed: true`
- Errors are logged but don't affect admission decision
- Queue overflow results in dropped events (logged as warning)

### Asynchronous Event Processing

**Decision**: Events are queued and persisted asynchronously.

**Rationale**:
- Webhook must respond in <100ms
- Database writes can be slow or fail
- Don't block Kubernetes API server
- Allows buffering during database outages

**Implementation**:
- Buffered Go channel (capacity: 1000 events)
- Single worker goroutine processes queue
- Non-blocking queue insertion
- Events dropped if queue full (logged)

### RFC 6902 JSON Patch

**Decision**: Use standard JSON Patch format for diffs.

**Rationale**:
- Standard format for structural diffs
- Machine-readable and human-readable
- Compatible with patch libraries
- Not text-based (as required)

**Operations**:
- `add`: Field added to resource
- `remove`: Field removed from resource
- `replace`: Field value changed

**Example**:
```json
[
  {
    "op": "replace",
    "path": "/spec/replicas",
    "value": 5
  },
  {
    "op": "add",
    "path": "/spec/template/metadata/labels/env",
    "value": "production"
  }
]
```

### Secret Hashing

**Decision**: SHA-256 hash all Secret values, never store plaintext.

**Rationale**:
- Security: Prevent secret leakage in audit logs
- Compliance: Meet security requirements
- Still track that secrets changed (hash changes)

**Implementation**:
- Detects Secret resources by kind
- Hashes both `data` (base64-encoded) and `stringData` (plaintext)
- Format: `sha256:<hex-hash>`
- Original values never stored

### Field Filtering

**Decision**: Ignore Kubernetes noise fields in diffs.

**Rationale**:
- Most Kubernetes fields change frequently but aren't user-initiated
- Reduces diff noise and storage size
- Focus on meaningful changes

**Ignored Fields**:
- `metadata.managedFields` - Controller-managed fields
- `metadata.resourceVersion` - Version tracking
- `metadata.generation` - Generation counter
- `status` - Runtime state
- `metadata.creationTimestamp` - Immutable timestamp

### PostgreSQL with JSONB

**Decision**: Use PostgreSQL with JSONB columns for storage.

**Rationale**:
- Flexible schema for evolving event structure
- Efficient JSON queries with GIN indexes
- Standard SQL for analysis
- Good performance for time-series data

**Alternative Considerations**:
- ClickHouse: Better for analytics, but adds complexity
- OpenSearch: Full-text search, but heavier stack

## Deployment Architecture

### Kubernetes Resources

**ValidatingWebhookConfiguration** (`webhook.yaml`):
- Intercepts CREATE, UPDATE, DELETE operations
- Scope: Cluster-wide
- Resources: Deployments, StatefulSets, DaemonSets, Services, ConfigMaps, Secrets
- Failure policy: `Ignore` (fail-open)
- TLS communication required

**Deployment** (`deployment.yaml`):
- Replicas: 2 (for high availability)
- Resources: CPU/memory limits defined
- Health probes: `/health` endpoint
- Volume mounts: TLS certificates, database secrets

**Service** (`service.yaml`):
- Type: ClusterIP
- Port: 443 (HTTPS)
- Selector: Matches webhook pods

**RBAC** (`rbac.yaml`):
- ServiceAccount: `kubechronicle-webhook`
- ClusterRole: Read-only permissions
- ClusterRoleBinding: Binds service account to role

### TLS Configuration

**Self-Signed Certificates** (default):
- Generated via `generate-certs.sh`
- Includes correct Subject Alternative Names (SANs)
- Stored in Kubernetes Secret: `kubechronicle-webhook-tls`
- No external CA or cert-manager required

**Certificate Requirements**:
- SANs must include:
  - `kubechronicle-webhook.kubechronicle.svc`
  - `kubechronicle-webhook.kubechronicle.svc.cluster.local`
- Valid for 1 year (configurable)

### High Availability

**Deployment Strategy**:
- 2+ replicas for redundancy
- Pod disruption budgets (recommended)
- Distributed across nodes (recommended)

**Database**:
- PostgreSQL should be deployed with HA (primary-replica)
- Connection pooling handles connection failures
- Application gracefully degrades if DB unavailable (events dropped, logged)

## Performance Characteristics

### Latency

- **Webhook Response Time**: <100ms (target)
  - Decoding: ~5-10ms
  - Diff computation: ~10-50ms (depends on object size)
  - Queue insertion: <1ms (non-blocking)
  - Total: Typically 20-70ms

### Throughput

- **Events/Second**: ~100-500 (depends on cluster activity)
- **Queue Capacity**: 1000 events
- **Database Write Rate**: Depends on PostgreSQL performance
- **Bottlenecks**: 
  - Diff computation for large objects
  - Database write latency during high load

### Resource Usage

- **CPU**: ~100-500m per pod (depends on event rate)
- **Memory**: ~128-512Mi per pod
- **Network**: Minimal (local cluster traffic)

## Scalability

### Horizontal Scaling

- **Webhook Pods**: Can scale to 3-5 replicas
  - Kubernetes load balances webhook requests
  - Each pod has independent event queue
  - No coordination needed

**Limitations**:
- More pods = more database connections
- Database becomes bottleneck at high scale

### Vertical Scaling

- **Increase Queue Size**: Currently 1000, can be increased
- **Worker Threads**: Currently 1, can be increased for faster processing
- **Database Connection Pool**: Configured in PostgreSQL store

### Database Scaling

**Recommendations**:
- Use read replicas for query API
- Partition `change_events` table by timestamp (monthly)
- Archive old events (>1 year) to cold storage
- Use connection pooling (PGBouncer)

## Security Considerations

### Webhook Security

- **TLS Required**: All webhook communication is encrypted
- **RBAC**: Least privilege permissions (read-only)
- **Network Policy**: Restrict access to webhook service (recommended)

### Secret Handling

- **Never Stored Plaintext**: All Secret values are SHA-256 hashed
- **Diff Tracking**: Still tracks that secrets changed (hash changes)
- **Audit**: Security teams can see secret changes without exposing values

### Data Protection

- **Database Encryption**: Use PostgreSQL encryption at rest
- **Network Encryption**: TLS for database connections (recommended)
- **Access Control**: Limit database access to webhook service account

## Monitoring & Observability

### Metrics (Recommended)

- Webhook request count (by operation type)
- Webhook latency (p50, p95, p99)
- Queue depth (events pending)
- Events processed per second
- Events dropped (queue full)
- Database write errors
- Database connection pool metrics

### Logging

- All errors are logged with context
- Queue overflow warnings
- Database connection issues
- Request/response logging (debug mode)

### Health Checks

- `/health` endpoint for Kubernetes liveness/readiness probes
- Database connectivity check
- Event processing status

## Future Enhancements

### Potential Improvements

1. **Query API**: REST API for querying change events
2. **Web UI**: Timeline visualization of resource changes
3. **Alerting**: Notify on specific change patterns
4. **Integrations**: Slack, webhooks, SIEM systems
5. **Filtering**: Configurable field ignore rules
6. **Retention Policies**: Automatic cleanup of old events
7. **Compression**: Compress large object snapshots
8. **Batch Writes**: Batch database writes for better throughput

### Known Limitations

- **Queue Overflow**: Events dropped if queue full (logged)
- **Database Dependency**: Requires PostgreSQL (optional but recommended)
- **Diff Size**: Large objects produce large diffs
- **No Reconciliation**: Doesn't track GitOps reconciliation changes differently

## Component Dependencies

```
cmd/webhook
  ├── internal/admission (Handler, Decoder)
  │     ├── internal/diff (ComputeDiff, filterIgnoredFields)
  │     └── internal/model (ChangeEvent, Actor, Source, PatchOp)
  ├── internal/store (PostgreSQL implementation)
  │     └── internal/model
  └── internal/config
```

## Technology Stack

- **Language**: Go 1.21+
- **Kubernetes**: client-go, apimachinery, controller-runtime
- **Database**: PostgreSQL 12+ with JSONB
- **HTTP**: Standard library `net/http`
- **TLS**: Standard library `crypto/tls`
- **Logging**: klog (Kubernetes logging library)
