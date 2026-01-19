package diff

import "strings"

// ignoredPaths contains field paths that should be excluded from diffs.
// These are Kubernetes noise fields that change frequently but don't represent
// meaningful business logic changes.
var ignoredPaths = map[string]bool{
	"/metadata/managedFields":     true,
	"/metadata/resourceVersion":   true,
	"/metadata/generation":        true,
	"/metadata/creationTimestamp": true,
	"/status":                     true,
	"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration": true,
}

// ShouldIgnoreField determines if a field path should be excluded from diffs.
// Paths are in JSON Pointer format (RFC 6901).
func ShouldIgnoreField(path string) bool {
	// Normalize path (remove leading slash if present for comparison)
	normalized := path
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	// Direct match
	if ignoredPaths[normalized] {
		return true
	}

	// Check if path is under /status (all status fields should be ignored)
	if strings.HasPrefix(normalized, "/status/") {
		return true
	}

	// Check if path is under /metadata/managedFields
	if strings.HasPrefix(normalized, "/metadata/managedFields/") {
		return true
	}

	return false
}

// IsIgnoredPath checks if a path should be ignored (alias for ShouldIgnoreField).
func IsIgnoredPath(path string) bool {
	return ShouldIgnoreField(path)
}
