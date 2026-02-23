# Audit Log Processor

The audit log processor tracks `kubectl exec` operations (pod exec and node exec) by processing Kubernetes audit logs.

## Overview

Kubernetes admission webhooks only intercept resource mutations (CREATE, UPDATE, DELETE), not subresource operations like `exec`. To track exec operations, kubechronicle includes an audit log processor that:

1. Reads Kubernetes audit logs (from file or webhook)
2. Filters for exec operations (`/exec` subresource)
3. Extracts relevant metadata (command, container, TTY, stdin, etc.)
4. Stores exec events in the same database as resource changes

## Prerequisites

### Enable Kubernetes Audit Logging

You need to configure Kubernetes audit logging first. Add audit policy and webhook configuration to your kube-apiserver:

**1. Create audit policy file** (`/etc/kubernetes/audit-policy.yaml`):

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  # Log all exec operations
  - level: RequestResponse
    verbs: ["create"]
    resources:
      - group: ""
        resources: ["pods/exec", "nodes/proxy"]
```

**2. Configure kube-apiserver** to use audit logging:

```yaml
# Add to kube-apiserver manifest
spec:
  containers:
  - name: kube-apiserver
    command:
    - kube-apiserver
    - --audit-policy-file=/etc/kubernetes/audit-policy.yaml
    - --audit-webhook-config-file=/etc/kubernetes/audit-webhook-config.yaml
    # Or use log backend:
    # - --audit-log-path=/var/log/audit.log
    # - --audit-log-maxage=30
    # - --audit-log-maxbackup=10
    # - --audit-log-maxsize=100
```

**3. Create audit webhook configuration** (if using webhook backend):

```yaml
apiVersion: v1
kind: Config
clusters:
- name: kubechronicle-audit
  cluster:
    server: https://kubechronicle-audit:8444/audit
users:
- name: audit-webhook
contexts:
- context:
    cluster: kubechronicle-audit
    user: audit-webhook
  name: default
current-context: default
```

## Usage

### Option 1: Watch Audit Log File

If your Kubernetes cluster writes audit logs to a file:

```bash
./bin/audit-processor \
  -audit-log-file=/var/log/audit.log \
  -database-url="postgres://user:pass@localhost/kubechronicle"
```

### Option 2: Watch Audit Log Directory

If audit logs are written to multiple files in a directory:

```bash
./bin/audit-processor \
  -audit-log-dir=/var/log/audit \
  -database-url="postgres://user:pass@localhost/kubechronicle"
```

### Option 3: Receive via Webhook

If your Kubernetes cluster sends audit logs via webhook:

```bash
./bin/audit-processor \
  -enable-webhook \
  -webhook-port=8444 \
  -database-url="postgres://user:pass@localhost/kubechronicle"
```

Then configure Kubernetes to send audit logs to `https://your-service:8444/audit`.

## Command Line Options

- `-audit-log-file`: Path to Kubernetes audit log file to watch
- `-audit-log-dir`: Path to directory containing audit log files
- `-enable-webhook`: Enable HTTP webhook endpoint for receiving audit logs
- `-webhook-port`: Port for audit log webhook endpoint (default: 8444)
- `-database-url`: PostgreSQL connection string (or use `DATABASE_URL` env var)

## Exec Event Structure

Exec events are stored with `operation='EXEC'` and include:

```json
{
  "id": "EXEC-Pod-my-pod-user@example.com-1234567890",
  "timestamp": "2024-01-20T10:30:00Z",
  "operation": "EXEC",
  "resource_kind": "Pod",
  "namespace": "default",
  "name": "my-pod",
  "actor": {
    "username": "user@example.com",
    "groups": ["system:authenticated"],
    "source_ip": "10.0.0.1"
  },
  "source": {
    "tool": "kubectl"
  },
  "exec_metadata": {
    "command": ["/bin/sh"],
    "container": "my-container",
    "stdin": true,
    "tty": true,
    "target_type": "pod"
  },
  "allowed": true
}
```

For node exec operations:

```json
{
  "resource_kind": "Node",
  "namespace": "",
  "name": "node-1",
  "exec_metadata": {
    "target_type": "node",
    "node_name": "node-1"
  }
}
```

## Querying Exec Events

### Get all exec operations:

```sql
SELECT * FROM change_events 
WHERE operation = 'EXEC'
ORDER BY timestamp DESC;
```

### Get exec operations for a specific pod:

```sql
SELECT * FROM change_events 
WHERE operation = 'EXEC' 
  AND resource_kind = 'Pod' 
  AND namespace = 'default' 
  AND name = 'my-pod'
ORDER BY timestamp DESC;
```

### Get exec operations by user:

```sql
SELECT * FROM change_events 
WHERE operation = 'EXEC' 
  AND actor->>'username' = 'user@example.com'
ORDER BY timestamp DESC;
```

### Get exec operations with specific command:

```sql
SELECT * FROM change_events 
WHERE operation = 'EXEC' 
  AND exec_metadata->>'command' LIKE '%rm%'
ORDER BY timestamp DESC;
```

## Deployment

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubechronicle-audit-processor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubechronicle-audit-processor
  template:
    metadata:
      labels:
        app: kubechronicle-audit-processor
    spec:
      containers:
      - name: audit-processor
        image: kubechronicle/audit-processor:latest
        args:
        - -enable-webhook
        - -webhook-port=8444
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: kubechronicle-db
              key: url
        volumeMounts:
        # If using file-based audit logs, mount the audit log directory
        - name: audit-logs
          mountPath: /var/log/audit
          readOnly: true
      volumes:
      - name: audit-logs
        hostPath:
          path: /var/log/audit
          type: Directory
```

## Limitations

1. **Command Content**: The actual commands executed inside containers are not captured by Kubernetes audit logs. Only the exec session initiation is logged.

2. **Audit Log Availability**: Requires Kubernetes audit logging to be enabled and properly configured.

3. **Performance**: Processing large audit log files can be resource-intensive. Consider using log rotation and archiving.

4. **Security**: Audit logs may contain sensitive information. Ensure proper access controls and encryption.

## Troubleshooting

### No exec events appearing

1. Verify audit logging is enabled: `kubectl get apiserver -o yaml | grep audit`
2. Check audit log file exists and is readable
3. Verify exec operations are being logged: `grep -i exec /var/log/audit.log`
4. Check processor logs: `kubectl logs -l app=kubechronicle-audit-processor`

### High memory usage

- Reduce queue size in `service.go` (default: 1000)
- Process audit logs in batches
- Use log rotation to limit file size

### Missing metadata

- Some exec metadata (like command) may not be available in audit logs
- Check Kubernetes audit policy level (use `RequestResponse` for full details)
