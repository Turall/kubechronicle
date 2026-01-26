package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/config"
)

func TestAuthConfigFromConfig_AuthDisabled(t *testing.T) {
	cfg := &config.AuthConfig{
		EnableAuth: false,
	}

	authConfig, err := AuthConfigFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if authConfig.EnableAuth {
		t.Error("Expected auth to be disabled")
	}
}

func TestAuthConfigFromConfig_NilConfig(t *testing.T) {
	authConfig, err := AuthConfigFromConfig(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if authConfig.EnableAuth {
		t.Error("Expected auth to be disabled for nil config")
	}
}

func TestAuthConfigFromConfig_AuthEnabled(t *testing.T) {
	usersJSON := `{
		"user1": {
			"password": "$2a$10$test",
			"roles": ["viewer"],
			"email": "user1@example.com"
		},
		"user2": {
			"password": "$2a$10$test2",
			"roles": ["admin", "viewer"]
		}
	}`

	cfg := &config.AuthConfig{
		EnableAuth:        true,
		JWTSecret:         "test-secret",
		JWTExpirationHours: 12,
		UsersJSON:         usersJSON,
	}

	authConfig, err := AuthConfigFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !authConfig.EnableAuth {
		t.Error("Expected auth to be enabled")
	}
	if authConfig.JWTSecret != "test-secret" {
		t.Errorf("Expected JWTSecret test-secret, got %s", authConfig.JWTSecret)
	}
	if authConfig.JWTExpiration != 12*time.Hour {
		t.Errorf("Expected expiration 12h, got %v", authConfig.JWTExpiration)
	}
	if len(authConfig.Users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(authConfig.Users))
	}

	user1, ok := authConfig.Users["user1"]
	if !ok {
		t.Error("Expected user1 to be present")
	}
	if len(user1.Roles) != 1 || user1.Roles[0] != "viewer" {
		t.Errorf("Expected user1 roles [viewer], got %v", user1.Roles)
	}
	if user1.Email != "user1@example.com" {
		t.Errorf("Expected user1 email user1@example.com, got %s", user1.Email)
	}

	user2, ok := authConfig.Users["user2"]
	if !ok {
		t.Error("Expected user2 to be present")
	}
	if len(user2.Roles) != 2 {
		t.Errorf("Expected user2 to have 2 roles, got %d", len(user2.Roles))
	}
}

func TestAuthConfigFromConfig_DefaultExpiration(t *testing.T) {
	cfg := &config.AuthConfig{
		EnableAuth:        true,
		JWTSecret:         "test-secret",
		JWTExpirationHours: 0, // Not set
		UsersJSON:         `{}`,
	}

	authConfig, err := AuthConfigFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if authConfig.JWTExpiration != 24*time.Hour {
		t.Errorf("Expected default expiration 24h, got %v", authConfig.JWTExpiration)
	}
}

func TestAuthConfigFromConfig_InvalidUsersJSON(t *testing.T) {
	cfg := &config.AuthConfig{
		EnableAuth: true,
		JWTSecret:  "test-secret",
		UsersJSON: "invalid json",
	}

	_, err := AuthConfigFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestAuthConfigFromConfig_EmptyUsersJSON(t *testing.T) {
	cfg := &config.AuthConfig{
		EnableAuth: true,
		JWTSecret:  "test-secret",
		UsersJSON:  "",
	}

	authConfig, err := AuthConfigFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(authConfig.Users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(authConfig.Users))
	}
}

func TestAuthConfigFromConfig_UsersJSONRoundTrip(t *testing.T) {
	// Test that we can marshal and unmarshal users correctly
	originalUsers := map[string]UserInfo{
		"user1": {
			Password: "$2a$10$test",
			Roles:    []string{"viewer"},
			Email:    "user1@example.com",
		},
		"user2": {
			Password: "$2a$10$test2",
			Roles:    []string{"admin"},
		},
	}

	usersJSONBytes, err := json.Marshal(originalUsers)
	if err != nil {
		t.Fatalf("Failed to marshal users: %v", err)
	}

	cfg := &config.AuthConfig{
		EnableAuth: true,
		JWTSecret:  "test-secret",
		UsersJSON:  string(usersJSONBytes),
	}

	authConfig, err := AuthConfigFromConfig(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(authConfig.Users) != len(originalUsers) {
		t.Errorf("Expected %d users, got %d", len(originalUsers), len(authConfig.Users))
	}

	for username, originalUser := range originalUsers {
		loadedUser, ok := authConfig.Users[username]
		if !ok {
			t.Errorf("User %s not found", username)
			continue
		}
		if loadedUser.Password != originalUser.Password {
			t.Errorf("User %s password mismatch", username)
		}
		if len(loadedUser.Roles) != len(originalUser.Roles) {
			t.Errorf("User %s roles count mismatch", username)
		}
		if loadedUser.Email != originalUser.Email {
			t.Errorf("User %s email mismatch", username)
		}
	}
}
