package auth

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	"k8s.io/klog/v2"
)

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// LoginHandler handles login requests.
type LoginHandler struct {
	auth *Authenticator
}

// NewLoginHandler creates a new login handler.
func NewLoginHandler(auth *Authenticator) *LoginHandler {
	return &LoginHandler{
		auth: auth,
	}
}

// HandleLogin handles POST /api/auth/login requests.
func (h *LoginHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers on all responses
	h.setCORSHeaders(w)
	
	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate credentials
	userInfo, ok := h.auth.config.Users[req.Username]
	if !ok {
		klog.V(2).Infof("Login attempt with unknown username: %s", req.Username)
		h.sendError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(userInfo.Password), []byte(req.Password)); err != nil {
		klog.V(2).Infof("Login attempt with invalid password for user: %s", req.Username)
		h.sendError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate token
	user := &User{
		Username: req.Username,
		Roles:    userInfo.Roles,
		Email:    userInfo.Email,
	}

	token, err := h.auth.GenerateToken(user)
	if err != nil {
		klog.Errorf("Failed to generate token: %v", err)
		h.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Send response
	response := LoginResponse{
		Token: token,
		User:  *user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// setCORSHeaders sets CORS headers on the response.
func (h *LoginHandler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// sendError sends an error response with CORS headers.
func (h *LoginHandler) sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	w.Write([]byte(message))
}
