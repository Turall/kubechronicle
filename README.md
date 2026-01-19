# kubechronicle

kubechronicle provides a **human-readable, queryable history of Kubernetes resource changes**.

## Features

- **Tracks WHO** changed a resource (username, groups, service account, source IP)
- **Records WHAT** changed (RFC 6902 JSON Patch diffs)
- **Captures WHEN** changes occurred (precise timestamps)
- **Shows HOW** resources changed (structured diffs)
- Supports **CREATE, UPDATE, and DELETE** operations
- **Secret-safe**: Never stores plaintext secrets (SHA-256 hashed)
- **Fail-open**: Never blocks Kubernetes API requests (observe-only)

## Architecture

kubechronicle uses a Kubernetes **ValidatingAdmissionWebhook** to observe resource changes without blocking or mutating requests. Change events are persisted to PostgreSQL for querying and analysis.

### Components

- **Admission Webhook**: Observes all resource changes (observe-only)
- **Diff Engine**: Computes RFC 6902 JSON Patch diffs with ignore rules
- **Storage Layer**: PostgreSQL backend with JSONB for flexible querying
- **Async Processing**: Non-blocking event persistence

## Quick Start

### Prerequisites

- Kubernetes cluster (1.20+)
- PostgreSQL database (optional, webhook works without it)
- `openssl` for generating self-signed certificates (included in most systems)

### Deployment

1. **Generate self-signed TLS certificates** (no cert-manager required):
   ```bash
   cd deploy/webhook
   ./generate-certs.sh
   # or: make generate-certs
   ```

2. **Create database secret** (optional):
   ```bash
   make create-db-secret
   ```

3. **Deploy kubechronicle**:
   ```bash
   make deploy
   ```

4. **Verify deployment**:
   ```bash
   make verify
   ```

**Note:** Self-signed certificates are the default and recommended approach. No cert-manager or CA certificates required. See [deploy/webhook/README.md](deploy/webhook/README.md) for detailed deployment instructions.

## Tracked Resources

Currently tracks:
- **Core Resources**: Deployments, StatefulSets, DaemonSets, Services, ConfigMaps, Secrets (hashed), Ingress
- **Custom Resource Definitions**: CRD definitions themselves (CustomResourceDefinition resources)

### Tracking CRD Instances

Kubernetes admission webhooks **don't support wildcards** in rules, so you need to add specific rules for each CRD type you want to track.

**To track CRD instances, add rules to `deploy/webhook/webhook.yaml`:**

```yaml
# Example: Track cert-manager Certificates
- apiGroups: ["cert-manager.io"]
  apiVersions: ["v1"]
  operations: ["CREATE", "UPDATE", "DELETE"]
  resources: ["certificates"]

# Example: Track ArgoCD Applications
- apiGroups: ["argoproj.io"]
  apiVersions: ["v1alpha1"]
  operations: ["CREATE", "UPDATE", "DELETE"]
  resources: ["applications"]

# Example: Track any CRD from a specific API group
- apiGroups: ["your-crd-group.io"]
  apiVersions: ["v1", "v1alpha1"]  # Can specify multiple versions
  operations: ["CREATE", "UPDATE", "DELETE"]
  resources: ["your-crd-resource"]
```

**To find CRD details:**
```bash
# List all CRDs
kubectl get crds

# Get details for a specific CRD
kubectl get crd <crd-name> -o yaml
# Look for: spec.group, spec.versions[*].name, spec.names.plural
```

See `deploy/webhook/crd-examples.yaml` for common CRD examples (cert-manager, ArgoCD, Prometheus, Istio, etc.).

After adding CRD rules, apply the updated webhook configuration:
```bash
kubectl apply -f deploy/webhook/webhook.yaml
```

## Ignored Fields

The diff engine automatically ignores Kubernetes noise fields:
- `metadata.managedFields`
- `metadata.resourceVersion`
- `metadata.generation`
- `metadata.creationTimestamp`
- `status` (entire subtree)
- `kubectl.kubernetes.io/last-applied-configuration`

## Ignore Patterns

You can configure kubechronicle to ignore specific namespaces, resource names, or resource kinds using ignore patterns. This is useful to exclude system namespaces or noisy resources from tracking.

### Configuration

Ignore patterns can be configured via the `IGNORE_CONFIG` environment variable (JSON format) or using comma-separated environment variables for simple cases:

**Option 1: JSON Configuration (Recommended)**
```bash
export IGNORE_CONFIG='{
  "namespace_patterns": ["kube-*", "cert-manager"],
  "name_patterns": ["*-controller", "*system*"],
  "resource_kind_patterns": ["ConfigMap"]
}'
```

**Option 2: Comma-Separated Lists (Simple)**
```bash
export IGNORE_NAMESPACES="kube-*,cert-manager,kube-system"
export IGNORE_NAMES="*-controller,*system*"
```

### Pattern Matching

