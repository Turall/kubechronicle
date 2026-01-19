package diff

import "testing"

func TestComputeDiff_EmptyObjects(t *testing.T) {
	oldObj := map[string]interface{}{}
	newObj := map[string]interface{}{}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}
	if len(patches) != 0 {
		t.Errorf("ComputeDiff() returned %d patches, want 0", len(patches))
	}
}

func TestComputeDiff_SimpleAdd(t *testing.T) {
	oldObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
			"newField": "value",
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}
	if len(patches) == 0 {
		t.Error("ComputeDiff() returned no patches, expected at least one")
	}

	// Check that we have an add operation
	foundAdd := false
	for _, patch := range patches {
		if patch.Op == "add" && patch.Path == "/spec/newField" {
			foundAdd = true
			if patch.Value != "value" {
				t.Errorf("patch.Value = %v, want 'value'", patch.Value)
			}
		}
	}
	if !foundAdd {
		t.Error("Expected add operation for /spec/newField")
	}
}

func TestComputeDiff_SimpleRemove(t *testing.T) {
	oldObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
			"oldField": "value",
		},
	}
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	foundRemove := false
	for _, patch := range patches {
		if patch.Op == "remove" && patch.Path == "/spec/oldField" {
			foundRemove = true
		}
	}
	if !foundRemove {
		t.Errorf("Expected remove operation for /spec/oldField, got patches: %+v", patches)
	}
}

func TestComputeDiff_SimpleReplace(t *testing.T) {
	oldObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 3,
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	foundReplace := false
	for _, patch := range patches {
		if patch.Op == "replace" && patch.Path == "/spec/replicas" {
			foundReplace = true
			if patch.Value != 3 {
				t.Errorf("patch.Value = %v, want 3", patch.Value)
			}
		}
	}
	if !foundReplace {
		t.Errorf("Expected replace operation for /spec/replicas, got patches: %+v", patches)
	}
}

func TestComputeDiff_IgnoresMetadataFields(t *testing.T) {
	oldObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":              "test",
			"resourceVersion":   "1",
			"generation":        1,
			"managedFields":     []interface{}{},
			"creationTimestamp": "2023-01-01T00:00:00Z",
		},
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}
	newObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":              "test",
			"resourceVersion":   "2",             // Changed but should be ignored
			"generation":        2,               // Changed but should be ignored
			"managedFields":     []interface{}{}, // Changed but should be ignored
			"creationTimestamp": "2023-01-01T00:00:00Z",
		},
		"spec": map[string]interface{}{
			"replicas": 2, // This should be in the diff
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	// Should only have one patch for replicas change
	hasReplicasPatch := false
	hasIgnoredPatch := false
	for _, patch := range patches {
		if patch.Path == "/spec/replicas" {
			hasReplicasPatch = true
		}
		if patch.Path == "/metadata/resourceVersion" || patch.Path == "/metadata/generation" {
			hasIgnoredPatch = true
		}
	}

	if !hasReplicasPatch {
		t.Errorf("Expected patch for /spec/replicas, got patches: %+v", patches)
	}
	if hasIgnoredPatch {
		t.Errorf("Should not have patches for ignored fields, got: %+v", patches)
	}
}

func TestComputeDiff_IgnoresStatus(t *testing.T) {
	oldObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{},
		},
	}
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available"},
			},
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	// Should have no patches since only status changed
	if len(patches) != 0 {
		t.Errorf("ComputeDiff() returned %d patches, want 0 (status changes should be ignored)", len(patches))
	}
}

