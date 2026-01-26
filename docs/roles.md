# Role-Based Access Control (RBAC)

## Current Roles

### `viewer` Role
- **Access**: Read-only access to all data
- **Permissions**:
  - View change events
  - View resource history
  - View user activity
  - Cannot modify any data

### `admin` Role
- **Access**: Full access (currently same as viewer, but extensible)
- **Permissions**:
  - All viewer permissions
  - Future: Admin-only endpoints (user management, configuration, etc.)

## Current Implementation Status

All authenticated users (regardless of role) can access:
- `GET /api/changes` - List all changes
- `GET /api/changes/{id}` - Get specific change
- `GET /api/resources/{kind}/{namespace}/{name}/history` - Resource history
- `GET /api/users/{username}/activity` - User activity

## Implementing Role-Based Restrictions

To enforce different permissions, you can wrap endpoints with role requirements:

### Example: Admin-Only Endpoint

```go
// In cmd/api/main.go

// Admin-only endpoint (future)
adminMux := http.NewServeMux()
adminMux.HandleFunc("/api/admin/users", adminUserHandler)
adminMux.HandleFunc("/api/admin/config", adminConfigHandler)

// Wrap with admin role requirement
mux.Handle("/api/admin/", authenticator.RequireRole("admin")(adminMux))
```

### Example: Require Any Role

```go
// Allow both admin and viewer
mux.Handle("/api/changes", authenticator.RequireAnyRole("admin", "viewer")(apiServer.HandleListChanges))
```

### Example: Viewer-Only Endpoint

```go
// Only viewers can access (admins excluded)
mux.Handle("/api/readonly/", authenticator.RequireRole("viewer")(readonlyHandler))
```

## Recommended Role Structure

### Option 1: Simple (Current)
- `viewer`: Read-only access
- `admin`: Full access (includes viewer permissions)

### Option 2: Granular (Future)
- `viewer`: Read-only access to all data
- `editor`: Can modify some resources
- `admin`: Full administrative access
- `auditor`: Read-only access with audit logs

## Usage Examples

### Creating Users with Roles

```json
{
  "admin": {
    "password": "$2a$10$...",
    "roles": ["admin", "viewer"],
    "email": "admin@example.com"
  },
  "viewer": {
    "password": "$2a$10$...",
    "roles": ["viewer"]
  }
}
```

### Checking User Role in Code

```go
import "github.com/kubechronicle/kubechronicle/internal/auth"

func myHandler(w http.ResponseWriter, r *http.Request) {
    user, ok := auth.GetUser(r)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Check if user has admin role
    isAdmin := false
    for _, role := range user.Roles {
        if role == "admin" {
            isAdmin = true
            break
        }
    }
    
    if isAdmin {
        // Admin-only logic
    } else {
        // Viewer logic
    }
}
```

## Future Enhancements

1. **Namespace-based access**: Restrict access by Kubernetes namespace
2. **Resource-level permissions**: Fine-grained control per resource type
3. **Time-based restrictions**: Limit access during certain hours
4. **IP-based restrictions**: Allow access only from specific IPs
5. **Audit logging**: Track which roles accessed which endpoints
