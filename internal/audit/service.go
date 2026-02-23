package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/model"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

// Service processes Kubernetes audit logs and stores exec events.
type Service struct {
	processor *Processor
	store     store.Store
	queue     chan *model.ChangeEvent
}

// NewService creates a new audit log service.
func NewService(store store.Store) *Service {
	return &Service{
		processor: NewProcessor(),
		store:     store,
		queue:     make(chan *model.ChangeEvent, 1000), // Buffered channel for async processing
	}
}

// Start starts the async event processing worker.
func (s *Service) Start(ctx context.Context) {
	go s.processEvents(ctx)
}

// processEvents processes exec events asynchronously.
func (s *Service) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.queue:
			// Save to store
			if s.store != nil {
				if err := s.store.Save(event); err != nil {
					klog.Errorf("Failed to save exec event %s: %v", event.ID, err)
				} else {
					klog.Infof("Saved exec event %s: EXEC %s/%s in namespace %s (user: %s)",
						event.ID, event.ResourceKind, event.Name, event.Namespace, event.Actor.Username)
				}
			} else {
				klog.V(2).Infof("Exec event (no store): %+v", event)
			}
		}
	}
}

// ProcessAuditLogLine processes a single audit log line.
func (s *Service) ProcessAuditLogLine(line []byte) error {
	// Skip empty lines
	lineStr := strings.TrimSpace(string(line))
	if lineStr == "" {
		return nil
	}

	// Parse audit event
	auditEvent, err := s.processor.ParseAuditLog(line)
	if err != nil {
		klog.V(4).Infof("Failed to parse audit log line: %v", err)
		return nil // Skip invalid lines
	}

	// Check if it's an exec operation
	if !s.processor.IsExecOperation(auditEvent) {
		return nil // Not an exec operation, skip
	}

	// Only process successful exec operations (response code 200-299)
	if auditEvent.ResponseStatus != nil && auditEvent.ResponseStatus.Code < 200 || auditEvent.ResponseStatus.Code >= 300 {
		klog.V(3).Infof("Skipping exec operation with non-success status code: %d", auditEvent.ResponseStatus.Code)
		return nil
	}

	// Extract exec event
	execEvent, err := s.processor.ExtractExecEvent(auditEvent)
	if err != nil {
		klog.V(3).Infof("Failed to extract exec event: %v", err)
		return nil
	}

	// Queue for async processing (non-blocking)
	select {
	case s.queue <- execEvent:
		// Successfully queued
	default:
		// Queue full, log warning but don't block
		klog.Warningf("Event queue full, dropping exec event: %s", execEvent.ID)
	}

	return nil
}

// WatchAuditLogFile watches an audit log file and processes new lines.
func (s *Service) WatchAuditLogFile(ctx context.Context, filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("audit log file does not exist: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	// Seek to end of file to start reading new lines
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	scanner := bufio.NewScanner(file)
	ticker := time.NewTicker(100 * time.Millisecond) // Check for new lines every 100ms
	defer ticker.Stop()

	klog.Infof("Watching audit log file: %s", filePath)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Read new lines
			for scanner.Scan() {
				line := scanner.Bytes()
				if err := s.ProcessAuditLogLine(line); err != nil {
					klog.V(3).Infof("Error processing audit log line: %v", err)
				}
			}
			if err := scanner.Err(); err != nil {
				if err != io.EOF {
					klog.Errorf("Error reading audit log file: %v", err)
				}
			}
		}
	}
}

// WatchAuditLogDirectory watches a directory for audit log files.
func (s *Service) WatchAuditLogDirectory(ctx context.Context, dirPath string) error {
	klog.Infof("Watching audit log directory: %s", dirPath)

	ticker := time.NewTicker(5 * time.Second) // Check for new files every 5 seconds
	defer ticker.Stop()

	processedFiles := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// List files in directory
			files, err := os.ReadDir(dirPath)
			if err != nil {
				klog.Errorf("Failed to read audit log directory: %v", err)
				continue
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}

				// Only process JSON files
				if !strings.HasSuffix(file.Name(), ".log") && !strings.HasSuffix(file.Name(), ".json") {
					continue
				}

				filePath := filepath.Join(dirPath, file.Name())
				fileKey := filePath

				// Process file if not already processed
				if !processedFiles[fileKey] {
					processedFiles[fileKey] = true
					go func(path string) {
						if err := s.processAuditLogFile(ctx, path); err != nil {
							klog.Errorf("Error processing audit log file %s: %v", path, err)
						}
					}(filePath)
				}
			}
		}
	}
}

// processAuditLogFile processes an entire audit log file.
func (s *Service) processAuditLogFile(ctx context.Context, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	klog.Infof("Processing audit log file: %s", filePath)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Bytes()
		if err := s.ProcessAuditLogLine(line); err != nil {
			klog.V(3).Infof("Error processing audit log line: %v", err)
		}
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading audit log file: %w", err)
	}

	klog.Infof("Processed %d lines from audit log file: %s", lineCount, filePath)
	return nil
}

// HandleAuditWebhook handles incoming audit log events via HTTP webhook.
func (s *Service) HandleAuditWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse as JSON array (audit logs can be batched)
	var events []json.RawMessage
	if err := json.Unmarshal(body, &events); err != nil {
		// Try parsing as single event
		if err := s.ProcessAuditLogLine(body); err != nil {
			http.Error(w, fmt.Sprintf("Failed to process audit log: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Process each event in the batch
		for _, eventData := range events {
			if err := s.ProcessAuditLogLine(eventData); err != nil {
				klog.V(3).Infof("Error processing audit log event: %v", err)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
