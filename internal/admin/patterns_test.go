package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kubechronicle/kubechronicle/internal/config"
)

func TestHandleGetIgnoreConfig_Success(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	// Create a ConfigMap with ignore config
	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*", "default"},
		NamePatterns:     []string{"*-controller"},
	}
	ignoreJSON, _ := json.Marshal(ignoreConfig)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-patterns",
			Namespace: "default",
		},
		Data: map[string]string{
			"IGNORE_CONFIG": string(ignoreJSON),
		},
	}
	clientset.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})

	req := httptest.NewRequest("GET", "/kubechronicle/api/admin/patterns/ignore", nil)
	w := httptest.NewRecorder()

	handler.HandleGetIgnoreConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result config.IgnoreConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.NamespacePatterns) != 2 {
		t.Errorf("Expected 2 namespace patterns, got %d", len(result.NamespacePatterns))
	}
	if len(result.NamePatterns) != 1 {
		t.Errorf("Expected 1 name pattern, got %d", len(result.NamePatterns))
	}
}

func TestHandleGetIgnoreConfig_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	req := httptest.NewRequest("GET", "/kubechronicle/api/admin/patterns/ignore", nil)
	w := httptest.NewRecorder()

	handler.HandleGetIgnoreConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result config.IgnoreConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should return empty config
	if result.NamespacePatterns != nil && len(result.NamespacePatterns) > 0 {
		t.Error("Expected empty namespace patterns")
	}
}

func TestHandleUpdateIgnoreConfig_Success(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*"},
		NamePatterns:     []string{"*-controller"},
	}
	body, _ := json.Marshal(ignoreConfig)

	req := httptest.NewRequest("PUT", "/kubechronicle/api/admin/patterns/ignore", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleUpdateIgnoreConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ConfigMap was updated
	cm, err := clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "test-patterns", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data["IGNORE_CONFIG"] == "" {
		t.Error("IGNORE_CONFIG not set in ConfigMap")
	}

	var savedConfig config.IgnoreConfig
	if err := json.Unmarshal([]byte(cm.Data["IGNORE_CONFIG"]), &savedConfig); err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if len(savedConfig.NamespacePatterns) != 1 {
		t.Errorf("Expected 1 namespace pattern, got %d", len(savedConfig.NamespacePatterns))
	}
}

func TestHandleUpdateIgnoreConfig_InvalidBody(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	req := httptest.NewRequest("PUT", "/kubechronicle/api/admin/patterns/ignore", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleUpdateIgnoreConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUpdateIgnoreConfig_WrongMethod(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	req := httptest.NewRequest("POST", "/kubechronicle/api/admin/patterns/ignore", nil)
	w := httptest.NewRecorder()

	handler.HandleUpdateIgnoreConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetBlockConfig_Success(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	blockConfig := &config.BlockConfig{
		NamespacePatterns: []string{"production"},
		OperationPatterns: []string{"DELETE"},
		Message:           "Custom message",
	}
	blockJSON, _ := json.Marshal(blockConfig)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-patterns",
			Namespace: "default",
		},
		Data: map[string]string{
			"BLOCK_CONFIG": string(blockJSON),
		},
	}
	clientset.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})

	req := httptest.NewRequest("GET", "/kubechronicle/api/admin/patterns/block", nil)
	w := httptest.NewRecorder()

	handler.HandleGetBlockConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result config.BlockConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.NamespacePatterns) != 1 {
		t.Errorf("Expected 1 namespace pattern, got %d", len(result.NamespacePatterns))
	}
	if result.Message != "Custom message" {
		t.Errorf("Expected message 'Custom message', got %s", result.Message)
	}
}

func TestHandleUpdateBlockConfig_Success(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	blockConfig := &config.BlockConfig{
		NamespacePatterns: []string{"production"},
		OperationPatterns: []string{"DELETE"},
	}
	body, _ := json.Marshal(blockConfig)

	req := httptest.NewRequest("PUT", "/kubechronicle/api/admin/patterns/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleUpdateBlockConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify default message was set
	var response config.BlockConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Message == "" {
		t.Error("Expected default message to be set")
	}
	if response.Message != "Resource blocked by kubechronicle policy" {
		t.Errorf("Expected default message, got %s", response.Message)
	}
}

func TestHandleOptions(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	req := httptest.NewRequest("OPTIONS", "/kubechronicle/api/admin/patterns/ignore", nil)
	w := httptest.NewRecorder()

	handler.HandleGetIgnoreConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header")
	}
}

func TestGetConfigMap_CreatesIfNotExists(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	cm, err := handler.getConfigMap(context.Background())
	if err != nil {
		t.Fatalf("Failed to get ConfigMap: %v", err)
	}

	if cm.Name != "test-patterns" {
		t.Errorf("Expected ConfigMap name test-patterns, got %s", cm.Name)
	}
	if cm.Namespace != "default" {
		t.Errorf("Expected namespace default, got %s", cm.Namespace)
	}
}

func TestUpdateConfigMap_UpdatesBothConfigs(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	handler := NewPatternsHandler(clientset, "default", "test-patterns")

	ignoreConfig := &config.IgnoreConfig{
		NamespacePatterns: []string{"kube-*"},
	}
	blockConfig := &config.BlockConfig{
		NamespacePatterns: []string{"production"},
	}

	err := handler.updateConfigMap(context.Background(), "IGNORE_CONFIG", ignoreConfig, nil)
	if err != nil {
		t.Fatalf("Failed to update ignore config: %v", err)
	}

	err = handler.updateConfigMap(context.Background(), "BLOCK_CONFIG", nil, blockConfig)
	if err != nil {
		t.Fatalf("Failed to update block config: %v", err)
	}

	cm, err := clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "test-patterns", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data["IGNORE_CONFIG"] == "" {
		t.Error("IGNORE_CONFIG not set")
	}
	if cm.Data["BLOCK_CONFIG"] == "" {
		t.Error("BLOCK_CONFIG not set")
	}
}
