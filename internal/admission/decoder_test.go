package admission

import (
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewDecoder(t *testing.T) {
	decoder := NewDecoder()
	if decoder == nil {
		t.Error("NewDecoder() returned nil")
	}
}

func TestDecodeAdmissionReview(t *testing.T) {
	decoder := NewDecoder()

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
		},
	}

	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Failed to marshal review: %v", err)
	}

	decoded, err := decoder.DecodeAdmissionReview(body)
	if err != nil {
		t.Fatalf("DecodeAdmissionReview() error = %v", err)
	}
	if decoded == nil {
		t.Fatal("DecodeAdmissionReview() returned nil")
	}
	if decoded.APIVersion != "admission.k8s.io/v1" {
		t.Errorf("APIVersion = %s, want admission.k8s.io/v1", decoded.APIVersion)
	}
}

func TestDecodeAdmissionReview_InvalidVersion(t *testing.T) {
	decoder := NewDecoder()

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1beta1", // Wrong version
			Kind:       "AdmissionReview",
		},
	}

	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Failed to marshal review: %v", err)
	}

	_, err = decoder.DecodeAdmissionReview(body)
	if err == nil {
		t.Error("DecodeAdmissionReview() should return error for invalid version")
	}
}

func TestDecodeAdmissionReview_InvalidJSON(t *testing.T) {
	decoder := NewDecoder()

	_, err := decoder.DecodeAdmissionReview([]byte("invalid json"))
	if err == nil {
		t.Error("DecodeAdmissionReview() should return error for invalid JSON")
	}
}

func TestDecodeRequest_CREATE(t *testing.T) {
	decoder := NewDecoder()

	objectJSON := `{
		"metadata": {
			"name": "test-deployment",
			"namespace": "default"
		},
		"spec": {
			"replicas": 1
		}
	}`

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		Namespace: "default",
		Name:      "test-deployment",
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com",
			Groups:   []string{"system:authenticated"},
		},
		Object: runtime.RawExtension{
			Raw: []byte(objectJSON),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event == nil {
		t.Fatal("DecodeRequest() returned nil")
	}
	if event.Operation != "CREATE" {
		t.Errorf("Operation = %s, want CREATE", event.Operation)
	}
	if event.ResourceKind != "Deployment" {
		t.Errorf("ResourceKind = %s, want Deployment", event.ResourceKind)
	}
	if event.Namespace != "default" {
		t.Errorf("Namespace = %s, want default", event.Namespace)
	}
	if event.Name != "test-deployment" {
		t.Errorf("Name = %s, want test-deployment", event.Name)
	}
	if event.Actor.Username != "user@example.com" {
		t.Errorf("Actor.Username = %s, want user@example.com", event.Actor.Username)
	}
}

func TestDecodeRequest_UPDATE(t *testing.T) {
	decoder := NewDecoder()

	oldObjectJSON := `{
		"metadata": {"name": "test", "namespace": "default"},
		"spec": {"replicas": 1}
	}`
	newObjectJSON := `{
		"metadata": {"name": "test", "namespace": "default"},
		"spec": {"replicas": 3}
	}`

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Update,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		Namespace: "default",
		Name:      "test",
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com",
		},
		OldObject: runtime.RawExtension{
			Raw: []byte(oldObjectJSON),
		},
		Object: runtime.RawExtension{
			Raw: []byte(newObjectJSON),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event.Operation != "UPDATE" {
		t.Errorf("Operation = %s, want UPDATE", event.Operation)
	}
	// Should have computed diff
	if len(event.Diff) == 0 {
		t.Error("Expected diff for UPDATE operation")
	}
}

func TestDecodeRequest_DELETE(t *testing.T) {
	decoder := NewDecoder()

	oldObjectJSON := `{
		"metadata": {"name": "test", "namespace": "default"},
		"spec": {"replicas": 1}
	}`

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Delete,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		Namespace: "default",
		Name:      "test",
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com",
		},
		OldObject: runtime.RawExtension{
			Raw: []byte(oldObjectJSON),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event.Operation != "DELETE" {
		t.Errorf("Operation = %s, want DELETE", event.Operation)
	}
	// Should have object snapshot for DELETE
	if event.ObjectSnapshot == nil {
		t.Error("Expected ObjectSnapshot for DELETE operation")
	}
	if event.Name != "test" {
		t.Errorf("Name = %s, want test", event.Name)
	}
}

