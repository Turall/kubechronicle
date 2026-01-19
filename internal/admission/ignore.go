package admission

import (
	"strings"

	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/model"
)

// ShouldIgnore checks if a change event should be ignored based on ignore patterns.
func ShouldIgnore(event *model.ChangeEvent, ignoreConfig *config.IgnoreConfig) bool {
	if ignoreConfig == nil {
		return false
	}

	// Check namespace patterns
	if matchesAnyPattern(event.Namespace, ignoreConfig.NamespacePatterns) {
		return true
	}

	// Check name patterns
	if matchesAnyPattern(event.Name, ignoreConfig.NamePatterns) {
		return true
	}

	// Check resource kind patterns
	if matchesAnyPattern(event.ResourceKind, ignoreConfig.ResourceKindPatterns) {
		return true
	}

	return false
}

// matchesAnyPattern checks if a string matches any of the given patterns.
// Supports wildcards: * matches any sequence of characters, ? matches a single character.
func matchesAnyPattern(s string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	for _, pattern := range patterns {
		if matchPattern(s, pattern) {
			return true
		}
	}

	return false
}

// matchPattern checks if a string matches a pattern with wildcard support.
// Supports:
//   - * : matches any sequence of characters (including empty)
//   - Exact match if no wildcards
func matchPattern(s, pattern string) bool {
	// Exact match (no wildcards)
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Convert to regex-like matching using recursive backtracking
	return matchWildcard(s, pattern)
}

// matchWildcard matches a string against a pattern with * wildcards.
// Uses a simple recursive algorithm.
func matchWildcard(s, pattern string) bool {
	// Base cases
	if pattern == "" {
		return s == ""
	}
	if pattern == "*" {
		return true // * matches everything
	}

	sLen := len(s)
	pLen := len(pattern)
	sIdx := 0
	pIdx := 0

	for pIdx < pLen {
		if sIdx > sLen {
			// String exhausted but pattern remains
			// Only match if remaining pattern is all *s
			for pIdx < pLen && pattern[pIdx] == '*' {
				pIdx++
			}
			return pIdx == pLen
		}

		if pattern[pIdx] == '*' {
			// Skip consecutive *s
			for pIdx < pLen && pattern[pIdx] == '*' {
				pIdx++
			}
			// If * is at the end, it matches the rest
			if pIdx == pLen {
				return true
			}
			// Try matching remaining pattern at each position in remaining string
			for i := sIdx; i <= sLen; i++ {
				if matchWildcard(s[i:], pattern[pIdx:]) {
					return true
				}
			}
			return false
		}

		// Regular character must match
		if sIdx < sLen && s[sIdx] == pattern[pIdx] {
			sIdx++
			pIdx++
		} else {
			return false
		}
	}

	// Pattern exhausted - match only if string is also exhausted
	return sIdx == sLen
}

// ShouldBlock checks if a change event should be blocked based on block patterns.
// Returns true if the event matches any block pattern and should be denied,
// along with the matching pattern and error message.
func ShouldBlock(event *model.ChangeEvent, blockConfig *config.BlockConfig) (bool, string, string) {
	if blockConfig == nil {
		return false, "", ""
	}

	// Check if operation is blocked
	// If operation_patterns is empty, all operations are considered
	// If operation_patterns has values, only those operations are blocked
	if len(blockConfig.OperationPatterns) > 0 {
		operationMatched := false
		for _, op := range blockConfig.OperationPatterns {
			if strings.EqualFold(event.Operation, op) {
				operationMatched = true
				break
			}
		}
		if !operationMatched {
			// Operation not in block list, don't block
			return false, "", ""
		}
	}

	// Check namespace patterns
	for _, pattern := range blockConfig.NamespacePatterns {
		if matchPattern(event.Namespace, pattern) {
			message := blockConfig.Message
			if message == "" {
				message = "Resource blocked by kubechronicle policy"
			}
			return true, pattern, message
		}
	}

	// Check name patterns
	for _, pattern := range blockConfig.NamePatterns {
		if matchPattern(event.Name, pattern) {
			message := blockConfig.Message
			if message == "" {
				message = "Resource blocked by kubechronicle policy"
			}
			return true, pattern, message
		}
	}

	// Check resource kind patterns
	for _, pattern := range blockConfig.ResourceKindPatterns {
		if matchPattern(event.ResourceKind, pattern) {
			message := blockConfig.Message
			if message == "" {
				message = "Resource blocked by kubechronicle policy"
			}
			return true, pattern, message
		}
	}

	return false, "", ""
}
