package sms

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Sender struct {
	apiKey   string
	senderID string
	client   *http.Client
}

func New(apiKey, senderID string) *Sender {
	return &Sender{
		apiKey:   apiKey,
		senderID: senderID,
		client:   &http.Client{},
	}
}

func (s *Sender) SendText(to, message string) error {
	data := url.Values{
		"api_key": {s.apiKey},
		"senderid": {s.senderID},
		"number":   {to},
		"message":  {message},
	}

	req, err := http.NewRequest(http.MethodPost, "https://bulksmsbd.net/api/smsapi", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
