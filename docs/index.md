# kubechronicle

**kubechronicle** provides a human-readable, queryable history of Kubernetes resource changes.


## Features

- **Tracks WHO** changed a resource (username, groups, service account, source IP)
- **Records WHAT** changed (RFC 6902 JSON Patch diffs)
- **Captures WHEN** changes occurred (precise timestamps)
- **Shows HOW** resources changed (structured diffs)
- Supports **CREATE, UPDATE, and DELETE** operations
- **Exec tracking**: Cache pod exec and node exec via audit logs
- **Secret-safe**: Never stores plaintext secrets (SHA-256 hashed)
- **Fail-open**: Never blocks Kubernetes API requests (observe-only)

## Architecture

kubechronicle uses a Kubernetes **ValidatingAdmissionWebhook** to observe resource changes without blocking or mutating requests. Change events are persisted to PostgreSQL for querying and analysis.

### Components

| Component | Description |
|----------|-------------|
| **Admission Webhook** | Observes all resource changes (observe-only) |
| **Diff Engine** | Computes RFC 6902 JSON Patch diffs with ignore rules |
| **Storage Layer** | PostgreSQL backend with JSONB for flexible querying |
| **Async Processing** | Non-blocking event persistence |
| **Audit Processor** | Optional: tracks pod/node exec via Kubernetes audit logs |

## Quick links

- [Getting Started](getting-started.md) — Prerequisites, deploy, verify
- [API Reference](api.md) — REST API for querying change events
- [Deployment](deployment.md) — Step-by-step deployment guide
- [Architecture](architecture.md) — Design and internals
- [Exec Tracking](audit-processor.md) — Cache pod/node exec via audit logs

## Security

- **TLS-encrypted**: All webhook communication uses TLS
- **Least privilege**: Minimal RBAC permissions (observe-only by default)
- **Non-root**: Pods run as non-root user
- **Secret hashing**: All Secret values are SHA-256 hashed
- **Fail-open by default**: Never blocks API server requests unless block patterns are configured
- **Optional enforcement**: Can be configured to block operations via `BLOCK_CONFIG`
