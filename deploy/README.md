# kubechronicle Deployment Manifests

This directory contains Kubernetes manifests for deploying kubechronicle without Helm charts.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.20+)
- `kubectl` configured to access your cluster
- `kustomize` (optional, for easier deployment)

### 1. Create Namespace

```bash
kubectl create namespace kubechronicle
```

### 2. Create Database Secret

```bash
kubectl create secret generic kubechronicle-database \
  --from-literal=url="postgres://user:password@host:5432/kubechronicle" \
  --namespace kubechronicle
```

### 3. (Optional) Enable Authentication

If you want to enable authentication:

```bash
# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32)

# Generate password hashes
go build -o bin/password-hash ./cmd/password-hash
ADMIN_PASS_HASH=$(./bin/password-hash "admin123")
VIEWER_PASS_HASH=$(./bin/password-hash "viewer123")

# Create auth secret
kubectl create secret generic kubechronicle-auth \
  --from-literal=jwt-secret="${JWT_SECRET}" \
  --from-literal=users="{\"admin\":{\"password\":\"${ADMIN_PASS_HASH}\",\"roles\":[\"admin\",\"viewer\"],\"email\":\"admin@example.com\"},\"viewer\":{\"password\":\"${VIEWER_PASS_HASH}\",\"roles\":[\"viewer\"]}}" \
  --namespace kubechronicle

# Uncomment auth env vars in deploy/api/deployment.yaml
```

### 4. Generate TLS Certificates for Webhook

```bash
cd deploy/webhook
./generate-certs.sh
kubectl create secret tls kubechronicle-webhook-tls \
  --cert=tls.crt \
  --key=tls.key \
  --namespace kubechronicle
cd ../..
```

### 5. Deploy

**Option A: Using kustomize (recommended)**

```bash
kubectl apply -k deploy/
```

**Option B: Deploy components individually**

```bash
# Deploy patterns ConfigMap
kubectl apply -f deploy/patterns-configmap.yaml

# Deploy API
kubectl apply -k deploy/api/

# Deploy UI
kubectl apply -k deploy/ui/

# Deploy Webhook
kubectl apply -k deploy/webhook/
```

### 6. Verify Deployment

```bash
kubectl get pods -n kubechronicle
kubectl get svc -n kubechronicle
kubectl get validatingwebhookconfiguration kubechronicle-webhook
```

## Component Details

### API Component (`deploy/api/`)

- **Deployment**: API server with pattern management support
- **Service**: ClusterIP service exposing API on port 80
- **ServiceAccount**: For RBAC permissions
- **RBAC**: Role and RoleBinding for ConfigMap management

**Features:**
- Pattern management via ConfigMap
- Optional authentication (JWT-based)
- Database connection via Secret

### UI Component (`deploy/ui/`)

- **Deployment**: React UI served via nginx
- **Service**: ClusterIP service exposing UI on port 80

**Access:**
```bash
kubectl port-forward -n kubechronicle svc/kubechronicle-ui 8080:80
# Open http://localhost:8080
```

### Webhook Component (`deploy/webhook/`)

- **Deployment**: Admission webhook for tracking changes
- **Service**: ClusterIP service on port 443
- **ServiceAccount**: For webhook operations
- **RBAC**: ClusterRole and ClusterRoleBinding
- **ValidatingWebhookConfiguration**: Webhook registration

**Features:**
- Reads patterns from ConfigMap
- TLS-encrypted communication
- Observe-only by default (fail-open)

## Pattern Management

Patterns are stored in the `kubechronicle-patterns` ConfigMap and can be managed via:

1. **UI** (admin-only): Navigate to Patterns page
2. **API** (admin-only): `PUT /api/admin/patterns/ignore` or `/api/admin/patterns/block`
3. **kubectl**: Direct ConfigMap editing

### Update Patterns via kubectl

```bash
# Edit ConfigMap
kubectl edit configmap kubechronicle-patterns -n kubechronicle

# Or patch directly
kubectl patch configmap kubechronicle-patterns -n kubechronicle \
  --type merge \
  -p '{"data":{"IGNORE_CONFIG":"{\"namespace_patterns\":[\"kube-*\"],\"name_patterns\":[],\"resource_kind_patterns\":[]}"}}'

# Restart webhook pods to pick up changes
kubectl rollout restart deployment/kubechronicle-webhook -n kubechronicle
```

## Authentication

Authentication is **disabled by default**. To enable:

