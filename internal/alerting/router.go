package alerting

import (
	"fmt"
	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// Router routes change events to configured alert senders.
type Router struct {
	senders    []Sender
	operations map[string]bool // Set of allowed operations (empty = all)
}

// NewRouter creates a new alert router with the given configuration.
func NewRouter(cfg *Config) (*Router, error) {
	if cfg == nil {
		return nil, nil // No alerting configured
	}

	r := &Router{
		senders:    make([]Sender, 0),
		operations: make(map[string]bool),
	}

	// Build operation filter
	if len(cfg.Operations) > 0 {
		for _, op := range cfg.Operations {
			r.operations[op] = true
		}
	}
	// If empty, allow all operations (map stays empty)

	// Initialize Slack sender
	if cfg.Slack != nil && cfg.Slack.WebhookURL != "" {
		sender := NewSlackSender(cfg.Slack)
		r.senders = append(r.senders, sender)
		klog.Infof("Slack alerting enabled")
	}

	// Initialize Telegram sender
	if cfg.Telegram != nil && cfg.Telegram.BotToken != "" && len(cfg.Telegram.ChatIDs) > 0 {
		sender, err := NewTelegramSender(cfg.Telegram)
		if err != nil {
			return nil, fmt.Errorf("failed to create Telegram sender: %w", err)
		}
		r.senders = append(r.senders, sender)
		klog.Infof("Telegram alerting enabled for %d chat(s)", len(cfg.Telegram.ChatIDs))
	}

	// Initialize Email sender
	if cfg.Email != nil && cfg.Email.SMTPHost != "" && len(cfg.Email.To) > 0 {
		sender, err := NewEmailSender(cfg.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to create Email sender: %w", err)
		}
		r.senders = append(r.senders, sender)
		klog.Infof("Email alerting enabled to %d recipient(s)", len(cfg.Email.To))
	}

	// Initialize Webhook sender
	if cfg.Webhook != nil && cfg.Webhook.URL != "" {
		sender := NewWebhookSender(cfg.Webhook)
		r.senders = append(r.senders, sender)
		klog.Infof("Webhook alerting enabled: %s", cfg.Webhook.URL)
	}

	if len(r.senders) == 0 {
		return nil, nil // No senders configured
	}

	return r, nil
}

// ShouldAlert checks if the event should trigger an alert based on operation filter.
func (r *Router) ShouldAlert(event *model.ChangeEvent) bool {
	if r == nil {
		return false
	}

	// If no operations specified, alert on all
	if len(r.operations) == 0 {
		return true
	}

	// Check if operation is in allowed set
	return r.operations[event.Operation]
}

// Send sends alerts for the given change event to all configured senders.
func (r *Router) Send(event *model.ChangeEvent) {
	if r == nil {
		return
	}

	// Check if we should alert for this operation
	if !r.ShouldAlert(event) {
		return
	}

	// Send to all configured senders (async, non-blocking)
	for _, sender := range r.senders {
		go func(s Sender) {
			if err := s.Send(event); err != nil {
				klog.Errorf("Failed to send alert via %s: %v", s.Name(), err)
			}
		}(sender)
	}
}
