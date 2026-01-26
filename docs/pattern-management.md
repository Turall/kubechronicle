# Pattern Management (Admin Feature)

Admins can now manage ignore and block patterns directly from the UI without needing to update Kubernetes deployments.

## Overview

- **Admin-only**: Only users with the `admin` role can access pattern management
- **Real-time updates**: Changes are immediately reflected in the webhook
- **ConfigMap-based**: Patterns are stored in a Kubernetes ConfigMap
- **UI-driven**: No need to edit YAML files or restart pods

## Accessing Pattern Management

1. Log in as an admin user
2. Navigate to the "Patterns" link in the navigation bar
3. You'll see two tabs:
   - **Ignore Patterns**: Resources matching these patterns are ignored (not tracked)
   - **Block Patterns**: Resources matching these patterns are blocked (denied)

## API Endpoints

### Get Ignore Patterns
```bash
GET /api/admin/patterns/ignore
Authorization: Bearer <admin-token>
```

### Update Ignore Patterns
```bash
PUT /api/admin/patterns/ignore
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "namespace_patterns": ["kube-*", "cert-manager"],
  "name_patterns": ["*-controller"],
  "resource_kind_patterns": ["ConfigMap"]
}
```

### Get Block Patterns
```bash
GET /api/admin/patterns/block
Authorization: Bearer <admin-token>
```

### Update Block Patterns
```bash
PUT /api/admin/patterns/block
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "namespace_patterns": ["production"],
  "name_patterns": [],
  "resource_kind_patterns": [],
  "operation_patterns": ["DELETE"],
  "message": "Deleting resources in production is not allowed"
}
```

## Pattern Syntax

Patterns support wildcards:
- `*` matches any sequence of characters (including empty)
- Exact matches if no wildcards

**Examples:**
- `kube-*` matches `kube-system`, `kube-public`, `kube-node-lease`
- `*-controller` matches `deployment-controller`, `namespace-controller`
- `*system*` matches `system-config`, `my-system-ns`, `system`
- `cert-manager` matches exactly `cert-manager`

## How It Works

1. **Storage**: Patterns are stored in a Kubernetes ConfigMap (`kubechronicle-patterns` by default)
2. **API Server**: Has RBAC permissions to read/update the ConfigMap
3. **Webhook**: Reads patterns from the ConfigMap via environment variables
4. **Updates**: When patterns are updated via API, the ConfigMap is updated, and the webhook picks up the changes (webhook pods may need to restart to pick up changes immediately, or you can configure a file watcher)

## Security

- **Authentication Required**: All endpoints require authentication
- **Admin Role Required**: Only users with `admin` role can access these endpoints
- **RBAC**: API server has minimal permissions (only for the patterns ConfigMap)
- **Validation**: Patterns are validated before being saved

## Helm Configuration

Pattern management is enabled by default. Configuration in `values.yaml`:

```yaml
patterns:
  # ConfigMap name for storing patterns
  configMapName: kubechronicle-patterns
  # Enable RBAC for API server to manage ConfigMaps
  rbac:
    create: true
```

## Troubleshooting

### "Forbidden: insufficient permissions"
- Ensure your user has the `admin` role
- Check that authentication is enabled
- Verify your JWT token is valid

### "Failed to update configuration"
- Check that the API server has RBAC permissions
- Verify the ConfigMap exists
- Check API server logs for detailed error messages

### Changes not taking effect
- Webhook pods read patterns at startup
- Restart webhook pods to pick up changes immediately:
  ```bash
  kubectl rollout restart deployment/kubechronicle-webhook -n kubechronicle
  ```
- Or wait for pods to restart naturally (they'll pick up the new ConfigMap values)

## Example: Complete Workflow

1. **Login as admin:**
   ```bash
   curl -X POST http://api-url/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"password"}'
   ```

2. **Get current ignore patterns:**
   ```bash
   curl http://api-url/api/admin/patterns/ignore \
     -H "Authorization: Bearer <token>"
   ```

3. **Update ignore patterns:**
   ```bash
   curl -X PUT http://api-url/api/admin/patterns/ignore \
     -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     -d '{
       "namespace_patterns": ["kube-*", "cert-manager"],
       "name_patterns": ["*-controller"],
       "resource_kind_patterns": []
     }'
   ```

4. **Restart webhook pods (if needed):**
   ```bash
   kubectl rollout restart deployment/kubechronicle-webhook -n kubechronicle
   ```

## UI Features

- **Tabbed Interface**: Switch between Ignore and Block patterns
- **Add/Remove Patterns**: Easy pattern management
- **Operation Selection**: For block patterns, select which operations to block (CREATE, UPDATE, DELETE)
- **Block Message**: Customize the error message shown when resources are blocked
- **Real-time Feedback**: Success/error messages for all operations
