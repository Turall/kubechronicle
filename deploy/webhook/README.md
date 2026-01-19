# kubechronicle Deployment Manifests

This directory contains all Kubernetes manifests needed to deploy kubechronicle.

## Files

- `namespace.yaml` - Creates the kubechronicle namespace
- `serviceaccount.yaml` - Service account for the webhook pod
- `rbac.yaml` - ClusterRole and ClusterRoleBinding (minimal permissions)
- `deployment.yaml` - Webhook deployment with 2 replicas
- `service.yaml` - ClusterIP service exposing the webhook
- `webhook.yaml` - ValidatingWebhookConfiguration for admission control
- `certificate.yaml` - Optional cert-manager certificate (for TLS)
- `kustomization.yaml` - Kustomize configuration for easy deployment

## Prerequisites

1. **TLS Certificates**: The webhook requires TLS certificates. Self-signed certificates are the default and recommended approach (no cert-manager or CA required).

   **Option A: Self-Signed Certificates (Recommended - No Dependencies)**
   
   Self-signed certificates work perfectly for internal cluster communication. Kubernetes will accept them as long as the certificate is valid for the service DNS name (which our script ensures).
   
   Using the provided script (easiest):
   ```bash
   # Generate self-signed certificate and create Kubernetes secret
   ./generate-certs.sh
   ```
   
   Or using Makefile:
   ```bash
   make generate-certs
   ```
   
   Or manually:
   ```bash
   # Generate self-signed certificate with proper Subject Alternative Names
   openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt -days 365 -nodes \
     -subj "/CN=kubechronicle-webhook.kubechronicle.svc" \
     -addext "subjectAltName=DNS:kubechronicle-webhook.kubechronicle.svc,DNS:kubechronicle-webhook.kubechronicle.svc.cluster.local,DNS:kubechronicle-webhook,DNS:localhost"
   
   # Create Kubernetes secret
   kubectl create namespace kubechronicle --dry-run=client -o yaml | kubectl apply -f -
   kubectl create secret tls kubechronicle-webhook-tls \
     --cert=tls.crt \
     --key=tls.key \
     -n kubechronicle
   
   # Clean up local files (optional)
   rm tls.crt tls.key
   ```
   
   **Note:** No CA bundle configuration needed! When using a service reference (not URL), Kubernetes automatically trusts certificates that are valid for the service DNS name.

   **Option B: Using cert-manager (Optional - Only if you have cert-manager installed)**
   ```bash
   # Install cert-manager if not already installed
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   
   # Deploy with certificate.yaml
   kubectl apply -f certificate.yaml
   ```

2. **Database Secret** (Optional): If using PostgreSQL, create a secret with the database URL:
   ```bash
   kubectl create secret generic kubechronicle-database \
     --from-literal=url="postgres://user:password@host:port/database?sslmode=disable" \
     -n kubechronicle
   ```

## Deployment

### Quick Start (Recommended)

```bash
# 1. Generate self-signed certificates
./generate-certs.sh

# 2. Deploy kubechronicle
make deploy
```

### Step-by-Step Deployment

**1. Generate TLS certificates:**
```bash
./generate-certs.sh
# or
make generate-certs
```

**2. (Optional) Create database secret:**
```bash
make create-db-secret
# or manually:
kubectl create secret generic kubechronicle-database \
  --from-literal=url="postgres://user:password@host:port/database?sslmode=disable" \
  -n kubechronicle
```

**3. Deploy kubechronicle:**
```bash
# Using Makefile (recommended)
make deploy

# Or using kubectl directly
kubectl apply -f namespace.yaml
kubectl apply -f serviceaccount.yaml
kubectl apply -f rbac.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f webhook.yaml

# Or using kustomize
kubectl apply -k .
```

## Verification

1. **Check webhook pod status:**
   ```bash
   kubectl get pods -n kubechronicle
   kubectl logs -n kubechronicle -l app.kubernetes.io/name=kubechronicle
   ```

2. **Check webhook configuration:**
   ```bash
   kubectl get validatingwebhookconfiguration kubechronicle-webhook
   ```

3. **Test webhook:**
   ```bash
   # Create a test deployment
   kubectl create deployment test --image=nginx
   
   # Check if event was recorded (if database is configured)
   # The webhook should allow the request and log the change
   ```

## Configuration

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string (optional, from secret)
- `TLS_CERT_PATH`: Path to TLS certificate (default: `/etc/webhook/certs/tls.crt`)
- `TLS_KEY_PATH`: Path to TLS private key (default: `/etc/webhook/certs/tls.key`)
- `LOG_LEVEL`: Logging level (default: `info`)

### Resource Limits

The deployment sets:
- Requests: 100m CPU, 128Mi memory
- Limits: 500m CPU, 512Mi memory

Adjust in `deployment.yaml` as needed.

## Security

- **Security Context**: Pods run as non-root (UID 65534)
- **Least Privilege**: RBAC grants minimal permissions (observe-only)
- **TLS**: All webhook communication is TLS-encrypted
- **Fail-Open**: Webhook uses `failurePolicy: Ignore` to never block API server

## Troubleshooting

1. **Webhook not receiving requests:**
   - Check webhook configuration: `kubectl get validatingwebhookconfiguration kubechronicle-webhook -o yaml`
   - Verify service is accessible: `kubectl get svc -n kubechronicle`
   - Check pod logs for errors

2. **Certificate issues:**
   - Verify TLS secret exists: `kubectl get secret kubechronicle-webhook-tls -n kubechronicle`
   - Check secret contains both tls.crt and tls.key: `kubectl describe secret kubechronicle-webhook-tls -n kubechronicle`
   - If secret is missing, regenerate: `./generate-certs.sh`
   - For cert-manager users: Check certificate validity: `kubectl get certificate -n kubechronicle`

3. **Database connection issues:**
   - Verify secret exists: `kubectl get secret kubechronicle-database -n kubechronicle`
   - Check pod logs for connection errors
   - Webhook will continue without database (events logged only)
