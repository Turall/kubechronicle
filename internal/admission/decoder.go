package admission

import (
	"encoding/json"
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"

	"github.com/kubechronicle/kubechronicle/internal/diff"
	"github.com/kubechronicle/kubechronicle/internal/model"
)

// Decoder extracts information from Kubernetes AdmissionRequest.
type Decoder struct{}

// NewDecoder creates a new decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// DecodeRequest extracts all required information from an AdmissionRequest.
func (d *Decoder) DecodeRequest(req *admissionv1.AdmissionRequest) (*model.ChangeEvent, error) {
	event := &model.ChangeEvent{
		Operation:    string(req.Operation),
		ResourceKind: req.Kind.Kind,
		Namespace:    req.Namespace,
		Name:         req.Name,
		Actor: model.Actor{
			Username: req.UserInfo.Username,
			Groups:   req.UserInfo.Groups,
		},
		Source: model.Source{
			Tool: d.detectSourceTool(req),
		},
	}

	// Extract service account if present
	if req.UserInfo.Username != "" {
		// Check if username is a service account
		// Format: system:serviceaccount:namespace:name
		username := req.UserInfo.Username
		if strings.HasPrefix(username, "system:serviceaccount") {
			event.Actor.ServiceAccount = username
		}
	}

	// Handle source IP from extra fields (if available)
	if req.UserInfo.Extra != nil {
		if remoteAddr, ok := req.UserInfo.Extra["authentication.kubernetes.io/remote-addr"]; ok && len(remoteAddr) > 0 {
			event.Actor.SourceIP = remoteAddr[0]
		}
	}

	// Decode oldObject (for UPDATE/DELETE)
	var oldObj map[string]interface{}
	if req.OldObject.Raw != nil {
		if err := json.Unmarshal(req.OldObject.Raw, &oldObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal oldObject: %w", err)
		}

		// For DELETE operations, extract name from oldObject if req.Name is empty
		if req.Operation == admissionv1.Delete && event.Name == "" {
			if metadata, ok := oldObj["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok && name != "" {
					event.Name = name
				}
			}
		}

		// For DELETE, store filtered snapshot (remove noise fields)
		if req.Operation == admissionv1.Delete {
			event.ObjectSnapshot = d.filterSnapshot(oldObj, event.ResourceKind)
		}
	}

	// Decode object (for CREATE/UPDATE)
	var newObj map[string]interface{}
	if req.Object.Raw != nil {
		if err := json.Unmarshal(req.Object.Raw, &newObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal object: %w", err)
		}
	}

	// Compute diff for UPDATE operations
	if req.Operation == admissionv1.Update && oldObj != nil && newObj != nil {
		patches, err := diff.ComputeDiff(oldObj, newObj, event.ResourceKind)
		if err != nil {
			// Error computing diff - continue without diff rather than failing
			// This ensures we still record the event even if diff computation fails
			event.Diff = nil
			// Note: Error logging happens in handler if needed
		} else {
			event.Diff = patches
		}
	}

	return event, nil
}

// detectSourceTool attempts to identify the tool that made the change.
func (d *Decoder) detectSourceTool(req *admissionv1.AdmissionRequest) string {
	// Check for Helm annotation in object
	if req.Object.Raw != nil {
		var obj map[string]interface{}
		if err := json.Unmarshal(req.Object.Raw, &obj); err == nil {
			if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
				if labels, ok := metadata["labels"].(map[string]interface{}); ok {
					if managedBy, ok := labels["app.kubernetes.io/managed-by"].(string); ok && managedBy == "Helm" {
						return "helm"
					}
				}
			}
		}
	}

	// Check user agent or other indicators
	username := req.UserInfo.Username
	if username == "system:kube-controller-manager" {
		return "controller"
	}
	if username == "system:kube-scheduler" {
		return "controller"
	}
	// Check for service accounts (format: system:serviceaccount:namespace:name)
	if strings.HasPrefix(username, "system:serviceaccount") {
		return "controller"
	}

	// Default to kubectl for human users
	if username != "" && username[:7] != "system:" {
		return "kubectl"
	}

	return "unknown"
}

// DecodeAdmissionReview decodes a raw AdmissionReview request.
func (d *Decoder) DecodeAdmissionReview(body []byte) (*admissionv1.AdmissionReview, error) {
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AdmissionReview: %w", err)
	}

	// Ensure we're using v1 API
	if review.APIVersion != "admission.k8s.io/v1" {
		return nil, fmt.Errorf("unsupported API version: %s, expected admission.k8s.io/v1", review.APIVersion)
	}

	return &review, nil
}

// GetObjectRaw returns the raw JSON bytes of the object.
func (d *Decoder) GetObjectRaw(req *admissionv1.AdmissionRequest) []byte {
	return req.Object.Raw
}

// GetOldObjectRaw returns the raw JSON bytes of the old object.
func (d *Decoder) GetOldObjectRaw(req *admissionv1.AdmissionRequest) []byte {
	return req.OldObject.Raw
}

// filterSnapshot filters out ignored fields from a DELETE snapshot.
// This reduces storage size by removing Kubernetes noise fields.
func (d *Decoder) filterSnapshot(obj map[string]interface{}, resourceKind string) map[string]interface{} {
	// Use the same filtering logic as diff computation
	filtered := diff.FilterIgnoredFields(obj, "").(map[string]interface{})

	// Hash Secret values if this is a Secret resource
	if resourceKind == "Secret" {
		filtered = diff.HashSecretValues(filtered).(map[string]interface{})
	}

	return filtered
}
