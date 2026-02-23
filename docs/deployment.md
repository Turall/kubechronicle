# Deployment Guide

This guide covers deploying kubechronicle using the manifests in the `deploy/` directory.

## Deployment order

1. **Namespace** (created automatically by kustomize)
2. **Secrets** (database, auth, TLS)
3. **ConfigMap** (patterns)
4. **API** (ServiceAccount, RBAC, Deployment, Service)
5. **UI** (Deployment, Service)
6. **Webhook** (ServiceAccount, RBAC, Deployment, Service, ValidatingWebhookConfiguration)

## Step-by-step deployment

### 1. Create namespace

```bash
kubectl create namespace kubechronicle
```

### 2. Create database secret

```bash
kubectl create secret generic kubechronicle-database \
  --from-literal=url="postgres://user:password@host:5432/kubechronicle" \
  --namespace kubechronicle
```

### 3. Create patterns ConfigMap

```bash
kubectl apply -f deploy/patterns-configmap.yaml
```

### 4. Generate webhook TLS certificates

```bash
cd deploy/webhook
./generate-certs.sh
kubectl create secret tls kubechronicle-webhook-tls \
  --cert=tls.crt \
  --key=tls.key \
  --namespace kubechronicle
cd ../..
```

### 5. (Optional) Enable authentication

```bash
# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32)

# Generate password hashes
go build -o bin/password-hash ./cmd/password-hash
ADMIN_PASS=$(./bin/password-hash "your-admin-password")
VIEWER_PASS=$(./bin/password-hash "your-viewer-password")

# Create auth secret
kubectl create secret generic kubechronicle-auth \
  --from-literal=jwt-secret="${JWT_SECRET}" \
  --from-literal=users="{\"admin\":{\"password\":\"${ADMIN_PASS}\",\"roles\":[\"admin\",\"viewer\"],\"email\":\"admin@example.com\"},\"viewer\":{\"password\":\"${VIEWER_PASS}\",\"roles\":[\"viewer\"]}}" \
  --namespace kubechronicle

# Edit deploy/api/deployment.yaml and uncomment auth env vars
# Then apply: kubectl apply -f deploy/api/deployment.yaml
```

### 6. Deploy all components

**Using kustomize (recommended):**

```bash
kubectl apply -k deploy/
```

**Or deploy individually:**

```bash
kubectl apply -k deploy/api/
kubectl apply -k deploy/ui/
kubectl apply -k deploy/webhook/
```

### 7. Verify deployment

```bash
# Check pods
kubectl get pods -n kubechronicle

# Check services
kubectl get svc -n kubechronicle

# Check webhook
kubectl get validatingwebhookconfiguration kubechronicle-webhook

# Check logs
kubectl logs -n kubechronicle -l app.kubernetes.io/component=api
kubectl logs -n kubechronicle -l app.kubernetes.io/component=webhook
```

## Accessing the UI

```bash
# Port forward
kubectl port-forward -n kubechronicle svc/kubechronicle-ui 8080:80

# Open http://localhost:8080 in your browser
```

If authentication is enabled, you'll be redirected to the login page.

## Managing patterns

### Via UI (admin only)

1. Log in as admin
2. Navigate to **Patterns** in the navigation
3. Edit ignore/block patterns
4. Click **Save**

### Via API (admin only)

```bash
# Get token
TOKEN=$(curl -X POST http://api-url/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}' | jq -r '.token')

# Get ignore patterns
curl http://api-url/api/admin/patterns/ignore \
  -H "Authorization: Bearer $TOKEN"

# Update ignore patterns
curl -X PUT http://api-url/api/admin/patterns/ignore \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "namespace_patterns": ["kube-*"],
    "name_patterns": ["*-controller"],
    "resource_kind_patterns": []
  }'
```

### Via kubectl

```bash
# Edit ConfigMap
kubectl edit configmap kubechronicle-patterns -n kubechronicle

# Restart webhook to pick up changes
kubectl rollout restart deployment/kubechronicle-webhook -n kubechronicle
```

## Troubleshooting

### API cannot access ConfigMap

Ensure RBAC is properly configured:

```bash
kubectl get role kubechronicle-api-patterns -n kubechronicle -o yaml
kubectl get rolebinding kubechronicle-api-patterns -n kubechronicle -o yaml
```

### Webhook not working

Check webhook configuration:

```bash
kubectl get validatingwebhookconfiguration kubechronicle-webhook -o yaml
kubectl logs -n kubechronicle -l app.kubernetes.io/component=webhook
```

### Authentication issues

Verify auth secret and env vars:

```bash
kubectl get secret kubechronicle-auth -n kubechronicle
kubectl get deployment kubechronicle-api -n kubechronicle -o yaml | grep AUTH
```

## File structure

```
deploy/
├── kustomization.yaml          # Root kustomization (deploys all)
├── patterns-configmap.yaml     # Patterns configuration
├── auth-secret.yaml.example    # Example auth secret
├── api/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   └── rbac.yaml
├── ui/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   └── service.yaml
└── webhook/
    ├── kustomization.yaml
    ├── deployment.yaml
    ├── service.yaml
    ├── serviceaccount.yaml
    ├── rbac.yaml
    └── webhook.yaml             # ValidatingWebhookConfiguration
```
