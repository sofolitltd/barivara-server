package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Sender struct {
	apiKey   string
	senderID string
	client   *http.Client
}

type Balance struct {
	Balance      float64 `json:"balance"`
	ExpiryDate   string  `json:"expiryDate,omitempty"`
	ProviderCode int     `json:"providerCode,omitempty"`
}

func (s *Sender) GetBalance() (Balance, error) {
	endpoint := fmt.Sprintf("https://bulksmsbd.net/api/getBalanceApi?api_key=%s&response=json", url.QueryEscape(s.apiKey))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Balance{}, fmt.Errorf("create balance request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return Balance{}, fmt.Errorf("balance request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Balance{}, fmt.Errorf("read balance response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Balance{}, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return Balance{}, fmt.Errorf("decode balance response: %w", err)
	}
	result := findBalanceValue(payload)
	if result.providerCode != 0 && result.providerCode != 1000 && result.providerCode != 202 {
		return Balance{}, fmt.Errorf("provider error (code %d): %s", result.providerCode, result.message)
	}
	if result.balanceSet {
		return Balance{Balance: result.balance, ExpiryDate: result.expiry, ProviderCode: result.providerCode}, nil
	}
	return Balance{}, fmt.Errorf("provider balance not found in response: %s", strings.TrimSpace(string(body)))
}

type balanceResponse struct {
	balance      float64
	balanceSet   bool
	expiry       string
	message      string
	providerCode int
}

func findBalanceValue(value any) balanceResponse {
	var result balanceResponse
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			switch normalized {
			case "responsecode", "statuscode", "code":
				if code, ok := parseInt(child); ok {
					result.providerCode = code
				}
			case "errormessage", "message", "status":
				if text, ok := child.(string); ok && result.message == "" {
					result.message = text
				}
			case "expirydate", "expirationdate", "expiredate", "validity", "validtill":
				if text, ok := stringValue(child); ok {
					result.expiry = text
				}
			case "smsbalance", "balance", "availablebalance", "availablesms", "smscredit", "credit":
				if balance, ok := parseFloat(child); ok {
					result.balance, result.balanceSet = balance, true
				}
			}
		}
		for _, child := range typed {
			if _, nested := child.(map[string]any); !nested {
				continue
			}
			if nestedResult := findBalanceValue(child); nestedResult.balanceSet || nestedResult.providerCode != 0 || nestedResult.expiry != "" {
				if nestedResult.balanceSet {
					result.balance, result.balanceSet = nestedResult.balance, true
				}
				if result.providerCode == 0 {
					result.providerCode = nestedResult.providerCode
				}
				if result.expiry == "" {
					result.expiry = nestedResult.expiry
				}
			}
		}
	case []any:
		for _, child := range typed {
			if nestedResult := findBalanceValue(child); nestedResult.balanceSet || nestedResult.providerCode != 0 || nestedResult.expiry != "" {
				if nestedResult.balanceSet {
					result.balance, result.balanceSet = nestedResult.balance, true
				}
				if result.providerCode == 0 {
					result.providerCode = nestedResult.providerCode
				}
				if result.expiry == "" {
					result.expiry = nestedResult.expiry
				}
			}
		}
	}
	return result
}

func parseInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	}
	return 0, false
}

func parseFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	}
	return 0, false
}

func stringValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), typed != ""
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	}
	return "", false
}

func New(apiKey, senderID string) *Sender {
	if apiKey == "" || senderID == "" {
		return nil
	}
	return &Sender{
		apiKey:   apiKey,
		senderID: senderID,
		client:   &http.Client{},
	}
}

func (s *Sender) SendText(to, message string) error {
	if s == nil {
		return fmt.Errorf("SMS service is not configured")
	}
	data := url.Values{
		"api_key":  {s.apiKey},
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
