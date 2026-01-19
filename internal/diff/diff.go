package diff

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// ComputeDiff generates an RFC 6902 JSON Patch between old and new objects.
// It applies ignore rules and handles Secret hashing.
func ComputeDiff(oldObj, newObj map[string]interface{}, resourceKind string) ([]model.PatchOp, error) {
	// Filter ignored fields from both objects before diffing
	oldFiltered := FilterIgnoredFields(oldObj, "")
	newFiltered := FilterIgnoredFields(newObj, "")

	// Hash Secret values if this is a Secret resource
	if resourceKind == "Secret" {
		oldFiltered = HashSecretValues(oldFiltered)
		newFiltered = HashSecretValues(newFiltered)
	}

	// Compute the diff (empty path means root)
	patches := computePatchOperations(oldFiltered, newFiltered, "")

	return patches, nil
}

// FilterIgnoredFields recursively removes ignored fields from an object.
// It's exported for use by other packages that need to filter Kubernetes noise fields.
func FilterIgnoredFields(obj interface{}, pathPrefix string) interface{} {
	if obj == nil {
		return nil
	}

	switch v := obj.(type) {
	case map[string]interface{}:
		filtered := make(map[string]interface{})
		for key, value := range v {
			path := pathPrefix + "/" + escapeJSONPointer(key)
			if !ShouldIgnoreField(path) {
				filtered[key] = FilterIgnoredFields(value, path)
			}
		}
		return filtered
	case []interface{}:
		filtered := make([]interface{}, 0, len(v))
		for i, item := range v {
			path := pathPrefix + "/" + fmt.Sprintf("%d", i)
			if !ShouldIgnoreField(path) {
				filtered = append(filtered, FilterIgnoredFields(item, path))
			}
		}
		return filtered
	default:
		return v
	}
}

// HashSecretValues hashes all values in a Secret's data and stringData fields.
// It's exported for use by other packages that need to hash Secret values.
func HashSecretValues(obj interface{}) interface{} {
	if obj == nil {
		return nil
	}

	secretMap, ok := obj.(map[string]interface{})
	if !ok {
		return obj
	}

	result := make(map[string]interface{})
	for key, value := range secretMap {
		if key == "data" || key == "stringData" {
			// Hash the values in data/stringData
			if dataMap, ok := value.(map[string]interface{}); ok {
				hashedData := make(map[string]interface{})
				for k, v := range dataMap {
					hashedData[k] = hashValue(v)
				}
				result[key] = hashedData
			} else {
				result[key] = value
			}
		} else {
			// Recursively process other fields
			result[key] = HashSecretValues(value)
		}
	}

	return result
}

// hashValue computes SHA-256 hash of a value and returns it as a hex string.
func hashValue(value interface{}) string {
	var data []byte

	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		// For other types, JSON encode them
		jsonData, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("<hash-error:%v>", err)
		}
		data = jsonData
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}

// computePatchOperations generates RFC 6902 patch operations between two objects.
func computePatchOperations(oldObj, newObj interface{}, path string) []model.PatchOp {
	var patches []model.PatchOp

	// Handle nil cases
	if oldObj == nil && newObj == nil {
		return patches
	}

	if oldObj == nil {
		// Entire object was added
		return []model.PatchOp{{Op: "add", Path: path, Value: newObj}}
	}

	if newObj == nil {
		// Entire object was removed
		return []model.PatchOp{{Op: "remove", Path: path}}
	}

	oldType := reflect.TypeOf(oldObj)
	newType := reflect.TypeOf(newObj)

	// Type mismatch - replace entire value
	if oldType != newType {
		return []model.PatchOp{{Op: "replace", Path: path, Value: newObj}}
	}

	// Handle maps
	if oldMap, ok := oldObj.(map[string]interface{}); ok {
		newMap := newObj.(map[string]interface{})

		// Build path prefix for nested keys
		pathPrefix := path
		if pathPrefix == "" {
			pathPrefix = "/"
		}

		// Find removed and modified keys
		for key, oldValue := range oldMap {
			// Add "/" separator if pathPrefix is not just "/"
			var keyPath string
			if pathPrefix == "/" {
				keyPath = pathPrefix + escapeJSONPointer(key)
			} else {
				keyPath = pathPrefix + "/" + escapeJSONPointer(key)
			}
			newValue, exists := newMap[key]

			if !exists {
				// Key was removed
				patches = append(patches, model.PatchOp{Op: "remove", Path: keyPath})
			} else if !reflect.DeepEqual(oldValue, newValue) {
				// Key was modified - recurse
				patches = append(patches, computePatchOperations(oldValue, newValue, keyPath)...)
			}
		}

		// Find added keys
		for key, newValue := range newMap {
			if _, exists := oldMap[key]; !exists {
				// Add "/" separator if pathPrefix is not just "/"
				var keyPath string
				if pathPrefix == "/" {
					keyPath = pathPrefix + escapeJSONPointer(key)
				} else {
					keyPath = pathPrefix + "/" + escapeJSONPointer(key)
				}
				patches = append(patches, model.PatchOp{Op: "add", Path: keyPath, Value: newValue})
			}
		}

		return patches
	}

	// Handle arrays
	if oldArray, ok := oldObj.([]interface{}); ok {
		newArray := newObj.([]interface{})

		// For arrays, we use a simple strategy: if lengths differ or any element differs, replace the entire array
		// A more sophisticated approach could use longest common subsequence, but for Kubernetes resources
		// this is usually sufficient
		if !reflect.DeepEqual(oldArray, newArray) {
			patches = append(patches, model.PatchOp{Op: "replace", Path: path, Value: newArray})
		}

		return patches
	}

	// Primitive values - if different, replace
	if !reflect.DeepEqual(oldObj, newObj) {
		patches = append(patches, model.PatchOp{Op: "replace", Path: path, Value: newObj})
	}

	return patches
}

// escapeJSONPointer escapes a JSON Pointer key according to RFC 6901.
// ~0 -> ~
// ~1 -> /
// / -> ~1
func escapeJSONPointer(key string) string {
	key = strings.ReplaceAll(key, "~", "~0")
	key = strings.ReplaceAll(key, "/", "~1")
	return key
}

// HashSecretValue computes SHA-256 hash of a Secret value.
// This is a utility function that can be used elsewhere.
func HashSecretValue(value string) string {
	hash := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", hash)
}

// HashBase64SecretValue computes SHA-256 hash of a base64-encoded Secret value.
func HashBase64SecretValue(base64Value string) string {
	decoded, err := base64.StdEncoding.DecodeString(base64Value)
	if err != nil {
		// If decoding fails, hash the original string
		return HashSecretValue(base64Value)
	}
	hash := sha256.Sum256(decoded)
	return fmt.Sprintf("sha256:%x", hash)
}
