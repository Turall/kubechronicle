package alerting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestSlackSender_Name(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/test",
	}
	sender := NewSlackSender(cfg)
	if sender.Name() != "slack" {
		t.Errorf("SlackSender.Name() = %s, want slack", sender.Name())
	}
}

func TestSlackSender_Send(t *testing.T) {
	// Create a test server
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		receivedPayload = payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		ID:          "test-id",
		Timestamp:   time.Now(),
		Operation:   "CREATE",
		ResourceKind: "Deployment",
		Namespace:   "default",
		Name:        "test-app",
		Actor: model.Actor{
			Username: "user@example.com",
			Groups:   []string{"system:authenticated"},
		},
		Source: model.Source{
			Tool: "kubectl",
		},
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("SlackSender.Send() error = %v", err)
	}

	// Verify payload structure
	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	// Check attachments exist
	attachments, ok := receivedPayload["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Error("Payload should contain attachments")
	}

	attachment := attachments[0].(map[string]interface{})
	if attachment["title"] == nil {
		t.Error("Attachment should have title")
	}
	if attachment["color"] == nil {
		t.Error("Attachment should have color")
	}
}

func TestSlackSender_Send_CREATE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
		ResourceKind: "Deployment",
		Name: "test",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("SlackSender.Send() error = %v", err)
	}
}

func TestSlackSender_Send_UPDATE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		Operation: "UPDATE",
		ResourceKind: "Deployment",
		Name: "test",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("SlackSender.Send() error = %v", err)
	}
}

func TestSlackSender_Send_DELETE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		Operation: "DELETE",
		ResourceKind: "Deployment",
		Name: "test",
	}

	err := sender.Send(event)
	if err != nil {
		t.Fatalf("SlackSender.Send() error = %v", err)
	}
}

func TestSlackSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &SlackConfig{
		WebhookURL: server.URL,
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err == nil {
		t.Error("SlackSender.Send() should return error on server error")
	}
}

func TestSlackSender_Send_InvalidURL(t *testing.T) {
	cfg := &SlackConfig{
		WebhookURL: "http://invalid-url-that-does-not-exist.local/test",
	}
	sender := NewSlackSender(cfg)

	event := &model.ChangeEvent{
		Operation: "CREATE",
	}

	err := sender.Send(event)
	if err == nil {
		t.Error("SlackSender.Send() should return error on invalid URL")
	}
}

func TestFormatSlackMessage(t *testing.T) {
	event := &model.ChangeEvent{
		Operation:    "CREATE",
		ResourceKind: "Deployment",
		Namespace:    "default",
		Name:         "test-app",
	}

	message := formatSlackMessage(event)
	if message == "" {
		t.Error("formatSlackMessage() should not return empty string")
	}
	if len(message) < 10 {
		t.Error("formatSlackMessage() should return meaningful message")
	}
}

func TestBuildSlackFields(t *testing.T) {
	event := &model.ChangeEvent{
		Operation:    "UPDATE",
		ResourceKind: "Deployment",
		Namespace:    "default",
		Name:         "test-app",
		Actor: model.Actor{
			Username:       "user@example.com",
			Groups:         []string{"system:authenticated"},
			ServiceAccount: "system:serviceaccount:default:my-sa",
			SourceIP:       "192.168.1.1",
		},
		Source: model.Source{
			Tool: "kubectl",
		},
		Diff: []model.PatchOp{
			{Op: "replace", Path: "/spec/replicas", Value: 5},
		},
	}

	fields := buildSlackFields(event)
	if len(fields) == 0 {
		t.Error("buildSlackFields() should return fields")
	}

	// Check basic fields
	foundResource := false
	for _, field := range fields {
		if title, ok := field["title"].(string); ok && title == "Resource" {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("buildSlackFields() should include Resource field")
	}
}
