package audit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// AuditEvent represents a Kubernetes audit log event.
type AuditEvent struct {
	Level      string                 `json:"level"`
	AuditID    string                 `json:"auditID"`
	Stage      string                 `json:"stage"`
	RequestURI string                 `json:"requestURI"`
	Verb       string                 `json:"verb"`
	User       AuditUser              `json:"user"`
	SourceIPs  []string               `json:"sourceIPs"`
	ObjectRef  *AuditObjectRef        `json:"objectRef"`
	RequestObject map[string]interface{} `json:"requestObject,omitempty"`
	ResponseStatus *AuditResponseStatus `json:"responseStatus,omitempty"`
	RequestReceivedTimestamp time.Time `json:"requestReceivedTimestamp"`
}

// AuditUser represents user information in audit logs.
type AuditUser struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
	UID      string   `json:"uid"`
}

// AuditObjectRef represents object reference in audit logs.
type AuditObjectRef struct {
	Resource    string `json:"resource"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	APIVersion  string `json:"apiVersion"`
	APIGroup    string `json:"apiGroup"`
	Subresource string `json:"subresource"`
}

// AuditResponseStatus represents response status in audit logs.
type AuditResponseStatus struct {
	Code int `json:"code"`
}

// Processor processes Kubernetes audit logs and extracts exec operations.
type Processor struct{}

// NewProcessor creates a new audit log processor.
func NewProcessor() *Processor {
	return &Processor{}
}

// ParseAuditLog parses a single audit log line and returns an AuditEvent.
func (p *Processor) ParseAuditLog(line []byte) (*AuditEvent, error) {
	var event AuditEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit log: %w", err)
	}
	return &event, nil
}

// IsExecOperation checks if an audit event represents an exec operation.
func (p *Processor) IsExecOperation(event *AuditEvent) bool {
	if event.ObjectRef == nil {
		return false
	}

	// Check for pod exec: /api/v1/namespaces/{namespace}/pods/{name}/exec
	// Check for node exec: /api/v1/nodes/{name}/proxy/exec
	if event.ObjectRef.Subresource == "exec" {
		return true
	}

	// Also check requestURI for exec patterns
	if strings.Contains(event.RequestURI, "/exec") {
		return true
	}

	return false
}

// ExtractExecEvent converts an audit event to a ChangeEvent for exec operations.
func (p *Processor) ExtractExecEvent(event *AuditEvent) (*model.ChangeEvent, error) {
	if !p.IsExecOperation(event) {
		return nil, fmt.Errorf("not an exec operation")
	}

	execEvent := &model.ChangeEvent{
		Operation:    "EXEC",
		Timestamp:    event.RequestReceivedTimestamp,
		Allowed:      true,
		BlockPattern: "",
	}

	// Extract actor information
	execEvent.Actor = model.Actor{
		Username: event.User.Username,
		Groups:   event.User.Groups,
	}

	if len(event.SourceIPs) > 0 {
		execEvent.Actor.SourceIP = event.SourceIPs[0]
	}

	// Check if username is a service account
	if strings.HasPrefix(event.User.Username, "system:serviceaccount") {
		execEvent.Actor.ServiceAccount = event.User.Username
	}

	// Extract resource information
	if event.ObjectRef != nil {
		execEvent.ResourceKind = event.ObjectRef.Resource
		execEvent.Namespace = event.ObjectRef.Namespace
		execEvent.Name = event.ObjectRef.Name
	} else {
		// Fallback: parse from RequestURI
		execEvent.ResourceKind = "Pod"
		if err := p.parseExecURI(event.RequestURI, execEvent); err != nil {
			klog.V(3).Infof("Failed to parse exec URI: %v", err)
		}
	}

	// Extract exec metadata
	execMetadata := &model.ExecMetadata{
		TargetType: "pod",
		Stdin:      false,
		TTY:        false,
	}

	// Determine if this is a node exec or pod exec
	if strings.Contains(event.RequestURI, "/nodes/") && strings.Contains(event.RequestURI, "/proxy/exec") {
		execMetadata.TargetType = "node"
		// Extract node name from URI
		parts := strings.Split(event.RequestURI, "/nodes/")
		if len(parts) > 1 {
			nodeParts := strings.Split(parts[1], "/")
			if len(nodeParts) > 0 {
				execMetadata.NodeName = nodeParts[0]
			}
		}
		execEvent.ResourceKind = "Node"
		execEvent.Namespace = "" // Nodes are cluster-scoped
		if execEvent.Name == "" {
			execEvent.Name = execMetadata.NodeName
		}
	}

	// Extract container name from query parameters or request object
	if event.RequestObject != nil {
		if container, ok := event.RequestObject["container"].(string); ok && container != "" {
			execMetadata.Container = container
		}
		if stdin, ok := event.RequestObject["stdin"].(bool); ok {
			execMetadata.Stdin = stdin
		}
		if tty, ok := event.RequestObject["tty"].(bool); ok {
			execMetadata.TTY = tty
		}
		if command, ok := event.RequestObject["command"].([]interface{}); ok {
			execMetadata.Command = make([]string, 0, len(command))
			for _, cmd := range command {
				if str, ok := cmd.(string); ok {
					execMetadata.Command = append(execMetadata.Command, str)
				}
			}
		}
	}

	// Parse query parameters from RequestURI
	if err := p.parseExecQueryParams(event.RequestURI, execMetadata); err != nil {
		klog.V(3).Infof("Failed to parse exec query params: %v", err)
	}

	execEvent.ExecMetadata = execMetadata

	// Detect source tool
	execEvent.Source = model.Source{
		Tool: p.detectSourceTool(event),
	}

	// Generate event ID
	execEvent.ID = p.generateEventID(execEvent)

	return execEvent, nil
}

// parseExecURI extracts namespace, name, and container from exec URI.
func (p *Processor) parseExecURI(uri string, event *model.ChangeEvent) error {
	// Format: /api/v1/namespaces/{namespace}/pods/{name}/exec
	// Or: /api/v1/nodes/{name}/proxy/exec
	parts := strings.Split(uri, "/")
	
	for i, part := range parts {
		if part == "namespaces" && i+1 < len(parts) {
			event.Namespace = parts[i+1]
		}
		if part == "pods" && i+1 < len(parts) {
			event.Name = parts[i+1]
			event.ResourceKind = "Pod"
		}
		if part == "nodes" && i+1 < len(parts) {
			event.Name = parts[i+1]
			event.ResourceKind = "Node"
		}
	}
	return nil
}

// parseExecQueryParams extracts query parameters from exec URI.
func (p *Processor) parseExecQueryParams(uri string, metadata *model.ExecMetadata) error {
	// Extract query string
	parts := strings.Split(uri, "?")
	if len(parts) < 2 {
		return nil
	}

	query := parts[1]
	params := strings.Split(query, "&")
	
	for _, param := range params {
		kv := strings.Split(param, "=")
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := kv[1]

		switch key {
		case "container":
			if metadata.Container == "" {
				metadata.Container = value
			}
		case "stdin":
			metadata.Stdin = (value == "true")
		case "tty":
			metadata.TTY = (value == "true")
		case "command":
			// Command is typically URL-encoded, but we'll handle it as-is
			// In practice, command is usually in requestObject
			if len(metadata.Command) == 0 {
				metadata.Command = []string{value}
			}
		}
	}
	return nil
}

// detectSourceTool attempts to identify the tool that initiated the exec.
func (p *Processor) detectSourceTool(event *AuditEvent) string {
	username := event.User.Username
	
	// Check for service accounts (controllers)
	if strings.HasPrefix(username, "system:serviceaccount") {
		return "controller"
	}
	
	// Check for system users
	if strings.HasPrefix(username, "system:") {
		return "system"
	}
	
	// Default to kubectl for human users
	if username != "" {
		return "kubectl"
	}
	
	return "unknown"
}

// generateEventID generates a unique ID for an exec event.
func (p *Processor) generateEventID(event *model.ChangeEvent) string {
	return fmt.Sprintf("EXEC-%s-%s-%s-%d",
		event.ResourceKind,
		event.Name,
		event.Actor.Username,
		event.Timestamp.UnixNano(),
	)
}
