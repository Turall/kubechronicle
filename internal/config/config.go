package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/alerting"
)

// Config holds application configuration.
type Config struct {
	WebhookPort  int
	TLSCertPath  string
	TLSKeyPath   string
	DatabaseURL  string
	LogLevel     string
	AlertConfig  *alerting.Config
	IgnoreConfig *IgnoreConfig
	BlockConfig  *BlockConfig
	AuthConfig   *AuthConfig
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// EnableAuth enables authentication (if false, all requests are allowed)
	EnableAuth bool `json:"enable_auth,omitempty"`
	
	// JWTSecret is the secret key for signing JWT tokens
	JWTSecret string `json:"jwt_secret,omitempty"`
	
	// JWTExpirationHours is the token expiration time in hours (default: 24)
	JWTExpirationHours int `json:"jwt_expiration_hours,omitempty"`
	
	// Users is a map of username -> user info (JSON format)
	UsersJSON string `json:"users_json,omitempty"`
}

// IgnoreConfig holds ignore pattern configuration.
type IgnoreConfig struct {
	// NamespacePatterns is a list of patterns for namespaces to ignore.
	// Supports wildcards: * matches any sequence, ? matches single character.
	// Examples: "kube-*", "*system*", "default"
	NamespacePatterns []string `json:"namespace_patterns,omitempty"`

	// NamePatterns is a list of patterns for resource names to ignore.
	// Supports wildcards: * matches any sequence, ? matches single character.
	// Examples: "*-controller", "*system*", "test-*"
	NamePatterns []string `json:"name_patterns,omitempty"`

	// ResourceKindPatterns is a list of patterns for resource kinds to ignore.
	// Supports wildcards: * matches any sequence, ? matches single character.
	// Examples: "ConfigMap", "Secret", "*-List"
	ResourceKindPatterns []string `json:"resource_kind_patterns,omitempty"`
}

// BlockConfig holds block pattern configuration.
// When a resource matches a block pattern, the webhook will deny the request.
type BlockConfig struct {
	// NamespacePatterns is a list of patterns for namespaces to block.
	// Supports wildcards: * matches any sequence.
	// Examples: "production", "*-prod", "critical-*"
	NamespacePatterns []string `json:"namespace_patterns,omitempty"`

	// NamePatterns is a list of patterns for resource names to block.
	// Supports wildcards: * matches any sequence.
	// Examples: "*-delete", "*critical*", "admin-*"
	NamePatterns []string `json:"name_patterns,omitempty"`

	// ResourceKindPatterns is a list of patterns for resource kinds to block.
	// Supports wildcards: * matches any sequence.
	// Examples: "Secret", "ConfigMap", "*Service"
	ResourceKindPatterns []string `json:"resource_kind_patterns,omitempty"`

	// OperationPatterns is a list of operations to block (CREATE, UPDATE, DELETE).
	// If empty, all operations matching other patterns will be blocked.
	// Examples: ["DELETE"], ["CREATE", "DELETE"]
	OperationPatterns []string `json:"operation_patterns,omitempty"`

	// Message is the error message returned when a request is blocked.
	// Default: "Resource blocked by kubechronicle policy"
	Message string `json:"message,omitempty"`
}

// LoadConfig loads configuration from environment variables and flags.
func LoadConfig() *Config {
	cfg := &Config{
		WebhookPort: 8443,
		TLSCertPath: getEnv("TLS_CERT_PATH", "/etc/webhook/certs/tls.crt"),
		TLSKeyPath:  getEnv("TLS_KEY_PATH", "/etc/webhook/certs/tls.key"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	// Load alerting configuration if provided
	if alertJSON := getEnv("ALERT_CONFIG", ""); alertJSON != "" {
		var alertConfig alerting.Config
		if err := json.Unmarshal([]byte(alertJSON), &alertConfig); err == nil {
			cfg.AlertConfig = &alertConfig
		}
	}

	// Load ignore configuration if provided
	if ignoreJSON := getEnv("IGNORE_CONFIG", ""); ignoreJSON != "" {
		ignoreJSON = strings.TrimSpace(ignoreJSON)
		var ignoreConfig IgnoreConfig
		if err := json.Unmarshal([]byte(ignoreJSON), &ignoreConfig); err == nil {
			cfg.IgnoreConfig = &ignoreConfig
			klog.Infof("Loaded ignore config: namespace_patterns=%v, name_patterns=%v, resource_kind_patterns=%v",
				ignoreConfig.NamespacePatterns, ignoreConfig.NamePatterns, ignoreConfig.ResourceKindPatterns)
		} else {
			klog.Warningf("Failed to parse IGNORE_CONFIG JSON: %v, raw value: %q", err, ignoreJSON)
		}
	} else {
		// Support comma-separated lists for backward compatibility
		if namespacePatterns := getEnv("IGNORE_NAMESPACES", ""); namespacePatterns != "" {
			cfg.IgnoreConfig = &IgnoreConfig{
				NamespacePatterns: parseList(namespacePatterns),
			}
		}
		if namePatterns := getEnv("IGNORE_NAMES", ""); namePatterns != "" {
			if cfg.IgnoreConfig == nil {
				cfg.IgnoreConfig = &IgnoreConfig{}
			}
			cfg.IgnoreConfig.NamePatterns = parseList(namePatterns)
		}
	}

	// Load block configuration if provided
	if blockJSON := getEnv("BLOCK_CONFIG", ""); blockJSON != "" {
		// Trim whitespace that might come from YAML multi-line strings
		blockJSON = strings.TrimSpace(blockJSON)
		var blockConfig BlockConfig
		if err := json.Unmarshal([]byte(blockJSON), &blockConfig); err == nil {
			cfg.BlockConfig = &blockConfig
			// Set default message if not provided
			if cfg.BlockConfig.Message == "" {
				cfg.BlockConfig.Message = "Resource blocked by kubechronicle policy"
			}
			klog.Infof("Loaded block config: namespace_patterns=%v, name_patterns=%v, resource_kind_patterns=%v, operation_patterns=%v",
				blockConfig.NamespacePatterns, blockConfig.NamePatterns, blockConfig.ResourceKindPatterns, blockConfig.OperationPatterns)
		} else {
			klog.Warningf("Failed to parse BLOCK_CONFIG JSON: %v, raw value: %q", err, blockJSON)
		}
	}

	// Load auth configuration if provided
	if enableAuth := getEnv("AUTH_ENABLED", ""); enableAuth == "true" || enableAuth == "1" {
		authConfig := &AuthConfig{
			EnableAuth: true,
		}
		
		// JWT Secret (required if auth is enabled)
		authConfig.JWTSecret = getEnv("JWT_SECRET", "")
		if authConfig.JWTSecret == "" {
			klog.Warning("AUTH_ENABLED is true but JWT_SECRET is not set. Authentication may not work correctly.")
		}
		
		// JWT Expiration (default: 24 hours)
		expHours := getEnv("JWT_EXPIRATION_HOURS", "24")
		if hours, err := strconv.Atoi(expHours); err == nil && hours > 0 {
			authConfig.JWTExpirationHours = hours
		} else {
			authConfig.JWTExpirationHours = 24
		}
		
		// Users configuration
		authConfig.UsersJSON = getEnv("AUTH_USERS", "")
		
		cfg.AuthConfig = authConfig
		klog.Infof("Authentication enabled: JWT expiration=%d hours", authConfig.JWTExpirationHours)
	}

	return cfg
}

// parseList parses a comma-separated list of strings.
func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
