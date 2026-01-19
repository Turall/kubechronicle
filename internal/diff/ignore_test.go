package diff

import "testing"

func TestShouldIgnoreField(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "ignore metadata.managedFields",
			path:     "/metadata/managedFields",
			expected: true,
		},
		{
			name:     "ignore metadata.resourceVersion",
			path:     "/metadata/resourceVersion",
			expected: true,
		},
		{
			name:     "ignore metadata.generation",
			path:     "/metadata/generation",
			expected: true,
		},
		{
			name:     "ignore metadata.creationTimestamp",
			path:     "/metadata/creationTimestamp",
			expected: true,
		},
		{
			name:     "ignore status root",
			path:     "/status",
			expected: true,
		},
		{
			name:     "ignore status subfield",
			path:     "/status/conditions",
			expected: true,
		},
		{
			name:     "ignore nested status",
			path:     "/status/conditions/0/type",
			expected: true,
		},
		{
			name:     "ignore kubectl annotation",
			path:     "/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration",
			expected: true,
		},
		{
			name:     "ignore managedFields subfield",
			path:     "/metadata/managedFields/0",
			expected: true,
		},
		{
			name:     "ignore nested managedFields",
			path:     "/metadata/managedFields/0/manager",
			expected: true,
		},
		{
			name:     "do not ignore metadata.name",
			path:     "/metadata/name",
			expected: false,
		},
		{
			name:     "do not ignore spec",
			path:     "/spec",
			expected: false,
		},
		{
			name:     "do not ignore spec.replicas",
			path:     "/spec/replicas",
			expected: false,
		},
		{
			name:     "path without leading slash",
			path:     "metadata/name",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldIgnoreField(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldIgnoreField(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsIgnoredPath(t *testing.T) {
	// Test that IsIgnoredPath is an alias for ShouldIgnoreField
	if IsIgnoredPath("/status") != ShouldIgnoreField("/status") {
		t.Error("IsIgnoredPath should return same result as ShouldIgnoreField")
	}
	if IsIgnoredPath("/metadata/name") != ShouldIgnoreField("/metadata/name") {
		t.Error("IsIgnoredPath should return same result as ShouldIgnoreField")
	}
}