1. Create auth secret (see step 3 above)
2. Uncomment auth environment variables in `deploy/api/deployment.yaml`:
   ```yaml
   - name: AUTH_ENABLED
     value: "true"
   - name: JWT_SECRET
     valueFrom:
       secretKeyRef:
         name: kubechronicle-auth
         key: jwt-secret
   - name: JWT_EXPIRATION_HOURS
     value: "24"
   - name: AUTH_USERS
     valueFrom:
       secretKeyRef:
         name: kubechronicle-auth
         key: users
   ```
3. Apply updated deployment:
   ```bash
   kubectl apply -f deploy/api/deployment.yaml
   ```

## Configuration

### Environment Variables

**API Server:**
- `DATABASE_URL`: PostgreSQL connection string (required)
- `LOG_LEVEL`: Logging level (default: "info")
- `NAMESPACE`: Kubernetes namespace (default: "kubechronicle")
- `PATTERNS_CONFIGMAP_NAME`: ConfigMap name for patterns (default: "kubechronicle-patterns")
- `AUTH_ENABLED`: Enable authentication (default: false)
- `JWT_SECRET`: JWT signing secret (required if AUTH_ENABLED=true)
- `JWT_EXPIRATION_HOURS`: Token expiration in hours (default: 24)
- `AUTH_USERS`: JSON string with user configuration (required if AUTH_ENABLED=true)

**Webhook:**
- `DATABASE_URL`: PostgreSQL connection string (optional)
- `LOG_LEVEL`: Logging level (default: "info")
- `TLS_CERT_PATH`: Path to TLS certificate (default: "/etc/webhook/certs/tls.crt")
- `TLS_KEY_PATH`: Path to TLS key (default: "/etc/webhook/certs/tls.key")
- `IGNORE_CONFIG`: JSON string with ignore patterns (loaded from ConfigMap)
- `BLOCK_CONFIG`: JSON string with block patterns (loaded from ConfigMap)

### Secrets

- `kubechronicle-database`: Database connection string
- `kubechronicle-auth`: Authentication configuration (optional)
- `kubechronicle-webhook-tls`: TLS certificates for webhook

### ConfigMaps

- `kubechronicle-patterns`: Ignore and block patterns

## Troubleshooting

### Webhook not receiving requests

```bash
# Check webhook status
kubectl get validatingwebhookconfiguration kubechronicle-webhook

# Check webhook logs
kubectl logs -n kubechronicle -l app.kubernetes.io/component=webhook

# Verify TLS certificate
kubectl get secret kubechronicle-webhook-tls -n kubechronicle
```

### API cannot access ConfigMap

```bash
# Check ServiceAccount
kubectl get serviceaccount kubechronicle-api -n kubechronicle

# Check Role and RoleBinding
kubectl get role kubechronicle-api-patterns -n kubechronicle
kubectl get rolebinding kubechronicle-api-patterns -n kubechronicle

# Check API logs
kubectl logs -n kubechronicle -l app.kubernetes.io/component=api
```

### Authentication not working

```bash
# Verify auth secret exists
kubectl get secret kubechronicle-auth -n kubechronicle

# Check API logs for auth errors
kubectl logs -n kubechronicle -l app.kubernetes.io/component=api | grep -i auth

# Verify AUTH_ENABLED is set
kubectl get deployment kubechronicle-api -n kubechronicle -o yaml | grep AUTH_ENABLED
```

## Upgrading

```bash
# Update images
kubectl set image deployment/kubechronicle-api \
  api=kubechronicle/api:new-tag \
  -n kubechronicle

kubectl set image deployment/kubechronicle-ui \
  ui=kubechronicle/ui:new-tag \
  -n kubechronicle

kubectl set image deployment/kubechronicle-webhook \
  webhook=kubechronicle/webhook:new-tag \
  -n kubechronicle
```

## Uninstalling

```bash
# Delete all resources
kubectl delete -k deploy/

# Or delete components individually
kubectl delete -k deploy/api/
kubectl delete -k deploy/ui/
kubectl delete -k deploy/webhook/
kubectl delete -f deploy/patterns-configmap.yaml

# Delete namespace (removes all resources)
kubectl delete namespace kubechronicle
```

**Note**: The ValidatingWebhookConfiguration and ClusterRole/ClusterRoleBinding are cluster-scoped and may need to be deleted manually:

```bash
kubectl delete validatingwebhookconfiguration kubechronicle-webhook
kubectl delete clusterrole kubechronicle-webhook
kubectl delete clusterrolebinding kubechronicle-webhook
```
