package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/alerting"
	"github.com/kubechronicle/kubechronicle/internal/config"
	"github.com/kubechronicle/kubechronicle/internal/model"
	"github.com/kubechronicle/kubechronicle/internal/store"
)

// Handler processes Kubernetes admission requests.
type Handler struct {
	decoder      *Decoder
	store        store.Store
	alertRouter  *alerting.Router
	ignoreConfig *config.IgnoreConfig
	blockConfig  *config.BlockConfig
	queue        chan *model.ChangeEvent
}

// NewHandler creates a new admission handler.
func NewHandler(store store.Store, alertRouter *alerting.Router, ignoreConfig *config.IgnoreConfig, blockConfig *config.BlockConfig) *Handler {
	return &Handler{
		decoder:      NewDecoder(),
		store:        store,
		alertRouter:  alertRouter,
		ignoreConfig: ignoreConfig,
		blockConfig:  blockConfig,
		queue:        make(chan *model.ChangeEvent, 1000), // Buffered channel for async processing
	}
}

// Start starts the async event processing worker.
func (h *Handler) Start(ctx context.Context) {
	go h.processEvents(ctx)
}

// processEvents processes change events asynchronously.
func (h *Handler) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-h.queue:
			// Save to store
			if h.store != nil {
				if err := h.store.Save(event); err != nil {
					klog.Errorf("Failed to save change event %s: %v", event.ID, err)
				} else {
					klog.Infof("Saved change event %s: %s %s/%s", event.ID, event.Operation, event.ResourceKind, event.Name)
				}
			} else {
				klog.V(2).Infof("Change event (no store): %+v", event)
			}

			// Send alerts
			if h.alertRouter != nil {
				h.alertRouter.Send(event)
			}
		}
	}
}

