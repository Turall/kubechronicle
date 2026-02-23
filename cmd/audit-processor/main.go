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

	"github.com/kubechronicle/kubechronicle/internal/audit"
	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	// Command line flags
	var (
		auditLogFile      = flag.String("audit-log-file", "", "Path to Kubernetes audit log file to watch")
		auditLogDir       = flag.String("audit-log-dir", "", "Path to directory containing audit log files")
		webhookPort       = flag.Int("webhook-port", 8444, "Port for audit log webhook endpoint")
		enableWebhook     = flag.Bool("enable-webhook", false, "Enable HTTP webhook endpoint for receiving audit logs")
		databaseURL       = flag.String("database-url", "", "PostgreSQL connection string (or use DATABASE_URL env var)")
	)
	flag.Parse()

	// Load configuration
	cfg := config.LoadConfig()
	if *databaseURL != "" {
		cfg.DatabaseURL = *databaseURL
	}

	// Initialize store
	var storeInstance store.Store
	if cfg.DatabaseURL != "" {
		var err error
		storeInstance, err = store.NewPostgreSQLStore(cfg.DatabaseURL)
		if err != nil {
			klog.Errorf("Failed to initialize store: %v, continuing without persistence", err)
		} else {
			defer storeInstance.Close()
		}
	} else {
		klog.Warning("No database URL provided, exec events will not be persisted")
	}

	// Create audit service
	auditService := audit.NewService(storeInstance)

	// Start event processing worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	auditService.Start(ctx)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start audit log watching
	if *auditLogFile != "" {
		klog.Infof("Starting audit log file watcher: %s", *auditLogFile)
		go func() {
			if err := auditService.WatchAuditLogFile(ctx, *auditLogFile); err != nil {
				klog.Errorf("Error watching audit log file: %v", err)
			}
		}()
	} else if *auditLogDir != "" {
		klog.Infof("Starting audit log directory watcher: %s", *auditLogDir)
		go func() {
			if err := auditService.WatchAuditLogDirectory(ctx, *auditLogDir); err != nil {
				klog.Errorf("Error watching audit log directory: %v", err)
			}
		}()
	} else if !*enableWebhook {
		klog.Warning("No audit log source specified. Use -audit-log-file, -audit-log-dir, or -enable-webhook")
	}

	// Start webhook server if enabled
	if *enableWebhook {
		http.HandleFunc("/audit", auditService.HandleAuditWebhook)
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		server := &http.Server{
			Addr:         fmt.Sprintf(":%d", *webhookPort),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		klog.Infof("Starting audit log webhook server on port %d", *webhookPort)
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.Errorf("Audit log webhook server error: %v", err)
			}
		}()

		// Graceful shutdown for webhook server
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				klog.Errorf("Error shutting down webhook server: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	<-sigChan
	klog.Info("Shutting down audit log processor...")
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	klog.Info("Audit log processor stopped")
}
