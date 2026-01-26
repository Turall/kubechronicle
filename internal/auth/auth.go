package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"k8s.io/klog/v2"
)

// User represents an authenticated user.
type User struct {
	Username string
	Roles    []string
	Email    string
}

// Claims represents JWT claims.
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Email    string   `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// JWTSecret is the secret key for signing JWT tokens
	JWTSecret string
	
	// JWTExpiration is the token expiration time (default: 24 hours)
	JWTExpiration time.Duration
	
	// EnableAuth enables authentication (if false, all requests are allowed)
	EnableAuth bool
	
	// Users is a map of username -> user info (for simple auth)
	Users map[string]UserInfo
}

// UserInfo holds user information for authentication.
type UserInfo struct {
	Password string   `json:"password"` // bcrypt hashed
	Roles    []string `json:"roles"`
	Email    string   `json:"email,omitempty"`
}

// Authenticator handles authentication and authorization.
type Authenticator struct {
	config *AuthConfig
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(config *AuthConfig) *Authenticator {
	if config.JWTExpiration == 0 {
		config.JWTExpiration = 24 * time.Hour
	}
	return &Authenticator{
		config: config,
	}
}

// GenerateJWTSecret generates a random JWT secret.
func GenerateJWTSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateToken generates a JWT token for a user.
func (a *Authenticator) GenerateToken(user *User) (string, error) {
	expirationTime := time.Now().Add(a.config.JWTExpiration)
	claims := &Claims{
		Username: user.Username,
		Roles:    user.Roles,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "kubechronicle",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the user.
func (a *Authenticator) ValidateToken(tokenString string) (*User, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return &User{
		Username: claims.Username,
		Roles:    claims.Roles,
		Email:    claims.Email,
	}, nil
}

// Middleware returns an HTTP middleware for authentication.
func (a *Authenticator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health check and login endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/api/auth/login" {
				next.ServeHTTP(w, r)
				return
			}

			// If auth is disabled, allow all requests
			if !a.config.EnableAuth {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Parse Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			user, err := a.ValidateToken(tokenString)
			if err != nil {
				klog.V(2).Infof("Token validation failed: %v", err)
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), "user", user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns a middleware that requires a specific role.
func (a *Authenticator) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*User)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, r := range user.Roles {
				if r == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole returns a middleware that requires any of the specified roles.
func (a *Authenticator) RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*User)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range user.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUser extracts the user from the request context.
func GetUser(r *http.Request) (*User, bool) {
	user, ok := r.Context().Value("user").(*User)
	return user, ok
}
