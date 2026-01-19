package alerting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// SlackSender sends alerts to Slack via webhook.
type SlackSender struct {
	webhookURL string
	channel    string
	username   string
	client     *http.Client
}

// NewSlackSender creates a new Slack alert sender.
func NewSlackSender(cfg *SlackConfig) *SlackSender {
	return &SlackSender{
		webhookURL: cfg.WebhookURL,
		channel:    cfg.Channel,
		username:   cfg.Username,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the sender name.
func (s *SlackSender) Name() string {
	return "slack"
}

// Send sends an alert to Slack.
func (s *SlackSender) Send(event *model.ChangeEvent) error {
	// Format message
	message := formatSlackMessage(event)

	// Build payload
	payload := map[string]interface{}{
		"text": message,
	}

	if s.channel != "" {
		payload["channel"] = s.channel
	}
	if s.username != "" {
		payload["username"] = s.username
	}

	// Add color based on operation
	color := "#36a64f" // Green for CREATE
	if event.Operation == "UPDATE" {
		color = "#ffaa00" // Orange for UPDATE
	} else if event.Operation == "DELETE" {
		color = "#ff0000" // Red for DELETE
	}

	// Use attachment for better formatting
	attachment := map[string]interface{}{
		"color":     color,
		"title":     fmt.Sprintf("%s: %s/%s", event.Operation, event.ResourceKind, event.Name),
		"fields":    buildSlackFields(event),
		"timestamp": event.Timestamp.Unix(),
	}

	payload["attachments"] = []map[string]interface{}{attachment}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// Send HTTP request
	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Slack API returned status %d", resp.StatusCode)
	}

	return nil
}

func formatSlackMessage(event *model.ChangeEvent) string {
	return fmt.Sprintf("Kubernetes Resource %s: %s/%s/%s",
		event.Operation,
		event.ResourceKind,
		event.Namespace,
		event.Name,
	)
}

func buildSlackFields(event *model.ChangeEvent) []map[string]interface{} {
	fields := []map[string]interface{}{
		{"title": "Resource", "value": fmt.Sprintf("%s/%s", event.ResourceKind, event.Name), "short": true},
		{"title": "Namespace", "value": event.Namespace, "short": true},
		{"title": "User", "value": event.Actor.Username, "short": true},
		{"title": "Tool", "value": event.Source.Tool, "short": true},
	}

	if event.Actor.ServiceAccount != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Service Account",
			"value": event.Actor.ServiceAccount,
			"short": true,
		})
	}

	if event.Actor.SourceIP != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Source IP",
			"value": event.Actor.SourceIP,
			"short": true,
		})
	}

	if len(event.Diff) > 0 {
		diffSummary := fmt.Sprintf("%d change(s)", len(event.Diff))
		fields = append(fields, map[string]interface{}{
			"title": "Changes",
			"value": diffSummary,
			"short": false,
		})
	}

	return fields
}
