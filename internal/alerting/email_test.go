package alerting

import (
	"strings"
	"testing"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

func TestNewEmailSender_ValidConfig(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{"to@example.com"},
	}

	sender, err := NewEmailSender(cfg)
	if err != nil {
		t.Fatalf("NewEmailSender() error = %v", err)
	}
	if sender == nil {
		t.Fatal("NewEmailSender() returned nil")
	}
	if sender.Name() != "email" {
		t.Errorf("EmailSender.Name() = %s, want email", sender.Name())
	}
}

func TestNewEmailSender_InvalidConfig_MissingHost(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{"to@example.com"},
	}

	_, err := NewEmailSender(cfg)
	if err == nil {
		t.Error("NewEmailSender() with missing host should return error")
	}
}

func TestNewEmailSender_InvalidConfig_MissingPort(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 0,
		From:     "from@example.com",
		To:       []string{"to@example.com"},
	}

	_, err := NewEmailSender(cfg)
	if err == nil {
		t.Error("NewEmailSender() with missing port should return error")
	}
}

func TestNewEmailSender_InvalidConfig_MissingFrom(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "",
		To:       []string{"to@example.com"},
	}

	_, err := NewEmailSender(cfg)
	if err == nil {
		t.Error("NewEmailSender() with missing from address should return error")
	}
}

func TestNewEmailSender_InvalidConfig_NoRecipients(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{},
	}

	_, err := NewEmailSender(cfg)
	if err == nil {
		t.Error("NewEmailSender() with no recipients should return error")
	}
}

func TestEmailSender_GetSubject_Default(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{"to@example.com"},
		// No subject template
	}
	sender, _ := NewEmailSender(cfg)

	event := &model.ChangeEvent{
		Operation:    "CREATE",
		ResourceKind: "Deployment",
		Namespace:    "default",
		Name:         "test-app",
	}

	subject := sender.getSubject(event)
	if subject == "" {
		t.Error("getSubject() should not return empty string")
	}
	if !strings.Contains(subject, "CREATE") {
		t.Error("getSubject() should contain operation")
	}
}

func TestEmailSender_GetSubject_Template(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{"to@example.com"},
		Subject:  "[kubechronicle] {{operation}}: {{resource}} in {{namespace}}",
	}
	sender, _ := NewEmailSender(cfg)

	event := &model.ChangeEvent{
		Operation:    "UPDATE",
		ResourceKind: "Deployment",
		Namespace:    "production",
		Name:         "test-app",
	}

	subject := sender.getSubject(event)
	if !strings.Contains(subject, "UPDATE") {
		t.Error("getSubject() should replace {{operation}}")
	}
	if !strings.Contains(subject, "Deployment/test-app") {
		t.Error("getSubject() should replace {{resource}}")
	}
	if !strings.Contains(subject, "production") {
		t.Error("getSubject() should replace {{namespace}}")
	}
}

func TestFormatEmailBody(t *testing.T) {
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

	body := formatEmailBody(event)
	if body == "" {
		t.Error("formatEmailBody() should not return empty string")
	}
	if !strings.Contains(body, "CREATE") {
		t.Error("formatEmailBody() should contain operation")
	}
	if !strings.Contains(body, "Deployment") {
		t.Error("formatEmailBody() should contain resource kind")
	}
	if !strings.Contains(body, "user@example.com") {
		t.Error("formatEmailBody() should contain username")
	}
}

func TestFormatEmailBody_WithDiff(t *testing.T) {
	event := &model.ChangeEvent{
		Operation: "UPDATE",
		Diff: []model.PatchOp{
			{Op: "replace", Path: "/spec/replicas", Value: 5},
			{Op: "add", Path: "/spec/template/metadata/labels/env", Value: "prod"},
		},
	}

	body := formatEmailBody(event)
	if !strings.Contains(body, "2 patch operation") {
		t.Error("formatEmailBody() should include diff count")
	}
	if !strings.Contains(body, "replace /spec/replicas") {
		t.Error("formatEmailBody() should include diff operations")
	}
}

func TestEmailSender_BuildEmailMessage(t *testing.T) {
	cfg := &EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "from@example.com",
		To:       []string{"to1@example.com", "to2@example.com"},
	}
	sender, _ := NewEmailSender(cfg)

	subject := "Test Subject"
	body := "Test body content"

	message := sender.buildEmailMessage(subject, body)
	if message == "" {
		t.Error("buildEmailMessage() should not return empty string")
	}
	if !strings.Contains(message, subject) {
		t.Error("buildEmailMessage() should include subject")
	}
	if !strings.Contains(message, body) {
		t.Error("buildEmailMessage() should include body")
	}
	if !strings.Contains(message, "from@example.com") {
		t.Error("buildEmailMessage() should include From address")
	}
	if !strings.Contains(message, "to1@example.com") || !strings.Contains(message, "to2@example.com") {
		t.Error("buildEmailMessage() should include all To addresses")
	}
}
