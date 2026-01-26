package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestHandleLogin_Success(t *testing.T) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	config := &AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
		Users: map[string]UserInfo{
			"testuser": {
				Password: string(hashedPassword),
				Roles:    []string{"viewer"},
				Email:    "test@example.com",
			},
		},
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Token == "" {
		t.Error("Token is empty")
	}
	if response.User.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", response.User.Username)
	}
	if len(response.User.Roles) != 1 || response.User.Roles[0] != "viewer" {
		t.Errorf("Expected roles [viewer], got %v", response.User.Roles)
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	config := &AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
		Users: map[string]UserInfo{
			"testuser": {
				Password: string(hashedPassword),
				Roles:    []string{"viewer"},
			},
		},
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandleLogin_UnknownUser(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: 1 * time.Hour,
		EnableAuth:    true,
		Users:         map[string]UserInfo{},
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	loginReq := LoginRequest{
		Username: "unknownuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandleLogin_InvalidMethod(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	req := httptest.NewRequest("GET", "/api/auth/login", nil)
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleLogin_InvalidBody(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleLogin_Options(t *testing.T) {
	config := &AuthConfig{
		JWTSecret:  "test-secret",
		EnableAuth: true,
	}
	auth := NewAuthenticator(config)
	handler := NewLoginHandler(auth)

	req := httptest.NewRequest("OPTIONS", "/api/auth/login", nil)
	w := httptest.NewRecorder()

	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
		t.Errorf("Expected Access-Control-Allow-Methods 'POST, OPTIONS', got %s", w.Header().Get("Access-Control-Allow-Methods"))
	}
}
