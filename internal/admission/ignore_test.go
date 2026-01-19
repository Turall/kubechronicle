package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestShouldIgnore_NamespacePatterns(t *testing.T) {
	tests := []struct {
		name         string
		event        *model.ChangeEvent
		ignoreConfig *config.IgnoreConfig
		want         bool
	}{
		{
			name: "exact namespace match",
			event: &model.ChangeEvent{
				Namespace: "kube-system",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamespacePatterns: []string{"kube-system"},
			},
			want: true,
		},
		{
			name: "wildcard prefix match",
			event: &model.ChangeEvent{
				Namespace: "kube-system",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamespacePatterns: []string{"kube-*"},
			},
			want: true,
		},
		{
			name: "wildcard suffix match",
			event: &model.ChangeEvent{
				Namespace: "cert-manager",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamespacePatterns: []string{"*-manager"},
			},
			want: true,
		},
		{
			name: "wildcard contains match",
			event: &model.ChangeEvent{
				Namespace: "my-system-ns",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamespacePatterns: []string{"*system*"},
			},
			want: true,
		},
		{
			name: "no match",
			event: &model.ChangeEvent{
				Namespace: "default",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamespacePatterns: []string{"kube-*"},
			},
			want: false,
		},
		{
			name: "nil ignore config",
			event: &model.ChangeEvent{
				Namespace: "kube-system",
			},
			ignoreConfig: nil,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnore(tt.event, tt.ignoreConfig)
			if got != tt.want {
				t.Errorf("ShouldIgnore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_NamePatterns(t *testing.T) {
	tests := []struct {
		name         string
		event        *model.ChangeEvent
		ignoreConfig *config.IgnoreConfig
		want         bool
	}{
		{
			name: "exact name match",
			event: &model.ChangeEvent{
				Name: "test-deployment",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamePatterns: []string{"test-deployment"},
			},
			want: true,
		},
		{
			name: "wildcard prefix match",
			event: &model.ChangeEvent{
				Name: "kube-controller-manager",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamePatterns: []string{"kube-*"},
			},
			want: true,
		},
		{
			name: "wildcard suffix match",
			event: &model.ChangeEvent{
				Name: "my-controller",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamePatterns: []string{"*-controller"},
			},
			want: true,
		},
		{
			name: "multiple patterns",
			event: &model.ChangeEvent{
				Name: "test-deployment",
			},
			ignoreConfig: &config.IgnoreConfig{
				NamePatterns: []string{"prod-*", "test-*", "dev-*"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnore(tt.event, tt.ignoreConfig)
			if got != tt.want {
				t.Errorf("ShouldIgnore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_ResourceKindPatterns(t *testing.T) {
	tests := []struct {
		name         string
		event        *model.ChangeEvent
		ignoreConfig *config.IgnoreConfig
		want         bool
	}{
		{
			name: "exact resource kind match",
			event: &model.ChangeEvent{
				ResourceKind: "ConfigMap",
			},
			ignoreConfig: &config.IgnoreConfig{
				ResourceKindPatterns: []string{"ConfigMap"},
			},
			want: true,
		},
		{
			name: "wildcard match",
			event: &model.ChangeEvent{
				ResourceKind: "ConfigMap",
			},
			ignoreConfig: &config.IgnoreConfig{
				ResourceKindPatterns: []string{"*Map"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnore(tt.event, tt.ignoreConfig)
			if got != tt.want {
				t.Errorf("ShouldIgnore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldIgnore_MultipleCriteria(t *testing.T) {
	event := &model.ChangeEvent{
		Namespace:    "kube-system",
		Name:         "test-deployment",
		ResourceKind: "Deployment",
	}

	// Should match if ANY criteria matches
	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns:    []string{"default"}, // Doesn't match
		NamePatterns:         []string{"test-*"},  // Matches!
		ResourceKindPatterns: []string{"Service"}, // Doesn't match
	}

	if !ShouldIgnore(event, ignoreConfig) {
		t.Error("ShouldIgnore() should return true when name pattern matches")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		{"exact match", "test", "test", true},
		{"no match", "test", "prod", false},
		{"wildcard prefix", "test-app", "test-*", true},
		{"wildcard suffix", "app-test", "*-test", true},
		{"wildcard contains", "my-test-app", "*test*", true},
		{"wildcard matches empty", "test", "test*", true},
		{"wildcard at start", "test-app", "*app", true},
		{"multiple wildcards", "test-app-prod", "test-*-prod", true},
		{"empty string", "", "", true},
		{"empty string with wildcard", "", "*", true},
		{"empty string no match", "", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestHandler_HandleAdmissionReview_IgnorePattern(t *testing.T) {
	mockStore := &mockStore{}
	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*"},
	}
	handler := NewHandler(mockStore, nil, ignoreConfig, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler.Start(ctx)

	// Create a request for a resource in kube-system (should be ignored)
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Deployment",
			},
			Namespace: "kube-system",
			Name:      "test-deployment",
			UserInfo: authenticationv1.UserInfo{
				Username: "test-user",
			},
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {"name": "test-deployment", "namespace": "kube-system"}}`),
			},
		},
	}

	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Event should be ignored, so nothing should be saved
	if len(mockStore.savedEvents) != 0 {
		t.Errorf("Expected 0 saved events (should be ignored), got %d", len(mockStore.savedEvents))
	}
}

func TestShouldBlock_NamespacePatterns(t *testing.T) {
	tests := []struct {
		name        string
		event       *model.ChangeEvent
		blockConfig *config.BlockConfig
		wantBlock   bool
		wantPattern string
		wantMessage string
	}{
		{
			name: "exact namespace match",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
			},
			wantBlock:   true,
			wantPattern: "production",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "wildcard namespace match",
			event: &model.ChangeEvent{
				Namespace: "prod-app",
				Operation: "DELETE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"prod-*"},
			},
			wantBlock:   true,
			wantPattern: "prod-*",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "no match",
			event: &model.ChangeEvent{
				Namespace: "development",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
			},
			wantBlock:   false,
			wantPattern: "",
			wantMessage: "",
		},
		{
			name: "nil block config",
			event: &model.ChangeEvent{
				Namespace: "production",
			},
			blockConfig: nil,
			wantBlock:   false,
			wantPattern: "",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, pattern, message := ShouldBlock(tt.event, tt.blockConfig)
			if blocked != tt.wantBlock {
				t.Errorf("ShouldBlock() blocked = %v, want %v", blocked, tt.wantBlock)
			}
			if blocked {
				if pattern != tt.wantPattern {
					t.Errorf("ShouldBlock() pattern = %q, want %q", pattern, tt.wantPattern)
				}
				if message != tt.wantMessage {
					t.Errorf("ShouldBlock() message = %q, want %q", message, tt.wantMessage)
				}
			}
		})
	}
}

func TestShouldBlock_OperationPatterns(t *testing.T) {
	tests := []struct {
		name        string
		event       *model.ChangeEvent
		blockConfig *config.BlockConfig
		wantBlock   bool
		wantPattern string
		wantMessage string
	}{
		{
			name: "block DELETE operation",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "DELETE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
				OperationPatterns: []string{"DELETE"},
			},
			wantBlock:   true,
			wantPattern: "production",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "block multiple operations",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "UPDATE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
				OperationPatterns: []string{"DELETE", "UPDATE"},
			},
			wantBlock:   true,
			wantPattern: "production",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "operation not in block list",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
				OperationPatterns: []string{"DELETE"},
			},
			wantBlock:   false,
			wantPattern: "",
			wantMessage: "",
		},
		{
			name: "empty operation patterns blocks all",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
				OperationPatterns: []string{},
			},
			wantBlock:   true,
			wantPattern: "production",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "case insensitive operation match",
			event: &model.ChangeEvent{
				Namespace: "production",
				Operation: "delete",
			},
			blockConfig: &config.BlockConfig{
				NamespacePatterns: []string{"production"},
				OperationPatterns: []string{"DELETE"},
			},
			wantBlock:   true,
			wantPattern: "production",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, pattern, message := ShouldBlock(tt.event, tt.blockConfig)
			if blocked != tt.wantBlock {
				t.Errorf("ShouldBlock() blocked = %v, want %v", blocked, tt.wantBlock)
			}
			if blocked {
				if pattern != tt.wantPattern {
					t.Errorf("ShouldBlock() pattern = %q, want %q", pattern, tt.wantPattern)
				}
				if message != tt.wantMessage {
					t.Errorf("ShouldBlock() message = %q, want %q", message, tt.wantMessage)
				}
			}
		})
	}
}

func TestShouldBlock_NoPatterns(t *testing.T) {
	// Test with empty block config (no patterns)
	blockConfig := &config.BlockConfig{
		NamespacePatterns:    []string{},
		NamePatterns:         []string{},
		ResourceKindPatterns: []string{},
		OperationPatterns:    []string{},
	}

	event := &model.ChangeEvent{
		Namespace:    "production",
		Name:         "test",
		ResourceKind: "Deployment",
		Operation:    "CREATE",
	}

	blocked, _, _ := ShouldBlock(event, blockConfig)
	if blocked {
		t.Error("ShouldBlock() should not block when no patterns are defined")
	}
}

func TestShouldBlock_NamePatterns(t *testing.T) {
	tests := []struct {
		name        string
		event       *model.ChangeEvent
		blockConfig *config.BlockConfig
		wantBlock   bool
		wantPattern string
		wantMessage string
	}{
		{
			name: "exact name match",
			event: &model.ChangeEvent{
				Name:      "critical-app",
				Operation: "DELETE",
			},
			blockConfig: &config.BlockConfig{
				NamePatterns: []string{"critical-app"},
			},
			wantBlock:   true,
			wantPattern: "critical-app",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "wildcard name match",
			event: &model.ChangeEvent{
				Name:      "test-critical-resource",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamePatterns: []string{"*-critical-*"},
			},
			wantBlock:   true,
			wantPattern: "*-critical-*",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "custom message",
			event: &model.ChangeEvent{
				Name:      "critical",
				Operation: "UPDATE",
			},
			blockConfig: &config.BlockConfig{
				NamePatterns: []string{"critical"},
				Message:      "Critical resources cannot be modified",
			},
			wantBlock:   true,
			wantPattern: "critical",
			wantMessage: "Critical resources cannot be modified",
		},
		{
			name: "no match",
			event: &model.ChangeEvent{
				Name:      "normal-app",
				Operation: "CREATE",
			},
			blockConfig: &config.BlockConfig{
				NamePatterns: []string{"critical-*"},
			},
			wantBlock:   false,
			wantPattern: "",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, pattern, message := ShouldBlock(tt.event, tt.blockConfig)
			if blocked != tt.wantBlock {
				t.Errorf("ShouldBlock() blocked = %v, want %v", blocked, tt.wantBlock)
			}
			if blocked {
				if pattern != tt.wantPattern {
					t.Errorf("ShouldBlock() pattern = %q, want %q", pattern, tt.wantPattern)
				}
				if message != tt.wantMessage {
					t.Errorf("ShouldBlock() message = %q, want %q", message, tt.wantMessage)
				}
			}
		})
	}
}

func TestShouldBlock_ResourceKindPatterns(t *testing.T) {
	tests := []struct {
		name        string
		event       *model.ChangeEvent
		blockConfig *config.BlockConfig
		wantBlock   bool
		wantPattern string
		wantMessage string
	}{
		{
			name: "exact resource kind match",
			event: &model.ChangeEvent{
				ResourceKind: "Secret",
				Operation:    "CREATE",
			},
			blockConfig: &config.BlockConfig{
				ResourceKindPatterns: []string{"Secret"},
			},
			wantBlock:   true,
			wantPattern: "Secret",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "wildcard resource kind match",
			event: &model.ChangeEvent{
				ResourceKind: "ConfigMap",
				Operation:    "DELETE",
			},
			blockConfig: &config.BlockConfig{
				ResourceKindPatterns: []string{"Config*"},
			},
			wantBlock:   true,
			wantPattern: "Config*",
			wantMessage: "Resource blocked by kubechronicle policy",
		},
		{
			name: "no match",
			event: &model.ChangeEvent{
				ResourceKind: "Deployment",
				Operation:    "CREATE",
			},
			blockConfig: &config.BlockConfig{
				ResourceKindPatterns: []string{"Secret"},
			},
			wantBlock:   false,
			wantPattern: "",
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, pattern, message := ShouldBlock(tt.event, tt.blockConfig)
			if blocked != tt.wantBlock {
				t.Errorf("ShouldBlock() blocked = %v, want %v", blocked, tt.wantBlock)
			}
			if blocked {
				if pattern != tt.wantPattern {
					t.Errorf("ShouldBlock() pattern = %q, want %q", pattern, tt.wantPattern)
				}
				if message != tt.wantMessage {
					t.Errorf("ShouldBlock() message = %q, want %q", message, tt.wantMessage)
				}
			}
		})
	}
}

func TestMatchWildcard_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		pattern  string
		expected bool
	}{
		{
			name:     "empty string empty pattern",
			s:        "",
			pattern:  "",
			expected: true,
		},
		{
			name:     "empty string with *",
			s:        "",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "empty string with **",
			s:        "",
			pattern:  "**",
			expected: true,
		},
		{
			name:     "empty string with non-wildcard",
			s:        "",
			pattern:  "test",
			expected: false,
		},
		{
			name:     "string exhausted pattern remaining with *",
			s:        "test",
			pattern:  "test*",
			expected: true,
		},
		{
			name:     "string exhausted pattern remaining with **",
			s:        "test",
			pattern:  "test**",
			expected: true,
		},
		{
			name:     "string exhausted pattern remaining with non-*",
			s:        "test",
			pattern:  "testx",
			expected: false,
		},
		{
			name:     "multiple consecutive wildcards",
			s:        "test",
			pattern:  "***test***",
			expected: true,
		},
		{
			name:     "wildcard in middle",
			s:        "abtestcd",
			pattern:  "ab*cd",
			expected: true,
		},
		{
			name:     "wildcard at start",
			s:        "test",
			pattern:  "*est",
			expected: true,
		},
		{
			name:     "wildcard at end",
			s:        "test",
			pattern:  "tes*",
			expected: true,
		},
		{
			name:     "multiple wildcards complex",
			s:        "abc123def456",
			pattern:  "abc*def*",
			expected: true,
		},
		{
			name:     "pattern longer than string with wildcard",
			s:        "test",
			pattern:  "test*extra",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchWildcard(tt.s, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.s, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestHandler_HandleAdmissionReview_BlockPattern(t *testing.T) {
	mockStore := &mockStore{}
	blockConfig := &config.BlockConfig{
		NamespacePatterns: []string{"production"},
		Message:           "Deployment to production namespace is not allowed",
	}
	handler := NewHandler(mockStore, nil, nil, blockConfig)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler.Start(ctx)

	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Kind: metav1.GroupVersionKind{
				Kind: "Deployment",
			},
			Namespace: "production",
			Name:      "test-deployment",
			UserInfo: authenticationv1.UserInfo{
				Username: "test-user",
			},
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {"name": "test-deployment", "namespace": "production"}}`),
			},
		},
	}

	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.HandleAdmissionReview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Response.Allowed {
		t.Error("Expected request to be blocked, but it was allowed")
	}

	if response.Response.Result == nil {
		t.Error("Expected error result in response")
	} else if !strings.Contains(response.Response.Result.Message, "not allowed") {
		t.Errorf("Expected error message to contain 'not allowed', got: %s", response.Response.Result.Message)
	}

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Blocked events should be saved with allowed=false and block_pattern set
	if len(mockStore.savedEvents) != 1 {
		t.Errorf("Expected 1 saved event (blocked event should be tracked), got %d", len(mockStore.savedEvents))
	} else {
		event := mockStore.savedEvents[0]
		if event.Allowed {
			t.Error("Expected saved event to have Allowed=false")
		}
		if event.BlockPattern != "production" {
			t.Errorf("Expected BlockPattern='production', got %q", event.BlockPattern)
		}
	}
}
