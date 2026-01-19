package alerting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestWebhookSender_Name(t *testing.T) {
	cfg := &WebhookConfig{
		URL: "https://example.com/webhook",
	}
	sender := NewWebhookSender(cfg)
	if sender.Name() != "webhook" {
		t.Errorf("WebhookSender.Name() = %s, want webhook", sender.Name())
	}
}

func TestWebhookSender_Send(t *testing.T) {
	// Create a test server
	var receivedEvent *model.ChangeEvent
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		var event model.ChangeEvent
		json.NewDecoder(r.Body).Decode(&event)
		receivedEvent = &event
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Headers: map[string]string{
			"Authorization":   "Bearer test-token",
			"X-Custom-Header": "custom-value",
		},
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		ID:           "test-id",
		Timestamp:    time.Now(),
		Operation:    "CREATE",
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

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("WebhookSender.Send() error = %v", err)
	}

	// Verify event was received
	if receivedEvent == nil {
		t.Fatal("No event received")
	}
	if receivedEvent.ID != event.ID {
		t.Errorf("Received event ID = %s, want %s", receivedEvent.ID, event.ID)
	}
	if receivedEvent.Operation != event.Operation {
		t.Errorf("Received event Operation = %s, want %s", receivedEvent.Operation, event.Operation)
	}

	// Verify headers
	if receivedHeaders.Get("Authorization") != "Bearer test-token" {
		t.Error("Authorization header not set correctly")
	}
	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Error("Custom header not set correctly")
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header should be application/json")
	}
}

func TestWebhookSender_Send_DefaultMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
		// No method specified, should default to POST
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("WebhookSender.Send() error = %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("Request method = %s, want POST", receivedMethod)
	}
}

func TestWebhookSender_Send_CustomMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL:    server.URL,
		Method: "PUT",
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("WebhookSender.Send() error = %v", err)
	}

	if receivedMethod != "PUT" {
		t.Errorf("Request method = %s, want PUT", receivedMethod)
	}
}

func TestWebhookSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err == nil {
		t.Error("WebhookSender.Send() should return error on server error")
	}
}

func TestWebhookSender_Send_InvalidURL(t *testing.T) {
	cfg := &WebhookConfig{
		URL: "http://invalid-url-that-does-not-exist.local/webhook",
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err == nil {
		t.Error("WebhookSender.Send() should return error on invalid URL")
	}
}

func TestWebhookSender_Send_NoHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &WebhookConfig{
		URL: server.URL,
		// No headers
	}
	sender := NewWebhookSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("WebhookSender.Send() error = %v", err)
	}

	// Should still have Content-Type
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header should be set even without custom headers")
	}
}
