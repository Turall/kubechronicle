package auth

import (
	"encoding/json"
	"time"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/config"
)

// AuthConfigFromConfig converts config.AuthConfig to auth.AuthConfig.
func AuthConfigFromConfig(cfg *config.AuthConfig) (*AuthConfig, error) {
	if cfg == nil || !cfg.EnableAuth {
		return &AuthConfig{
			EnableAuth: false,
		}, nil
	}

	authConfig := &AuthConfig{
		EnableAuth: true,
		JWTSecret:  cfg.JWTSecret,
		Users:      make(map[string]UserInfo),
	}

	// Set expiration
	if cfg.JWTExpirationHours > 0 {
		authConfig.JWTExpiration = time.Duration(cfg.JWTExpirationHours) * time.Hour
	} else {
		authConfig.JWTExpiration = 24 * time.Hour
	}

	// Parse users
	if cfg.UsersJSON != "" {
		var usersMap map[string]UserInfo
		if err := json.Unmarshal([]byte(cfg.UsersJSON), &usersMap); err != nil {
			return nil, err
		}
		authConfig.Users = usersMap
		klog.Infof("Loaded %d users for authentication", len(usersMap))
	}

	return authConfig, nil
}