Patterns support wildcards:
- `*` matches any sequence of characters (including empty)
- Exact matches if no wildcards

**Examples:**
- `kube-*` matches `kube-system`, `kube-public`, `kube-node-lease`
- `*-controller` matches `deployment-controller`, `namespace-controller`
- `*system*` matches `system-config`, `my-system-ns`, `system`
- `cert-manager` matches exactly `cert-manager`

### Kubernetes Deployment

Add to your `deployment.yaml`:
```yaml
env:
  - name: IGNORE_CONFIG
    value: |
      {
        "namespace_patterns": ["kube-*", "cert-manager"],
        "name_patterns": ["*-controller"],
        "resource_kind_patterns": []
      }
```

**Note:** Ignored resources are still allowed through the webhook (fail-open), but they won't be tracked or stored in the database.

## Block Operations

You can configure kubechronicle to **block** (deny) specific operations based on patterns. This allows you to enforce policies and prevent certain changes.

**⚠️ Warning:** Blocking operations changes the webhook from observe-only to enforcement mode. Use with caution and test thoroughly.

### Configuration

Block patterns are configured via the `BLOCK_CONFIG` environment variable (JSON format):

```bash
export BLOCK_CONFIG='{
  "namespace_patterns": ["production"],
  "name_patterns": ["*-delete", "*critical*"],
  "resource_kind_patterns": ["Secret"],
  "operation_patterns": ["DELETE"],
  "message": "Deleting resources in production namespace is not allowed"
}'
```

### Block Configuration Options

- **`namespace_patterns`**: Block resources in matching namespaces
- **`name_patterns`**: Block resources with matching names
- **`resource_kind_patterns`**: Block specific resource kinds
- **`operation_patterns`**: Block specific operations (CREATE, UPDATE, DELETE). If empty, all operations matching other patterns are blocked.
- **`message`**: Custom error message returned when a request is blocked (default: "Resource blocked by kubechronicle policy")

### Examples

**Block all DELETE operations in production:**
```json
{
  "namespace_patterns": ["production"],
  "operation_patterns": ["DELETE"],
  "message": "Deleting resources in production is not allowed"
}
```

**Block all Secret resources:**
```json
{
  "resource_kind_patterns": ["Secret"],
  "message": "Secret resources are blocked by policy"
}
```

**Block resources with "critical" in the name:**
```json
{
  "name_patterns": ["*critical*"],
  "message": "Critical resources cannot be modified"
}
```

### Kubernetes Deployment

Add to your `deployment.yaml`:
```yaml
env:
  - name: BLOCK_CONFIG
    value: |
      {
        "namespace_patterns": ["production"],
        "operation_patterns": ["DELETE"],
        "message": "Deleting resources in production is not allowed"
      }
```

**Important Notes:**
- Blocking is checked **before** ignore patterns
- Blocked requests return `Allowed: false` with an error message
- Blocked events are **not** saved to the database
- The webhook must respond in <100ms, so keep block patterns simple
- Test blocking rules in a non-production environment first

## Security

- **TLS-encrypted**: All webhook communication uses TLS
- **Least privilege**: Minimal RBAC permissions (observe-only by default)
- **Non-root**: Pods run as non-root user
- **Secret hashing**: All Secret values are SHA-256 hashed
- **Fail-open by default**: Never blocks API server requests unless block patterns are configured
- **Optional enforcement**: Can be configured to block operations via `BLOCK_CONFIG`

## Development

### Quick Start

```bash
# 1. Install dependencies
make deps

# 2. Build the project
make build

# 3. Generate TLS certificates (for local testing)
cd deploy/webhook && ./generate-certs.sh && cd ../..
mkdir -p certs && cp deploy/webhook/tls.crt deploy/webhook/tls.key certs/

# 4. Run locally
make run
```

### Building

```bash
# Using Makefile
make build

# Or manually
go build -o bin/webhook ./cmd/webhook
```

### Running Locally

```bash
# Using Makefile (requires certs in ./certs/)
make run

# Or manually
export DATABASE_URL="postgres://user:pass@localhost/kubechronicle"
export TLS_CERT_PATH="./certs/tls.crt"
export TLS_KEY_PATH="./certs/tls.key"
./bin/webhook
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed development instructions.

## Testing

The project maintains **90% test coverage**. Run tests with:

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Verify 90% coverage threshold
make test-coverage-check
```

See [COVERAGE.md](COVERAGE.md) for detailed testing guidelines.

## Project Structure

```
/cmd/webhook/          - Main application entry point
/internal/
  /admission/          - Webhook handler and request decoder
  /diff/               - RFC 6902 diff engine with ignore rules
  /store/               - PostgreSQL storage layer
  /model/               - Data models
  /config/              - Configuration management
/deploy/webhook/       - Kubernetes manifests
/docs/                 - Documentation
```

## License

Open source - see LICENSE file for details.
