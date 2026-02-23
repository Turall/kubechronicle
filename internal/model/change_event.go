package model

import "time"

// ChangeEvent represents a single Kubernetes resource change or exec operation.
type ChangeEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Operation   string    `json:"operation"` // CREATE, UPDATE, DELETE, EXEC
	ResourceKind string   `json:"resource_kind"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Actor       Actor     `json:"actor"`
	Source      Source    `json:"source"`
	Diff        []PatchOp `json:"diff,omitempty"`
	ObjectSnapshot map[string]interface{} `json:"object_snapshot,omitempty"` // For DELETE only
	Allowed     bool      `json:"allowed"` // Whether the operation was allowed (true) or blocked (false)
	BlockPattern string   `json:"block_pattern,omitempty"` // The pattern that blocked the request (if blocked)
	ExecMetadata *ExecMetadata `json:"exec_metadata,omitempty"` // For EXEC operations only
}

// ExecMetadata contains information about exec operations.
type ExecMetadata struct {
	Command     []string `json:"command,omitempty"`     // Command executed (if available)
	Container   string   `json:"container,omitempty"`   // Container name
	Stdin       bool     `json:"stdin"`                 // Whether stdin was used
	TTY         bool     `json:"tty"`                   // Whether TTY was allocated
	TargetType  string   `json:"target_type"`          // "pod" or "node"
	NodeName    string   `json:"node_name,omitempty"`   // Node name (for node exec)
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
