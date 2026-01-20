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

	"github.com/kubechronicle/kubechronicle/internal/api"
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

	// Create API server
	apiServer := api.NewServer(eventStore)

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/changes", apiServer.HandleListChanges)
	mux.HandleFunc("/api/changes/", apiServer.HandleGetChange)
	mux.HandleFunc("/api/resources/", apiServer.HandleResourceHistory)
	mux.HandleFunc("/api/users/", apiServer.HandleUserActivity)
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("kubechronicle API server\n\nEndpoints:\n  GET /api/changes\n  GET /api/changes/{id}\n  GET /api/resources/{kind}/{namespace}/{name}/history\n  GET /api/users/{username}/activity\n  GET /health\n"))
		} else {
			http.NotFound(w, r)
		}
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
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
