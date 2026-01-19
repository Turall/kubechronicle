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

	"github.com/kubechronicle/kubechronicle/internal/admission"
	"github.com/kubechronicle/kubechronicle/internal/alerting"
	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

func main() {
	cfg := config.LoadConfig()

	var (
		port     = flag.Int("port", cfg.WebhookPort, "Port to listen on")
		certPath = flag.String("cert", cfg.TLSCertPath, "Path to TLS certificate")
		keyPath  = flag.String("key", cfg.TLSKeyPath, "Path to TLS private key")
	)
	flag.Parse()

	klog.Infof("Starting kubechronicle webhook on port %d", *port)
	klog.Infof("Certificate: %s, Key: %s", *certPath, *keyPath)

	// Initialize store
	var eventStore store.Store
	if cfg.DatabaseURL != "" {
		var err error
		eventStore, err = store.NewPostgreSQLStore(cfg.DatabaseURL)
		if err != nil {
			klog.Warningf("Failed to initialize store: %v, continuing without persistence", err)
		}
	}

	// Initialize alerting router
	var alertRouter *alerting.Router
	if cfg.AlertConfig != nil {
		var err error
		alertRouter, err = alerting.NewRouter(cfg.AlertConfig)
		if err != nil {
			klog.Warningf("Failed to initialize alerting: %v, continuing without alerts", err)
		} else if alertRouter != nil {
			klog.Info("Alerting enabled")
		}
	}

	// Log configuration
	if cfg.IgnoreConfig != nil {
		klog.Infof("Ignore config enabled: namespace_patterns=%v, name_patterns=%v, resource_kind_patterns=%v",
			cfg.IgnoreConfig.NamespacePatterns, cfg.IgnoreConfig.NamePatterns, cfg.IgnoreConfig.ResourceKindPatterns)
	} else {
		klog.Info("Ignore config is NOT enabled (nil)")
	}
	if cfg.BlockConfig != nil {
		klog.Infof("Block config enabled: namespace_patterns=%v, name_patterns=%v, resource_kind_patterns=%v, operation_patterns=%v",
			cfg.BlockConfig.NamespacePatterns, cfg.BlockConfig.NamePatterns, cfg.BlockConfig.ResourceKindPatterns, cfg.BlockConfig.OperationPatterns)
	} else {
		klog.Info("Block config is NOT enabled (nil)")
	}

	// Create admission handler
	handler := admission.NewHandler(eventStore, alertRouter, cfg.IgnoreConfig, cfg.BlockConfig)

	// Start async event processor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler.Start(ctx)

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", handler.HandleAdmissionReview)
	mux.HandleFunc("/health", healthCheck)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		klog.Infof("Webhook server listening on :%d", *port)
		if err := server.ListenAndServeTLS(*certPath, *keyPath); err != nil && err != http.ErrServerClosed {
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

	// Close store
	if eventStore != nil {
		if err := eventStore.Close(); err != nil {
			klog.Errorf("Error closing store: %v", err)
		}
	}

	cancel()
	klog.Info("Shutdown complete")
}

// healthCheck provides a simple health check endpoint.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
