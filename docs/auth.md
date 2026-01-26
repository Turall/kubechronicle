# Authentication and Authorization

kubechronicle supports JWT-based authentication and role-based access control (RBAC).

## Overview

- **Authentication**: JWT tokens issued after successful login
- **Authorization**: Role-based access control (RBAC)
- **Default**: Authentication is **disabled** by default (all requests allowed)

## Enabling Authentication

### 1. Generate JWT Secret

```bash
# Generate a random secret (32 bytes, base64 encoded)
openssl rand -base64 32

# Or use kubechronicle's password-hash tool (it can generate secrets too)
go run ./cmd/password-hash/main.go | base64
```

### 2. Create User Passwords

Generate bcrypt hashes for user passwords:

```bash
# Build the password-hash tool
go build -o bin/password-hash ./cmd/password-hash

# Generate a password hash
./bin/password-hash "your-secure-password"
# Output: $2a$10$...
```

### 3. Create Kubernetes Secrets

```bash
# Create auth secret with JWT secret and users
kubectl create secret generic kubechronicle-auth \
  --from-literal=jwt-secret="<your-jwt-secret>" \
  --from-literal=users='{"admin":{"password":"$2a$10$...","roles":["admin","viewer"],"email":"admin@example.com"},"viewer":{"password":"$2a$10$...","roles":["viewer"]}}' \
  --namespace kubechronicle
```

### 4. Configure Helm Values

```yaml
api:
  auth:
    enabled: true
    jwtSecret:
      secretName: kubechronicle-auth
      secretKey: jwt-secret
    jwtExpirationHours: 24
    users:
      secretName: kubechronicle-auth
      secretKey: users
```

### 5. Deploy

```bash
helm upgrade --install kubechronicle ./helm/kubechronicle \
  --namespace kubechronicle \
  --set api.auth.enabled=true
```

## User Configuration Format

Users are configured as JSON in the Kubernetes Secret:

```json
{
  "username": {
    "password": "$2a$10$...",  // bcrypt hash
    "roles": ["admin", "viewer"],
    "email": "user@example.com"  // optional
  }
}
```

### Roles

- **admin**: Full access (currently same as viewer, but extensible for admin-only endpoints)
- **viewer**: Read-only access (default for all users)

**⚠️ Important**: Currently, both `admin` and `viewer` roles have **identical access**. The role system is in place, but no endpoints enforce role restrictions yet. All authenticated users can access all endpoints regardless of role. See [roles.md](./roles.md) for details on implementing role-based restrictions.

## API Usage

### Login

```bash
curl -X POST http://api-url/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-password"}'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "username": "admin",
    "roles": ["admin", "viewer"],
    "email": "admin@example.com"
  }
}
```

### Using the Token

Include the token in the `Authorization` header:

```bash
curl http://api-url/api/changes \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Endpoints

### Public Endpoints (No Auth Required)

- `GET /health` - Health check
- `POST /api/auth/login` - Login endpoint

### Protected Endpoints (Require Auth)

- `GET /api/changes` - List changes
- `GET /api/changes/{id}` - Get change by ID
- `GET /api/resources/{kind}/{namespace}/{name}/history` - Resource history
- `GET /api/users/{username}/activity` - User activity

## Authorization

Currently, all authenticated users have access to all endpoints. Role-based restrictions can be added by:

1. Using `RequireRole()` middleware for specific endpoints
2. Implementing namespace-based access control
3. Adding resource-level permissions

Example (future enhancement):
```go
// Require admin role for admin endpoints
mux.Handle("/api/admin/", authenticator.RequireRole("admin")(adminHandler))
```

## Security Best Practices

1. **Use Strong Passwords**: Generate secure passwords for users
2. **Rotate JWT Secrets**: Regularly rotate JWT secrets
3. **HTTPS Only**: Always use HTTPS in production
4. **Token Expiration**: Use reasonable token expiration times (default: 24 hours)
5. **Store Secrets Securely**: Use Kubernetes Secrets, not ConfigMaps
6. **Network Policies**: Restrict API access using NetworkPolicies
7. **Monitor Access**: Log authentication attempts and failures

## Troubleshooting

### "Authorization header required"

- Ensure authentication is enabled
- Include `Authorization: Bearer <token>` header
- Verify token hasn't expired

### "Invalid or expired token"

- Token may have expired (check `JWT_EXPIRATION_HOURS`)
- Token may be invalid (verify JWT_SECRET matches)
- Request a new token via `/api/auth/login`

### "Invalid credentials"

- Verify username exists in users configuration
- Verify password hash is correct (use password-hash tool)
- Check users JSON format is valid

## Example: Complete Setup

```bash
# 1. Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32)

# 2. Generate password hashes
ADMIN_PASS_HASH=$(./bin/password-hash "admin123")
VIEWER_PASS_HASH=$(./bin/password-hash "viewer123")

# 3. Create users JSON
USERS_JSON=$(cat <<EOF
{
  "admin": {
    "password": "${ADMIN_PASS_HASH}",
    "roles": ["admin", "viewer"],
    "email": "admin@example.com"
  },
  "viewer": {
    "password": "${VIEWER_PASS_HASH}",
    "roles": ["viewer"]
  }
}
EOF
)

# 4. Create secret
kubectl create secret generic kubechronicle-auth \
  --from-literal=jwt-secret="${JWT_SECRET}" \
  --from-literal=users="${USERS_JSON}" \
  --namespace kubechronicle

# 5. Deploy with auth enabled
helm upgrade --install kubechronicle ./helm/kubechronicle \
  --namespace kubechronicle \
  --set api.auth.enabled=true
```