// HandleAdmissionReview handles an AdmissionReview request and returns a response.
// This function always allows requests (observe-only) and processes them asynchronously.
func (h *Handler) HandleAdmissionReview(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Ensure we only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	var body []byte
	if r.Body != nil {
		var err error
		body, err = readBody(r)
		if err != nil {
			klog.Errorf("Failed to read request body: %v", err)
			h.sendErrorResponse(w, fmt.Errorf("failed to read body: %w", err))
			return
		}
	}

	// Decode AdmissionReview
	review, err := h.decoder.DecodeAdmissionReview(body)
	if err != nil {
		klog.Errorf("Failed to decode AdmissionReview: %v", err)
		h.sendErrorResponse(w, err)
		return
	}

	// Extract change event to check for blocking
	// We need to decode before responding to check block patterns
	event, err := h.decoder.DecodeRequest(review.Request)
	if err != nil {
		klog.Errorf("Failed to decode request: %v", err)
		// On decode error, fail-open (allow the request)
		response := &admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: &admissionv1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: true, // Fail-open on decode errors
			},
		}
		if err := h.sendResponse(w, response); err != nil {
			klog.Errorf("Failed to send response: %v", err)
		}
		return
	}

	// Debug: Log event details and config state for troubleshooting
	klog.V(2).Infof("Processing event: operation=%s, kind=%s, name=%s, namespace=%s, ignoreConfig=%v, blockConfig=%v",
		event.Operation, event.ResourceKind, event.Name, event.Namespace,
		h.ignoreConfig != nil, h.blockConfig != nil)
	if h.ignoreConfig != nil {
		klog.V(2).Infof("Ignore patterns: namespace=%v, name=%v, kind=%v",
			h.ignoreConfig.NamespacePatterns, h.ignoreConfig.NamePatterns, h.ignoreConfig.ResourceKindPatterns)
	}
	if h.blockConfig != nil {
		klog.V(2).Infof("Block patterns: namespace=%v, name=%v, kind=%v, operations=%v",
			h.blockConfig.NamespacePatterns, h.blockConfig.NamePatterns, h.blockConfig.ResourceKindPatterns, h.blockConfig.OperationPatterns)
	}

	// Check if this event should be blocked
	shouldBlock, blockPattern, blockMessage := ShouldBlock(event, h.blockConfig)
	if shouldBlock {
		// Set timestamp and ID for tracking blocked events
		event.Timestamp = time.Now()
		event.ID = generateEventID(event)
		event.Allowed = false
		event.BlockPattern = blockPattern

		klog.Warningf("Blocking %s: %s/%s in namespace %s (user: %s, source: %s) - pattern: %s, message: %s",
			event.Operation,
			event.ResourceKind,
			event.Name,
			event.Namespace,
			event.Actor.Username,
			event.Source.Tool,
			blockPattern,
			blockMessage,
		)

		// Save blocked event to database (if store is available)
		// This allows tracking of blocked attempts
		if h.store != nil {
			select {
			case h.queue <- event:
				// Successfully queued for async save
			default:
				klog.Warningf("Event queue full, dropping blocked event: %s", event.ID)
			}
		}

		// Deny the request
		response := &admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: &admissionv1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: false, // Block the request
				Result: &metav1.Status{
					Message: blockMessage,
					Reason:  metav1.StatusReasonForbidden,
					Code:    http.StatusForbidden,
				},
			},
		}
		if err := h.sendResponse(w, response); err != nil {
			klog.Errorf("Failed to send block response: %v", err)
		}
		return
	}

	// Check if this event should be ignored (but still allowed)
	shouldIgnore := ShouldIgnore(event, h.ignoreConfig)
	if shouldIgnore {
		klog.Infof("Ignoring %s: %s/%s in namespace %s (matches ignore pattern)",
			event.Operation,
			event.ResourceKind,
			event.Name,
			event.Namespace,
		)
		// Still allow the request, just don't process it
		response := &admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: &admissionv1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: true,
			},
		}
		if err := h.sendResponse(w, response); err != nil {
			klog.Errorf("Failed to send response: %v", err)
		}
		return
	}

	// Set timestamp and ID for tracking
	event.Timestamp = time.Now()
	event.ID = generateEventID(event)
	event.Allowed = true    // Operation was allowed
	event.BlockPattern = "" // No block pattern matched

	// Log the operation
	klog.Infof("Processing %s: %s/%s in namespace %s (user: %s, source: %s)",
		event.Operation,
		event.ResourceKind,
		event.Name,
		event.Namespace,
		event.Actor.Username,
		event.Source.Tool,
	)

	// Queue for async processing (non-blocking)
	select {
	case h.queue <- event:
		// Successfully queued
	default:
		// Queue full, log warning but don't block
		klog.Warningf("Event queue full, dropping event: %s", event.ID)
	}

	// Allow the request (observe-only, unless blocked above)
	response := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: true,
		},
	}

	// Send response
	if err := h.sendResponse(w, response); err != nil {
		klog.Errorf("Failed to send response: %v", err)
		return
	}

	// Log performance
	duration := time.Since(startTime)
	if duration > 100*time.Millisecond {
		klog.Warningf("Webhook response took %v (target: <100ms)", duration)
	} else {
		klog.V(3).Infof("Webhook response took %v", duration)
	}
}

// sendResponse sends an AdmissionReview response.
func (h *Handler) sendResponse(w http.ResponseWriter, review *admissionv1.AdmissionReview) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(review)
}

// sendErrorResponse sends an error response that still allows the request (fail-open).
func (h *Handler) sendErrorResponse(w http.ResponseWriter, err error) {
	// Even on error, we allow the request (fail-open behavior)
	response := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			Allowed: true, // Fail-open: always allow
			Result: &metav1.Status{
				Message: fmt.Sprintf("kubechronicle error (allowed): %v", err),
			},
		},
	}

	if err := h.sendResponse(w, response); err != nil {
		klog.Errorf("Failed to send error response: %v", err)
	}
}

// readBody reads the request body and restores it for potential re-reading.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// generateEventID generates a unique ID for a change event.
func generateEventID(event *model.ChangeEvent) string {
	// Simple ID generation: timestamp + resource identifier
	return fmt.Sprintf("%s-%s-%s-%d",
		event.Operation,
		event.ResourceKind,
		event.Name,
		event.Timestamp.UnixNano(),
	)
}
