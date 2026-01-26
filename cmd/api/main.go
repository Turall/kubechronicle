package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/admin"
	"github.com/kubechronicle/kubechronicle/internal/api"
	"github.com/kubechronicle/kubechronicle/internal/auth"
	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

func main() {
	cfg := config.LoadConfig()

	var (
		port = flag.Int("port", 8080, "Port to listen on")
	)
	flag.Parse()

	klog.Infof("Starting kubechronicle API server on port %d", *port)

	// Initialize store
	if cfg.DatabaseURL == "" {
		klog.Fatal("DATABASE_URL environment variable is required for API server")
	}

	eventStore, err := store.NewPostgreSQLStore(cfg.DatabaseURL)
	if err != nil {
		klog.Fatalf("Failed to initialize store: %v", err)
	}
	defer eventStore.Close()

	// Set up authentication
	var authenticator *auth.Authenticator
	var handler http.Handler
	if cfg.AuthConfig != nil && cfg.AuthConfig.EnableAuth {
		authConfig, err := auth.AuthConfigFromConfig(cfg.AuthConfig)
		if err != nil {
			klog.Fatalf("Failed to initialize auth config: %v", err)
		}
		authenticator = auth.NewAuthenticator(authConfig)
		klog.Info("Authentication enabled")
	} else {
		// Create a disabled authenticator
		authConfig := &auth.AuthConfig{EnableAuth: false}
		authenticator = auth.NewAuthenticator(authConfig)
		klog.Info("Authentication disabled - all requests allowed")
	}

	// Create API server
	apiServer := api.NewServer(eventStore)

	// Initialize Kubernetes client for admin endpoints (optional)
	var patternsHandler *admin.PatternsHandler
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "kubechronicle"
	}
	configMapName := os.Getenv("PATTERNS_CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = "kubechronicle-patterns"
	}

	k8sClient, err := admin.NewKubernetesClient()
	if err != nil {
		klog.Warningf("Failed to initialize Kubernetes client for admin endpoints: %v. Admin pattern management will be disabled.", err)
	} else {
		patternsHandler = admin.NewPatternsHandler(k8sClient, namespace, configMapName)
		klog.Info("Admin pattern management enabled")
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	
	// Login endpoint (no auth required)
	if cfg.AuthConfig != nil && cfg.AuthConfig.EnableAuth {
		loginHandler := auth.NewLoginHandler(authenticator)
		mux.HandleFunc("/api/auth/login", loginHandler.HandleLogin)
	}
	
	// API endpoints (protected by auth middleware)
	mux.HandleFunc("/api/changes", apiServer.HandleListChanges)
	mux.HandleFunc("/api/changes/", apiServer.HandleGetChange)
	mux.HandleFunc("/api/resources/", apiServer.HandleResourceHistory)
	mux.HandleFunc("/api/users/", apiServer.HandleUserActivity)
	
	// Admin endpoints (require admin role)
	if patternsHandler != nil {
		adminMux := http.NewServeMux()
		adminMux.HandleFunc("/api/admin/patterns/ignore", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				patternsHandler.HandleGetIgnoreConfig(w, r)
			} else if r.Method == http.MethodPut {
				patternsHandler.HandleUpdateIgnoreConfig(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		adminMux.HandleFunc("/api/admin/patterns/block", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				patternsHandler.HandleGetBlockConfig(w, r)
			} else if r.Method == http.MethodPut {
				patternsHandler.HandleUpdateBlockConfig(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		
		// Wrap admin endpoints with admin role requirement
		if cfg.AuthConfig != nil && cfg.AuthConfig.EnableAuth {
			mux.Handle("/api/admin/", authenticator.RequireRole("admin")(adminMux))
		} else {
			// If auth is disabled, allow all (for development)
			mux.Handle("/api/admin/", adminMux)
		}
	}
	
	// Health check (no auth required)
	mux.HandleFunc("/health", healthCheck)
	
	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/plain")
			message := "kubechronicle API server\n\nEndpoints:\n  POST /api/auth/login\n  GET /api/changes\n  GET /api/changes/{id}\n  GET /api/resources/{kind}/{namespace}/{name}/history\n  GET /api/users/{username}/activity\n  GET /health\n"
			w.Write([]byte(message))
		} else {
			http.NotFound(w, r)
		}
	})

	// Apply authentication middleware
	handler = authenticator.Middleware()(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		klog.Infof("API server listening on :%d", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	klog.Info("Shutting down...")

	// Shutdown server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Error during server shutdown: %v", err)
	}

	klog.Info("Shutdown complete")
}

// healthCheck provides a simple health check endpoint.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
