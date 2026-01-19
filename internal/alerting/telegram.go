package alerting

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubechronicle/kubechronicle/internal/model"
)

// TelegramSender sends alerts to Telegram.
type TelegramSender struct {
	botToken string
	chatIDs  []string
	apiURL   string
	client   *http.Client
}

// NewTelegramSender creates a new Telegram alert sender.
func NewTelegramSender(cfg *TelegramConfig) (*TelegramSender, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}
	if len(cfg.ChatIDs) == 0 {
		return nil, fmt.Errorf("at least one chat ID is required")
	}

	return &TelegramSender{
		botToken: cfg.BotToken,
		chatIDs:  cfg.ChatIDs,
		apiURL:   "https://api.telegram.org/bot",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Name returns the sender name.
func (s *TelegramSender) Name() string {
	return "telegram"
}

// Send sends an alert to Telegram.
func (s *TelegramSender) Send(event *model.ChangeEvent) error {
	message := formatTelegramMessage(event)

	// Send to all configured chat IDs
	for _, chatID := range s.chatIDs {
		if err := s.sendToChat(chatID, message); err != nil {
			return fmt.Errorf("failed to send to chat %s: %w", chatID, err)
		}
	}

	return nil
}

func (s *TelegramSender) sendToChat(chatID, message string) error {
	endpoint := fmt.Sprintf("%s%s/sendMessage", s.apiURL, s.botToken)

	data := url.Values{}
	data.Set("chat_id", chatID)
	data.Set("text", message)
	data.Set("parse_mode", "HTML") // Use HTML for basic formatting

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func formatTelegramMessage(event *model.ChangeEvent) string {
	var sb strings.Builder

	// Emoji based on operation
	emoji := "✅"
	if event.Operation == "UPDATE" {
		emoji = "🔄"
	} else if event.Operation == "DELETE" {
		emoji = "❌"
	}

	sb.WriteString(fmt.Sprintf("<b>%s Kubernetes Resource %s</b>\n\n", emoji, event.Operation))
	sb.WriteString(fmt.Sprintf("<b>Resource:</b> %s/%s\n", event.ResourceKind, event.Name))
	sb.WriteString(fmt.Sprintf("<b>Namespace:</b> %s\n", event.Namespace))
	sb.WriteString(fmt.Sprintf("<b>User:</b> %s\n", event.Actor.Username))
	sb.WriteString(fmt.Sprintf("<b>Tool:</b> %s\n", event.Source.Tool))

	if event.Actor.ServiceAccount != "" {
		sb.WriteString(fmt.Sprintf("<b>Service Account:</b> %s\n", event.Actor.ServiceAccount))
	}

	if event.Actor.SourceIP != "" {
		sb.WriteString(fmt.Sprintf("<b>Source IP:</b> %s\n", event.Actor.SourceIP))
	}

	sb.WriteString(fmt.Sprintf("\n<b>Time:</b> %s\n", event.Timestamp.Format(time.RFC3339)))

	if len(event.Diff) > 0 {
		sb.WriteString(fmt.Sprintf("\n<b>Changes:</b> %d patch operation(s)\n", len(event.Diff)))
	}

	return sb.String()
}
