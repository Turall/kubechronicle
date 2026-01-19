package alerting

import "github.com/kubechronicle/kubechronicle/internal/model"

// Sender is the interface for alert senders.
type Sender interface {
	// Send sends an alert for a change event.
	Send(event *model.ChangeEvent) error
	// Name returns the name of the sender (e.g., "slack", "email").
	Name() string
}

// Config represents alerting configuration.
type Config struct {
	// Enabled channels
	Slack     *SlackConfig     `json:"slack,omitempty"`
	Telegram  *TelegramConfig  `json:"telegram,omitempty"`
	Email     *EmailConfig     `json:"email,omitempty"`
	Webhook   *WebhookConfig   `json:"webhook,omitempty"`
	
	// Filter configuration
	Operations []string `json:"operations,omitempty"` // Empty means all operations
}

// SlackConfig contains Slack alerting configuration.
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"` // Optional channel override
	Username   string `json:"username,omitempty"` // Optional username override
}

// TelegramConfig contains Telegram alerting configuration.
type TelegramConfig struct {
	BotToken string   `json:"bot_token"`
	ChatIDs  []string `json:"chat_ids"` // Multiple chat IDs supported
}

// EmailConfig contains email alerting configuration.
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username,omitempty"`
	SMTPPassword string   `json:"smtp_password,omitempty"`
	From         string   `json:"from"`
	To           []string `json:"to"`
	Subject      string   `json:"subject,omitempty"` // Optional subject template
}

// WebhookConfig contains webhook alerting configuration.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"` // Optional headers
	Method  string            `json:"method,omitempty"`  // Default: POST
}