func TestDecodeRequest_DELETE_EmptyName(t *testing.T) {
	decoder := NewDecoder()

	// Test DELETE with empty req.Name - should extract from OldObject
	oldObjectJSON := `{
		"metadata": {"name": "test-deployment", "namespace": "cert-manager"},
		"spec": {"replicas": 1}
	}`

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Delete,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		Namespace: "cert-manager",
		Name:      "", // Empty name - should be extracted from OldObject
		UserInfo: authenticationv1.UserInfo{
			Username: "system:serviceaccount:kube-system:namespace-controller",
		},
		OldObject: runtime.RawExtension{
			Raw: []byte(oldObjectJSON),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event.Operation != "DELETE" {
		t.Errorf("Operation = %s, want DELETE", event.Operation)
	}
	// Should extract name from OldObject
	if event.Name != "test-deployment" {
		t.Errorf("Name = %s, want test-deployment", event.Name)
	}
	if event.ObjectSnapshot == nil {
		t.Error("Expected ObjectSnapshot for DELETE operation")
	}
}

func TestDecodeRequest_ServiceAccount(t *testing.T) {
	decoder := NewDecoder()

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		UserInfo: authenticationv1.UserInfo{
			Username: "system:serviceaccount:default:my-sa",
		},
		Object: runtime.RawExtension{
			Raw: []byte(`{"metadata": {"name": "test"}}`),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event.Actor.ServiceAccount != "system:serviceaccount:default:my-sa" {
		t.Errorf("ServiceAccount = %s, want system:serviceaccount:default:my-sa", event.Actor.ServiceAccount)
	}
}

func TestDecodeRequest_SourceIP(t *testing.T) {
	decoder := NewDecoder()

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Kind: "Deployment",
		},
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com",
			Extra: map[string]authenticationv1.ExtraValue{
				"authentication.kubernetes.io/remote-addr": {"192.168.1.1"},
			},
		},
		Object: runtime.RawExtension{
			Raw: []byte(`{"metadata": {"name": "test"}}`),
		},
	}

	event, err := decoder.DecodeRequest(req)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if event.Actor.SourceIP != "192.168.1.1" {
		t.Errorf("SourceIP = %s, want 192.168.1.1", event.Actor.SourceIP)
	}
}

func TestDetectSourceTool_Helm(t *testing.T) {
	decoder := NewDecoder()

	objectJSON := `{
		"metadata": {
			"labels": {
				"app.kubernetes.io/managed-by": "Helm"
			}
		}
	}`

	req := &admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: []byte(objectJSON),
		},
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com",
		},
	}

	tool := decoder.detectSourceTool(req)
	if tool != "helm" {
		t.Errorf("detectSourceTool() = %s, want helm", tool)
	}
}

func TestDetectSourceTool_Controller(t *testing.T) {
	decoder := NewDecoder()

	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "kube-controller-manager",
			username: "system:kube-controller-manager",
			want:     "controller",
		},
		{
			name:     "kube-scheduler",
			username: "system:kube-scheduler",
			want:     "controller",
		},
		{
			name:     "service account",
			username: "system:serviceaccount:kube-system:deployment-controller",
			want:     "controller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &admissionv1.AdmissionRequest{
				UserInfo: authenticationv1.UserInfo{
					Username: tt.username,
				},
			}
			tool := decoder.detectSourceTool(req)
			if tool != tt.want {
				t.Errorf("detectSourceTool() = %s, want %s", tool, tt.want)
			}
		})
	}
}

func TestDetectSourceTool_Kubectl(t *testing.T) {
	decoder := NewDecoder()

	req := &admissionv1.AdmissionRequest{
		UserInfo: authenticationv1.UserInfo{
			Username: "user@example.com", // Regular user
		},
	}

	tool := decoder.detectSourceTool(req)
	if tool != "kubectl" {
		t.Errorf("detectSourceTool() = %s, want kubectl", tool)
	}
}

func TestDetectSourceTool_Unknown(t *testing.T) {
	decoder := NewDecoder()

	req := &admissionv1.AdmissionRequest{
		UserInfo: authenticationv1.UserInfo{
			Username: "", // Empty username
		},
	}

	tool := decoder.detectSourceTool(req)
	if tool != "unknown" {
		t.Errorf("detectSourceTool() = %s, want unknown", tool)
	}
}

func TestGetObjectRaw(t *testing.T) {
	decoder := NewDecoder()

	rawData := []byte(`{"test": "data"}`)
	req := &admissionv1.AdmissionRequest{
		Object: runtime.RawExtension{
			Raw: rawData,
		},
	}

	result := decoder.GetObjectRaw(req)
	if len(result) != len(rawData) {
		t.Errorf("GetObjectRaw() returned %d bytes, want %d", len(result), len(rawData))
	}
}

func TestGetOldObjectRaw(t *testing.T) {
	decoder := NewDecoder()

	rawData := []byte(`{"test": "data"}`)
	req := &admissionv1.AdmissionRequest{
		OldObject: runtime.RawExtension{
			Raw: rawData,
		},
	}

	result := decoder.GetOldObjectRaw(req)
	if len(result) != len(rawData) {
		t.Errorf("GetOldObjectRaw() returned %d bytes, want %d", len(result), len(rawData))
	}
}

func TestDecodeRequest_InvalidJSON(t *testing.T) {
	decoder := NewDecoder()

	req := &admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: []byte("invalid json"),
		},
	}

	_, err := decoder.DecodeRequest(req)
	if err == nil {
		t.Error("DecodeRequest() should return error for invalid JSON")
	}
}
