package alerting

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// EmailSender sends alerts via email.
type EmailSender struct {
	config *EmailConfig
}

// NewEmailSender creates a new email alert sender.
func NewEmailSender(cfg *EmailConfig) (*EmailSender, error) {
	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if cfg.SMTPPort == 0 {
		return nil, fmt.Errorf("SMTP port is required")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if len(cfg.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	return &EmailSender{
		config: cfg,
	}, nil
}

// Name returns the sender name.
func (s *EmailSender) Name() string {
	return "email"
}

// Send sends an alert via email.
func (s *EmailSender) Send(event *model.ChangeEvent) error {
	subject := s.getSubject(event)
	body := formatEmailBody(event)

	// Build message
	message := s.buildEmailMessage(subject, body)

	// SMTP address
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	// Authentication
	var auth smtp.Auth
	if s.config.SMTPUsername != "" && s.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	// Send email
	err := smtp.SendMail(addr, auth, s.config.From, s.config.To, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *EmailSender) getSubject(event *model.ChangeEvent) string {
	if s.config.Subject != "" {
		// Simple template replacement
		subject := s.config.Subject
		subject = strings.ReplaceAll(subject, "{{operation}}", event.Operation)
		subject = strings.ReplaceAll(subject, "{{resource}}", fmt.Sprintf("%s/%s", event.ResourceKind, event.Name))
		subject = strings.ReplaceAll(subject, "{{namespace}}", event.Namespace)
		return subject
	}

	// Default subject
	return fmt.Sprintf("[kubechronicle] %s: %s/%s/%s",
		event.Operation,
		event.ResourceKind,
		event.Namespace,
		event.Name,
	)
}

func formatEmailBody(event *model.ChangeEvent) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Kubernetes Resource %s\n", event.Operation))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")
	sb.WriteString(fmt.Sprintf("Resource: %s/%s\n", event.ResourceKind, event.Name))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", event.Namespace))
	sb.WriteString(fmt.Sprintf("Operation: %s\n", event.Operation))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", event.Timestamp.Format(time.RFC3339)))

	sb.WriteString("Actor Information:\n")
	sb.WriteString(fmt.Sprintf("  Username: %s\n", event.Actor.Username))
	if len(event.Actor.Groups) > 0 {
		sb.WriteString(fmt.Sprintf("  Groups: %s\n", strings.Join(event.Actor.Groups, ", ")))
	}
	if event.Actor.ServiceAccount != "" {
		sb.WriteString(fmt.Sprintf("  Service Account: %s\n", event.Actor.ServiceAccount))
	}
	if event.Actor.SourceIP != "" {
		sb.WriteString(fmt.Sprintf("  Source IP: %s\n", event.Actor.SourceIP))
	}

	sb.WriteString(fmt.Sprintf("\nSource Tool: %s\n", event.Source.Tool))

	if len(event.Diff) > 0 {
		sb.WriteString(fmt.Sprintf("\nChanges: %d patch operation(s)\n", len(event.Diff)))
		sb.WriteString(strings.Repeat("-", 60) + "\n")
		for i, patch := range event.Diff {
			sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, patch.Op, patch.Path))
		}
	}

	return sb.String()
}

func (s *EmailSender) buildEmailMessage(subject, body string) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(s.config.To, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}
