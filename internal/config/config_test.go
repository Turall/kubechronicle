package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables
	os.Clearenv()

	cfg := LoadConfig()

	if cfg.WebhookPort != 8443 {
		t.Errorf("WebhookPort = %d, want 8443", cfg.WebhookPort)
	}
	if cfg.TLSCertPath != "/etc/webhook/certs/tls.crt" {
		t.Errorf("TLSCertPath = %s, want /etc/webhook/certs/tls.crt", cfg.TLSCertPath)
	}
	if cfg.TLSKeyPath != "/etc/webhook/certs/tls.key" {
		t.Errorf("TLSKeyPath = %s, want /etc/webhook/certs/tls.key", cfg.TLSKeyPath)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("DatabaseURL = %s, want empty", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %s, want info", cfg.LogLevel)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("TLS_CERT_PATH", "/custom/cert.pem")
	os.Setenv("TLS_KEY_PATH", "/custom/key.pem")
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("TLS_CERT_PATH")
		os.Unsetenv("TLS_KEY_PATH")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg := LoadConfig()

	if cfg.TLSCertPath != "/custom/cert.pem" {
		t.Errorf("TLSCertPath = %s, want /custom/cert.pem", cfg.TLSCertPath)
	}
	if cfg.TLSKeyPath != "/custom/key.pem" {
		t.Errorf("TLSKeyPath = %s, want /custom/key.pem", cfg.TLSKeyPath)
	}
	if cfg.DatabaseURL != "postgres://localhost/db" {
		t.Errorf("DatabaseURL = %s, want postgres://localhost/db", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
	}
}

func TestGetEnv(t *testing.T) {
	// Test with environment variable set
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("getEnv() = %s, want test-value", result)
	}

	// Test with environment variable not set
	result = getEnv("NONEXISTENT_VAR", "default-value")
	if result != "default-value" {
		t.Errorf("getEnv() = %s, want default-value", result)
	}

	// Test with empty environment variable (should use default)
	os.Setenv("EMPTY_VAR", "")
	result = getEnv("EMPTY_VAR", "default")
	if result != "default" {
		t.Errorf("getEnv() with empty env var = %s, want default", result)
	}
}

func TestLoadConfig_IgnoreConfig_JSON(t *testing.T) {
	os.Clearenv()
	ignoreJSON := `{
		"namespace_patterns": ["kube-*", "default"],
		"name_patterns": ["*-controller"],
		"resource_kind_patterns": ["ConfigMap"]
	}`
	os.Setenv("IGNORE_CONFIG", ignoreJSON)
	defer os.Unsetenv("IGNORE_CONFIG")

	cfg := LoadConfig()

	if cfg.IgnoreConfig == nil {
		t.Error("IgnoreConfig should not be nil")
	}
	if len(cfg.IgnoreConfig.NamespacePatterns) != 2 {
		t.Errorf("NamespacePatterns length = %d, want 2", len(cfg.IgnoreConfig.NamespacePatterns))
	}
	if len(cfg.IgnoreConfig.NamePatterns) != 1 {
		t.Errorf("NamePatterns length = %d, want 1", len(cfg.IgnoreConfig.NamePatterns))
	}
	if len(cfg.IgnoreConfig.ResourceKindPatterns) != 1 {
		t.Errorf("ResourceKindPatterns length = %d, want 1", len(cfg.IgnoreConfig.ResourceKindPatterns))
	}
}

func TestLoadConfig_IgnoreConfig_CommaSeparated(t *testing.T) {
	os.Clearenv()
	os.Setenv("IGNORE_NAMESPACES", "kube-*,default,test-*")
	os.Setenv("IGNORE_NAMES", "*-controller,*-system")
	defer func() {
		os.Unsetenv("IGNORE_NAMESPACES")
		os.Unsetenv("IGNORE_NAMES")
	}()

	cfg := LoadConfig()

	if cfg.IgnoreConfig == nil {
		t.Error("IgnoreConfig should not be nil")
	}
	if len(cfg.IgnoreConfig.NamespacePatterns) != 3 {
		t.Errorf("NamespacePatterns length = %d, want 3", len(cfg.IgnoreConfig.NamespacePatterns))
	}
	if len(cfg.IgnoreConfig.NamePatterns) != 2 {
		t.Errorf("NamePatterns length = %d, want 2", len(cfg.IgnoreConfig.NamePatterns))
	}
}

func TestLoadConfig_IgnoreConfig_InvalidJSON(t *testing.T) {
	os.Clearenv()
	os.Setenv("IGNORE_CONFIG", "invalid json")
	defer os.Unsetenv("IGNORE_CONFIG")

	cfg := LoadConfig()

	// Should not crash, and IgnoreConfig should be nil if JSON is invalid
	if cfg.IgnoreConfig != nil {
		t.Error("IgnoreConfig should be nil when JSON is invalid")
	}
}

func TestLoadConfig_BlockConfig_JSON(t *testing.T) {
	os.Clearenv()
	blockJSON := `{
		"namespace_patterns": ["production"],
		"operation_patterns": ["DELETE"],
		"message": "Custom block message"
	}`
	os.Setenv("BLOCK_CONFIG", blockJSON)
	defer os.Unsetenv("BLOCK_CONFIG")

	cfg := LoadConfig()

	if cfg.BlockConfig == nil {
		t.Error("BlockConfig should not be nil")
	}
	if len(cfg.BlockConfig.NamespacePatterns) != 1 {
		t.Errorf("NamespacePatterns length = %d, want 1", len(cfg.BlockConfig.NamespacePatterns))
	}
	if len(cfg.BlockConfig.OperationPatterns) != 1 {
		t.Errorf("OperationPatterns length = %d, want 1", len(cfg.BlockConfig.OperationPatterns))
	}
	if cfg.BlockConfig.Message != "Custom block message" {
		t.Errorf("Message = %s, want 'Custom block message'", cfg.BlockConfig.Message)
	}
}

func TestLoadConfig_BlockConfig_DefaultMessage(t *testing.T) {
	os.Clearenv()
	blockJSON := `{
		"namespace_patterns": ["production"]
	}`
	os.Setenv("BLOCK_CONFIG", blockJSON)
	defer os.Unsetenv("BLOCK_CONFIG")

	cfg := LoadConfig()

	if cfg.BlockConfig == nil {
		t.Error("BlockConfig should not be nil")
	}
	if cfg.BlockConfig.Message != "Resource blocked by kubechronicle policy" {
		t.Errorf("Message = %s, want default message", cfg.BlockConfig.Message)
	}
}

func TestLoadConfig_BlockConfig_InvalidJSON(t *testing.T) {
	os.Clearenv()
	os.Setenv("BLOCK_CONFIG", "invalid json")
	defer os.Unsetenv("BLOCK_CONFIG")

	cfg := LoadConfig()

	// Should not crash, and BlockConfig should be nil if JSON is invalid
	if cfg.BlockConfig != nil {
		t.Error("BlockConfig should be nil when JSON is invalid")
	}
}

func TestLoadConfig_AlertConfig(t *testing.T) {
	os.Clearenv()
	alertJSON := `{
		"operations": ["CREATE", "DELETE"]
	}`
	os.Setenv("ALERT_CONFIG", alertJSON)
	defer os.Unsetenv("ALERT_CONFIG")

	cfg := LoadConfig()

	if cfg.AlertConfig == nil {
		t.Error("AlertConfig should not be nil")
	}
	if len(cfg.AlertConfig.Operations) != 2 {
		t.Errorf("Operations length = %d, want 2", len(cfg.AlertConfig.Operations))
	}
}

func TestLoadConfig_AlertConfig_InvalidJSON(t *testing.T) {
	os.Clearenv()
	os.Setenv("ALERT_CONFIG", "invalid json")
	defer os.Unsetenv("ALERT_CONFIG")

	cfg := LoadConfig()

	// Should not crash, and AlertConfig should be nil if JSON is invalid
	if cfg.AlertConfig != nil {
		t.Error("AlertConfig should be nil when JSON is invalid")
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single item",
			input:    "test",
			expected: []string{"test"},
		},
		{
			name:     "multiple items",
			input:    "a,b,c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with spaces",
			input:    "a, b, c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with extra spaces",
			input:    "  a  ,  b  ,  c  ",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty items",
			input:    "a,,b",
			expected: []string{"a", "b"},
		},
		{
			name:     "trailing comma",
			input:    "a,b,",
			expected: []string{"a", "b"},
		},
		{
			name:     "leading comma",
			input:    ",a,b",
			expected: []string{"a", "b"},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseList() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("parseList()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
