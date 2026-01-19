package alerting

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestNewTelegramSender_ValidConfig(t *testing.T) {
	cfg := &TelegramConfig{
		BotToken: "123456:ABC-DEF",
		ChatIDs:  []string{"123456789"},
	}

	sender, err := NewTelegramSender(cfg)
	if err != nil {
		t.Fatalf("NewTelegramSender() error = %v", err)
	}
	if sender == nil {
		t.Fatal("NewTelegramSender() returned nil")
	}
	if sender.Name() != "telegram" {
		t.Errorf("TelegramSender.Name() = %s, want telegram", sender.Name())
	}
}

func TestNewTelegramSender_InvalidConfig_MissingToken(t *testing.T) {
	cfg := &TelegramConfig{
		BotToken: "",
		ChatIDs:  []string{"123456789"},
	}

	_, err := NewTelegramSender(cfg)
	if err == nil {
		t.Error("NewTelegramSender() with missing token should return error")
	}
}

func TestNewTelegramSender_InvalidConfig_NoChatIDs(t *testing.T) {
	cfg := &TelegramConfig{
		BotToken: "123456:ABC-DEF",
		ChatIDs:  []string{},
	}

	_, err := NewTelegramSender(cfg)
	if err == nil {
		t.Error("NewTelegramSender() with no chat IDs should return error")
	}
}

func TestTelegramSender_Send(t *testing.T) {
	// Create a test server
	var receivedRequest *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	cfg := &TelegramConfig{
		BotToken: "123456:ABC-DEF",
		ChatIDs:  []string{"123456789"},
	}
	sender, _ := NewTelegramSender(cfg)
	sender.apiURL = server.URL + "/bot" // Override API URL for testing

	event := &model.ChangeEvent{
		ID:          "test-id",
		Timestamp:   time.Now(),
		Operation:   "CREATE",
		ResourceKind: "Deployment",
		Namespace:   "default",
		Name:        "test-app",
		Actor: model.Actor{
			Username: "user@example.com",
		},
		Source: model.Source{
			Tool: "kubectl",
		},
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("TelegramSender.Send() error = %v", err)
	}

	// Verify request
	if receivedRequest == nil {
		t.Fatal("No request received")
	}
	if receivedRequest.Method != "POST" {
		t.Errorf("Request method = %s, want POST", receivedRequest.Method)
	}
}

func TestTelegramSender_Send_MultipleChatIDs(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	cfg := &TelegramConfig{
		BotToken: "123456:ABC-DEF",
		ChatIDs:  []string{"123456789", "987654321"},
	}
	sender, _ := NewTelegramSender(cfg)
	sender.apiURL = server.URL + "/bot"

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("TelegramSender.Send() error = %v", err)
	}

	// Should send to both chat IDs
	if requestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", requestCount)
	}
}

func TestTelegramSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &TelegramConfig{
		BotToken: "123456:ABC-DEF",
		ChatIDs:  []string{"123456789"},
	}
	sender, _ := NewTelegramSender(cfg)
	sender.apiURL = server.URL + "/bot"

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err == nil {
		t.Error("TelegramSender.Send() should return error on server error")
	}
}

func TestFormatTelegramMessage(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		wantEmoji string
	}{
		{"CREATE operation", "CREATE", "✅"},
		{"UPDATE operation", "UPDATE", "🔄"},
		{"DELETE operation", "DELETE", "❌"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &model.ChangeEvent{
				Operation:    tt.operation,
				ResourceKind: "Deployment",
				Namespace:    "default",
				Name:         "test-app",
				Actor: model.Actor{
					Username: "user@example.com",
				},
				Source: model.Source{
					Tool: "kubectl",
				},
			}

			message := formatTelegramMessage(event)
			if message == "" {
				t.Error("formatTelegramMessage() should not return empty string")
			}
			if !strings.Contains(message, tt.wantEmoji) {
				t.Errorf("formatTelegramMessage() should contain emoji %s", tt.wantEmoji)
			}
			if !strings.Contains(message, "CREATE") || strings.Contains(message, tt.operation) {
				// Should contain operation
			}
		})
	}
}

func TestFormatTelegramMessage_WithServiceAccount(t *testing.T) {
	event := &model.ChangeEvent{
		Operation: "CREATE",
		Actor: model.Actor{
			Username:       "user@example.com",
			ServiceAccount: "system:serviceaccount:default:my-sa",
		},
	}

	message := formatTelegramMessage(event)
	if !strings.Contains(message, "Service Account") {
		t.Error("formatTelegramMessage() should include Service Account when present")
	}
}

func TestFormatTelegramMessage_WithDiff(t *testing.T) {
	event := &model.ChangeEvent{
		Operation: "UPDATE",
		Diff: []model.PatchOp{
			{Op: "replace", Path: "/spec/replicas", Value: 5},
			{Op: "add", Path: "/spec/template/spec/containers/0/env", Value: "test"},
		},
	}

	message := formatTelegramMessage(event)
	if !strings.Contains(message, "2 patch operation") {
		t.Error("formatTelegramMessage() should include diff count")
	}
}
