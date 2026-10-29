// Package telegram sends admin notifications via the Telegram Bot API
// (https://core.telegram.org/bots/api#sendmessage).
package telegram

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Notifier struct {
	botToken string
	chatID   string
	client   *http.Client
}

// New returns nil if botToken or chatID is empty, so callers can send
// notifications unconditionally — a nil *Notifier just no-ops.
func New(botToken, chatID string) *Notifier {
	if botToken == "" || chatID == "" {
		return nil
	}
	return &Notifier{botToken: botToken, chatID: chatID, client: &http.Client{}}
}

func (n *Notifier) SendMessage(text string) error {
	if n == nil {
		return nil
	}

	data := url.Values{
		"chat_id":    {n.chatID},
		"text":       {text},
		"parse_mode": {"HTML"},
	}

	resp, err := n.client.PostForm(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken),
		data,
	)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
