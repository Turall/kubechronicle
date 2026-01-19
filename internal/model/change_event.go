package model

import "time"

// ChangeEvent represents a single Kubernetes resource change.
type ChangeEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Operation   string    `json:"operation"` // CREATE, UPDATE, DELETE
	ResourceKind string   `json:"resource_kind"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Actor       Actor     `json:"actor"`
	Source      Source    `json:"source"`
	Diff        []PatchOp `json:"diff,omitempty"`
	ObjectSnapshot map[string]interface{} `json:"object_snapshot,omitempty"` // For DELETE only
	Allowed     bool      `json:"allowed"` // Whether the operation was allowed (true) or blocked (false)
	BlockPattern string   `json:"block_pattern,omitempty"` // The pattern that blocked the request (if blocked)
}

// Actor represents who made the change.
type Actor struct {
	Username       string   `json:"username"`
	Groups         []string `json:"groups"`
	ServiceAccount string   `json:"service_account,omitempty"`
	SourceIP       string   `json:"source_ip"`
}

// Source identifies the tool that made the change.
type Source struct {
	Tool string `json:"tool"` // kubectl, helm, controller, unknown
}

// PatchOp represents a single RFC 6902 patch operation.
type PatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}
