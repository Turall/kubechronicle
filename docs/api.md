# kubechronicle REST API

The kubechronicle API server provides read-only endpoints for querying change event history.

## Endpoints

### GET /api/changes

List change events with optional filters, pagination, and sorting.

**Query Parameters:**
- `resource_kind` (string, optional): Filter by resource kind (e.g., "Deployment", "ConfigMap")
- `namespace` (string, optional): Filter by namespace
- `name` (string, optional): Filter by resource name
- `user` (string, optional): Filter by username
- `operation` (string, optional): Filter by operation ("CREATE", "UPDATE", "DELETE")
- `start_time` (string, optional): Filter by start time (RFC3339 format, e.g., "2024-01-19T00:00:00Z")
- `end_time` (string, optional): Filter by end time (RFC3339 format)
- `allowed` (boolean, optional): Filter by allowed status (true/false)
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Offset for pagination (default: 0)
- `sort` (string, optional): Sort order ("asc" or "desc", default: "desc")

**Response:**
```json
{
  "events": [
    {
      "id": "CREATE-Deployment-test-1234567890",
      "timestamp": "2024-01-19T10:00:00Z",
      "operation": "CREATE",
      "resource_kind": "Deployment",
      "namespace": "default",
      "name": "test",
      "actor": {
        "username": "user@example.com",
        "groups": ["system:authenticated"],
        "service_account": "",
        "source_ip": "10.0.0.1"
      },
      "source": {
        "tool": "kubectl"
      },
      "diff": [
        {
          "op": "add",
          "path": "/spec/replicas",
          "value": 3
        }
      ],
      "allowed": true,
      "block_pattern": ""
    }
  ],
  "total": 100,
  "limit": 50,
  "offset": 0
}
```

**Example:**
```bash
curl "http://localhost:8080/api/changes?namespace=default&operation=CREATE&limit=10"
```

### GET /api/changes/{id}

Get a specific change event by ID.

**Path Parameters:**
- `id` (string, required): Change event ID

**Response:**
```json
{
  "id": "CREATE-Deployment-test-1234567890",
  "timestamp": "2024-01-19T10:00:00Z",
  "operation": "CREATE",
  "resource_kind": "Deployment",
  "namespace": "default",
  "name": "test",
  "actor": { ... },
  "source": { ... },
  "diff": [ ... ],
  "allowed": true
}
```

**Example:**
```bash
curl "http://localhost:8080/api/changes/CREATE-Deployment-test-1234567890"
```

### GET /api/resources/{kind}/{namespace}/{name}/history

Get change history for a specific resource.

**Path Parameters:**
- `kind` (string, required): Resource kind (e.g., "Deployment", "ConfigMap")
- `namespace` (string, required): Namespace (use "-" for cluster-scoped resources)
- `name` (string, required): Resource name

**Query Parameters:**
- `limit` (integer, optional): Number of results per page (default: 50)
- `offset` (integer, optional): Offset for pagination (default: 0)
- `sort` (string, optional): Sort order ("asc" or "desc", default: "desc")

**Response:**
Same format as `GET /api/changes`

**Example:**
```bash
# URL-encode special characters in namespace/name if needed
curl "http://localhost:8080/api/resources/Deployment/default/my-app/history?limit=20"
```

### GET /api/users/{username}/activity

Get change events for a specific user.

**Path Parameters:**
- `username` (string, required): Username (URL-encoded if needed)

**Query Parameters:**
- `limit` (integer, optional): Number of results per page (default: 50)
- `offset` (integer, optional): Offset for pagination (default: 0)
- `sort` (string, optional): Sort order ("asc" or "desc", default: "desc")

**Response:**
Same format as `GET /api/changes`

**Example:**
```bash
curl "http://localhost:8080/api/users/user%40example.com/activity?limit=10"
```

## Running the API Server

```bash
# Build the API server
make build-api

# Run the API server (requires DATABASE_URL environment variable)
export DATABASE_URL="postgres://user:password@localhost:5432/kubechronicle?sslmode=disable"
./bin/api -port=8080
```

Or run directly:
```bash
go run ./cmd/api -port=8080
```

## CORS

The API server includes CORS headers to allow cross-origin requests from web browsers. All endpoints support OPTIONS preflight requests.

## Error Responses

All errors are returned in JSON format:

```json
{
  "error": "Error message here"
}
```

Common HTTP status codes:
- `200 OK`: Success
- `400 Bad Request`: Invalid request parameters
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## Notes

- All endpoints are read-only
- Secret values are never returned (they are hashed in the database)
- Pagination is recommended for large result sets
- Default sort order is descending (newest first)
- All timestamps are in UTC and RFC3339 format
