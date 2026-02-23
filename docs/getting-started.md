# Getting Started

## Prerequisites

- **Kubernetes cluster** (1.20+)
- **PostgreSQL database** (optional; webhook works without it)
- **openssl** for generating self-signed certificates (included in most systems)

## Deployment

### 1. Generate self-signed TLS certificates

No cert-manager is required.

```bash
cd deploy/webhook
./generate-certs.sh
# or: make generate-certs
```

### 2. Create database secret (optional)

```bash
make create-db-secret
```

### 3. Deploy kubechronicle

```bash
make deploy
```

### 4. Verify deployment

```bash
make verify
```

!!! note "Certificates"
    Self-signed certificates are the default and recommended approach. No cert-manager or CA certificates are required. See the webhook [Deployment](deployment.md) guide for more detail.

## Tracked resources

- **Core resources**: Deployments, StatefulSets, DaemonSets, Services, ConfigMaps, Secrets (hashed), Ingress
- **Custom Resource Definitions**: CRD definitions (CustomResourceDefinition resources)

To track CRD instances, add rules to `deploy/webhook/webhook.yaml` (Kubernetes does not support wildcards in webhook rules). See the root `README.md` and the `deploy/webhook/crd-examples.yaml` file in the repository for concrete examples.

## Ignored fields

The diff engine automatically ignores Kubernetes noise fields:

- `metadata.managedFields`
- `metadata.resourceVersion`
- `metadata.generation`
- `metadata.creationTimestamp`
- `status` (entire subtree)
- `kubectl.kubernetes.io/last-applied-configuration`

## Next steps

- [Deployment](deployment.md) — Full deployment guide and troubleshooting
- [API Reference](api.md) — Query change events via REST API
- [Exec Tracking](audit-processor.md) — Track pod/node exec via audit logs
