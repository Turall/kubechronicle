package alerting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// WebhookSender sends alerts to a custom webhook endpoint.
type WebhookSender struct {
	url     string
	method  string
	headers map[string]string
	client  *http.Client
}

// NewWebhookSender creates a new webhook alert sender.
func NewWebhookSender(cfg *WebhookConfig) *WebhookSender {
	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	return &WebhookSender{
		url:     cfg.URL,
		method:  method,
		headers: cfg.Headers,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the sender name.
func (s *WebhookSender) Name() string {
	return "webhook"
}

// Send sends an alert to the webhook endpoint.
func (s *WebhookSender) Send(event *model.ChangeEvent) error {
	// Marshal event to JSON
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create request
	req, err := http.NewRequest(s.method, s.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range s.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
