package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateJWTSecret(t *testing.T) {
	secret1, err := GenerateJWTSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}
	if secret1 == "" {
		t.Error("Secret is empty")
	}

	secret2, err := GenerateJWTSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	if secret1 == secret2 {
		t.Error("Secrets should be different")
	}
}

func TestGenerateToken(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
		Email:    "test@example.com",
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	if token == "" {
		t.Error("Token is empty")
	}
}

func TestValidateToken(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer", "admin"},
		Email:    "test@example.com",
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	validatedUser, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if validatedUser.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, validatedUser.Username)
	}
	if len(validatedUser.Roles) != len(user.Roles) {
		t.Errorf("Expected %d roles, got %d", len(user.Roles), len(validatedUser.Roles))
	}
	if validatedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, validatedUser.Email)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
	}
	auth := NewAuthenticator(config)

	_, err := auth.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: -1 * time.Hour, // Expired
		EnableAuth:    true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait a bit to ensure expiration
	time.Sleep(100 * time.Millisecond)

	_, err = auth.ValidateToken(token)
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

func TestMiddleware_AuthDisabled(t *testing.T) {
	config := &AuthConfig{
		EnableAuth: false,
	}
	auth := NewAuthenticator(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddleware_AuthEnabled_NoToken(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestMiddleware_AuthEnabled_ValidToken(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
	}
	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUser(r)
		if !ok {
			t.Error("User not found in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if user.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", user.Username)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddleware_SkipHealthAndLogin(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	wrapped := middleware(handler)

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Health endpoint should be accessible, got %d", w.Code)
	}

	// Test login endpoint
	req = httptest.NewRequest("POST", "/kubechronicle/api/auth/login", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Login endpoint should be accessible, got %d", w.Code)
	}
}

func TestRequireRole(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
	}
	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	roleMiddleware := auth.RequireRole("admin")
	wrapped := middleware(roleMiddleware(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRequireRole_HasRole(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"admin", "viewer"},
	}
	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	roleMiddleware := auth.RequireRole("admin")
	wrapped := middleware(roleMiddleware(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequireAnyRole(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
	}
	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	roleMiddleware := auth.RequireAnyRole("admin", "editor")
	wrapped := middleware(roleMiddleware(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRequireAnyRole_HasOneRole(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)

	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer", "admin"},
	}
	token, err := auth.GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.Middleware()
	roleMiddleware := auth.RequireAnyRole("admin", "editor")
	wrapped := middleware(roleMiddleware(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetUser(t *testing.T) {
	user := &User{
		Username: "testuser",
		Roles:    []string{"viewer"},
	}

	ctx := context.WithValue(context.Background(), "user", user)
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)

	retrievedUser, ok := GetUser(req)
	if !ok {
		t.Error("Expected to get user from context")
	}
	if retrievedUser.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrievedUser.Username)
	}
}

func TestGetUser_NotInContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	_, ok := GetUser(req)
	if ok {
		t.Error("Expected not to get user from context")
	}
}

func TestNewAuthenticator_DefaultExpiration(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
		// JWTExpiration not set
	}
	auth := NewAuthenticator(config)

	if auth.config.JWTExpiration == 0 {
		t.Error("Expected default expiration to be set")
	}
	if auth.config.JWTExpiration != 24*time.Hour {
		t.Errorf("Expected default expiration 24h, got %v", auth.config.JWTExpiration)
	}
}