func TestComputeDiff_SecretHashing(t *testing.T) {
	oldObj := map[string]interface{}{
		"data": map[string]interface{}{
			"password": "secret123",
			"username": "admin",
		},
	}
	newObj := map[string]interface{}{
		"data": map[string]interface{}{
			"password": "secret456", // Changed
			"username": "admin",     // Unchanged
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Secret")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	// Should have a patch for password change (but with hashed values)
	foundPasswordPatch := false
	for _, patch := range patches {
		if patch.Path == "/data/password" {
			foundPasswordPatch = true
			// Value should be a hash, not the plaintext
			hashValue, ok := patch.Value.(string)
			if !ok {
				t.Error("Password patch value should be a string (hash)")
			}
			if hashValue == "secret456" {
				t.Error("Password should be hashed, not stored as plaintext")
			}
			if len(hashValue) < 10 { // SHA-256 hex is 64 chars + "sha256:" prefix
				t.Errorf("Hash value too short: %s", hashValue)
			}
			// Verify it's a proper hash format
			if hashValue[:7] != "sha256:" {
				t.Errorf("Hash should start with 'sha256:', got %s", hashValue[:7])
			}
		}
		// Note: username won't have a patch if it's unchanged (same hash in old and new)
	}

	if !foundPasswordPatch {
		t.Errorf("Expected patch for /data/password, got patches: %+v", patches)
	}
}

func TestHashSecretValue(t *testing.T) {
	value := "test-secret"
	hash := HashSecretValue(value)

	if hash == value {
		t.Error("HashSecretValue() should not return the original value")
	}
	if len(hash) < 10 {
		t.Errorf("Hash should be longer, got %d chars", len(hash))
	}
	if hash[:7] != "sha256:" {
		t.Errorf("Hash should start with 'sha256:', got %s", hash[:7])
	}

	// Same value should produce same hash
	hash2 := HashSecretValue(value)
	if hash != hash2 {
		t.Error("Same value should produce same hash")
	}

	// Different value should produce different hash
	hash3 := HashSecretValue("different-secret")
	if hash == hash3 {
		t.Error("Different values should produce different hashes")
	}
}

func TestHashBase64SecretValue(t *testing.T) {
	// Test with valid base64
	base64Value := "dGVzdC1zZWNyZXQ=" // "test-secret" in base64
	hash := HashBase64SecretValue(base64Value)

	if len(hash) < 10 {
		t.Errorf("Hash should be longer, got %d chars", len(hash))
	}
	if hash[:7] != "sha256:" {
		t.Errorf("Hash should start with 'sha256:', got %s", hash[:7])
	}

	// Test with invalid base64 (should hash the string as-is)
	invalidBase64 := "not-valid-base64!!!"
	hash2 := HashBase64SecretValue(invalidBase64)
	if len(hash2) < 10 {
		t.Error("Should still produce hash for invalid base64")
	}
}

func TestHashValue_ByteArray(t *testing.T) {
	// Test hashValue with []byte
	byteValue := []byte("test-secret")
	hash := hashValue(byteValue)

	if len(hash) < 10 {
		t.Errorf("Hash should be longer, got %d chars", len(hash))
	}
	if hash[:7] != "sha256:" {
		t.Errorf("Hash should start with 'sha256:', got %s", hash[:7])
	}
}

func TestHashValue_OtherTypes(t *testing.T) {
	// Test hashValue with other types (should JSON encode)
	intValue := 123
	hash := hashValue(intValue)

	if len(hash) < 10 {
		t.Errorf("Hash should be longer, got %d chars", len(hash))
	}
	if hash[:7] != "sha256:" {
		t.Errorf("Hash should start with 'sha256:', got %s", hash[:7])
	}

	// Test with map
	mapValue := map[string]interface{}{"key": "value"}
	hash2 := hashValue(mapValue)
	if len(hash2) < 10 {
		t.Error("Should produce hash for map type")
	}
}

func TestHashSecretValues_StringData(t *testing.T) {
	obj := map[string]interface{}{
		"stringData": map[string]interface{}{
			"password": "secret123",
		},
	}

	result := HashSecretValues(obj)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	stringData, ok := resultMap["stringData"].(map[string]interface{})
	if !ok {
		t.Fatal("stringData should be a map")
	}

	hashedPassword, ok := stringData["password"].(string)
	if !ok {
		t.Fatal("Password should be a string")
	}
	if hashedPassword == "secret123" {
		t.Error("Password should be hashed, not plaintext")
	}
	if len(hashedPassword) < 10 {
		t.Error("Hash should be longer")
	}
}

func TestHashSecretValues_NonMapValue(t *testing.T) {
	// Test with non-map value (should return as-is)
	obj := "not-a-map"
	result := HashSecretValues(obj)
	if result != obj {
		t.Error("Non-map values should be returned as-is")
	}
}

func TestFilterIgnoredFields_ArrayWithIgnoredItems(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"managedFields": []interface{}{
				map[string]interface{}{"manager": "kubectl"},
			},
		},
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}

	result := FilterIgnoredFields(obj, "")
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	// managedFields should be filtered out
	if _, exists := resultMap["metadata"].(map[string]interface{})["managedFields"]; exists {
		t.Error("managedFields should be filtered out")
	}

	// spec should remain
	if _, exists := resultMap["spec"]; !exists {
		t.Error("spec should not be filtered")
	}
}

