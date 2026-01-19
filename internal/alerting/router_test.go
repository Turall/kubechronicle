package alerting

import (
	"testing"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestNewRouter_NilConfig(t *testing.T) {
	router, err := NewRouter(nil)
	if err != nil {
		t.Fatalf("NewRouter(nil) error = %v, want nil", err)
	}
	if router != nil {
		t.Error("NewRouter(nil) should return nil router")
	}
}

func TestNewRouter_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() error = %v, want nil", err)
	}
	if router != nil {
		t.Error("NewRouter() with no senders should return nil")
	}
}

func TestNewRouter_InvalidTelegramConfig(t *testing.T) {
	cfg := &Config{
		Telegram: &TelegramConfig{
			BotToken: "", // Missing token
			ChatIDs:  []string{"123"},
		},
	}
	// Router will try to create sender but skip if invalid (no error, just no sender)
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() should not return error (invalid configs are skipped): %v", err)
	}
	// Router should be nil because no valid senders were created
	if router != nil {
		t.Error("NewRouter() with invalid Telegram config should return nil (no valid senders)")
	}
}

func TestNewRouter_InvalidEmailConfig(t *testing.T) {
	cfg := &Config{
		Email: &EmailConfig{
			SMTPHost: "", // Missing host
			SMTPPort: 587,
			From:     "from@example.com",
			To:       []string{"to@example.com"},
		},
	}
	// Router will try to create sender but skip if invalid (no error, just no sender)
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() should not return error (invalid configs are skipped): %v", err)
	}
	// Router should be nil because no valid senders were created
	if router != nil {
		t.Error("NewRouter() with invalid Email config should return nil (no valid senders)")
	}
}

func TestRouter_ShouldAlert_AllOperations(t *testing.T) {
	cfg := &Config{
		Slack: &SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/test",
		},
		// No operations filter = all operations
	}
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	tests := []struct {
		name      string
		operation string
		want      bool
	}{
		{"CREATE operation", "CREATE", true},
		{"UPDATE operation", "UPDATE", true},
		{"DELETE operation", "DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &model.ChangeEvent{Operation: tt.operation}
			got := router.ShouldAlert(event)
			if got != tt.want {
				t.Errorf("Router.ShouldAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRouter_ShouldAlert_FilteredOperations(t *testing.T) {
	cfg := &Config{
		Slack: &SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/test",
		},
		Operations: []string{"CREATE", "DELETE"},
	}
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	tests := []struct {
		name      string
		operation string
		want      bool
	}{
		{"CREATE operation (allowed)", "CREATE", true},
		{"UPDATE operation (filtered)", "UPDATE", false},
		{"DELETE operation (allowed)", "DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &model.ChangeEvent{Operation: tt.operation}
			got := router.ShouldAlert(event)
			if got != tt.want {
				t.Errorf("Router.ShouldAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRouter_ShouldAlert_NilRouter(t *testing.T) {
	var router *Router
	event := &model.ChangeEvent{Operation: "CREATE"}
	got := router.ShouldAlert(event)
	if got {
		t.Error("Router.ShouldAlert() on nil router should return false")
	}
}

func TestRouter_Send_NilRouter(t *testing.T) {
	var router *Router
	event := &model.ChangeEvent{Operation: "CREATE"}
	// Should not panic
	router.Send(event)
}

func TestRouter_Send_FilteredOut(t *testing.T) {
	cfg := &Config{
		Slack: &SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/test",
		},
		Operations: []string{"CREATE"}, // Only CREATE
	}
	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	// UPDATE event should be filtered out
	event := &model.ChangeEvent{Operation: "UPDATE"}
	// Should not panic (senders won't be called)
	router.Send(event)
}
