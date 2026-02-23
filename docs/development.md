# Development Guide

This guide explains how to set up and run kubechronicle locally for development.

## Prerequisites

- **Go 1.25 or later**: [Install Go](https://golang.org/doc/install)
- **kubectl**: For testing with Kubernetes (optional)
- **PostgreSQL**: Optional, for testing storage layer
- **openssl**: For generating TLS certificates

## Quick start

### 1. Install dependencies

```bash
# Using Makefile (recommended)
make deps

# Or manually
go mod download
go mod tidy
```

### 2. Build the project

```bash
# Using Makefile
make build

# Or manually
go build -o bin/webhook ./cmd/webhook
```

### 3. Generate TLS certificates

For local development, you need TLS certificates:

```bash
# Option 1: Use the deployment script
cd deploy/webhook
./generate-certs.sh
# Copy certificates to project root
mkdir -p ../certs
cp tls.crt ../certs/
cp tls.key ../certs/
cd ../..

# Option 2: Generate manually
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/tls.key -out certs/tls.crt -days 365 -nodes \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,DNS:127.0.0.1"
```

### 4. Run locally

```bash
# Using Makefile
make run

# Or manually
./bin/webhook \
  -port=8443 \
  -cert=./certs/tls.crt \
  -key=./certs/tls.key
```

### 5. Test the webhook

In another terminal, test the health endpoint:

```bash
curl -k https://localhost:8443/health
```

## Environment variables

You can configure the webhook using environment variables:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/kubechronicle?sslmode=disable"
export TLS_CERT_PATH="./certs/tls.crt"
export TLS_KEY_PATH="./certs/tls.key"
export LOG_LEVEL="debug"
```

## Development workflow

### Format code

```bash
make fmt
# or
go fmt ./...
```

### Run tests

```bash
make test
# or
go test ./...
```

### Run linter

```bash
# Install golangci-lint first
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
make lint
```

### Build Docker image

```bash
make docker-build
# or
docker build -t kubechronicle/webhook:latest .
```

## Testing with Kubernetes

### 1. Deploy to local cluster

If you have a local Kubernetes cluster (minikube, kind, etc.):

```bash
# Build and push image (or use local registry)
docker build -t kubechronicle/webhook:latest .
# For kind:
kind load docker-image kubechronicle/webhook:latest

# Deploy
cd deploy/webhook
make deploy
```

### 2. Test webhook

```bash
# Create a test resource
kubectl create deployment test --image=nginx

# Check webhook logs
kubectl logs -n kubechronicle -l app.kubernetes.io/name=kubechronicle -f
```

## Project structure

```
.
├── cmd/
│   ├── webhook/          # Webhook entry point
│   ├── api/              # API server entry point
│   └── audit-processor/  # Audit log processor entry point
├── internal/
│   ├── admission/        # Webhook handler and decoder
│   ├── audit/            # Audit log processor (exec tracking)
│   ├── diff/             # RFC 6902 diff engine
│   ├── store/            # Storage layer
│   ├── model/            # Data models
│   └── config/           # Configuration
├── deploy/               # Kubernetes manifests
├── docs/                 # Documentation (MkDocs)
├── bin/                  # Build output (gitignored)
└── certs/                # TLS certificates (gitignored)
```

## Common issues

### "certificate signed by unknown authority"

This is normal for self-signed certificates. Use `-k` flag with curl or configure your client to skip verification.

### "bind: address already in use"

Another process is using port 8443. Change the port:

```bash
./bin/webhook -port=8444 -cert=./certs/tls.crt -key=./certs/tls.key
```

### "no such file or directory" for certificates

Make sure you've generated the certificates and they're in the `certs/` directory.