func TestFilterIgnoredFields_DefaultCase(t *testing.T) {
	// Test default case (primitive values)
	obj := "string-value"
	result := FilterIgnoredFields(obj, "")
	if result != obj {
		t.Error("Primitive values should be returned as-is")
	}

	obj2 := 123
	result2 := FilterIgnoredFields(obj2, "")
	if result2 != obj2 {
		t.Error("Primitive values should be returned as-is")
	}
}

func TestHashSecretValues_DataNotMap(t *testing.T) {
	// Test when data/stringData value is not a map
	obj := map[string]interface{}{
		"data": "not-a-map",
	}

	result := HashSecretValues(obj)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	// Should return the value as-is if it's not a map
	if resultMap["data"] != "not-a-map" {
		t.Error("Non-map data value should be returned as-is")
	}
}

func TestHashSecretValues_Recursive(t *testing.T) {
	// Test recursive hashing of nested structures
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"data": map[string]interface{}{
				"nested": "secret",
			},
		},
	}

	result := HashSecretValues(obj)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	metadata, ok := resultMap["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata should be a map")
	}

	nestedData, ok := metadata["data"].(map[string]interface{})
	if !ok {
		t.Fatal("nested data should be a map")
	}

	hashedNested, ok := nestedData["nested"].(string)
	if !ok {
		t.Fatal("nested value should be a string")
	}
	if hashedNested == "secret" {
		t.Error("Nested secret should be hashed")
	}
}

func TestComputeDiff_TypeMismatch(t *testing.T) {
	oldObj := map[string]interface{}{
		"field": "string",
	}
	newObj := map[string]interface{}{
		"field": 123, // Type changed from string to int
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	// Should have a replace operation
	foundReplace := false
	for _, patch := range patches {
		if patch.Op == "replace" && patch.Path == "/field" {
			foundReplace = true
		}
	}
	if !foundReplace {
		t.Error("Expected replace operation for type mismatch")
	}
}

func TestComputeDiff_AddRootLevel(t *testing.T) {
	// Test adding a root-level field
	oldObj := map[string]interface{}{}
	newObj := map[string]interface{}{
		"newField": "value",
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	foundAdd := false
	for _, patch := range patches {
		if patch.Op == "add" && patch.Path == "/newField" {
			foundAdd = true
		}
	}
	if !foundAdd {
		t.Errorf("Expected add operation for /newField, got patches: %+v", patches)
	}
}

func TestEscapeJSONPointer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape tilde",
			input:    "test~value",
			expected: "test~0value",
		},
		{
			name:     "escape slash",
			input:    "test/value",
			expected: "test~1value",
		},
		{
			name:     "escape both",
			input:    "test~1/value",
			expected: "test~01~1value",
		},
		{
			name:     "no escaping needed",
			input:    "testvalue",
			expected: "testvalue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJSONPointer(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestComputeDiff_ArrayReplace(t *testing.T) {
	oldObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "app", "image": "nginx:1.0"},
			},
		},
	}
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "app", "image": "nginx:2.0"},
			},
		},
	}

	patches, err := ComputeDiff(oldObj, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}

	// Should have a replace operation for the array
	foundReplace := false
	for _, patch := range patches {
		if patch.Op == "replace" {
			foundReplace = true
		}
	}
	if !foundReplace {
		t.Error("Expected replace operation for array change")
	}
}

func TestComputeDiff_NilObjects(t *testing.T) {
	// Test with nil old object (CREATE scenario)
	newObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}

	patches, err := ComputeDiff(nil, newObj, "Deployment")
	if err != nil {
		t.Fatalf("ComputeDiff() error = %v", err)
	}
	// Should handle nil gracefully
	_ = patches
}
