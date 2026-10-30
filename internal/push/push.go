// Package push sends FCM notifications to a user's Android app via the
// Firebase Admin SDK.
package push

import (
	"context"
	"encoding/json"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Notifier struct {
	client *messaging.Client
}

// New returns (nil, nil) if serviceAccountJSON is empty, so callers can
// send notifications unconditionally — a nil *Notifier just no-ops.
func New(ctx context.Context, serviceAccountJSON string) (*Notifier, error) {
	if serviceAccountJSON == "" {
		return nil, nil
	}
	var credentials struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(serviceAccountJSON), &credentials); err != nil {
		return nil, fmt.Errorf("firebase credentials JSON: %w", err)
	}
	if credentials.ProjectID == "" {
		return nil, fmt.Errorf("firebase credentials JSON: project_id is required")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: credentials.ProjectID}, option.WithCredentialsJSON([]byte(serviceAccountJSON)))
	if err != nil {
		return nil, fmt.Errorf("firebase init: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase messaging client: %w", err)
	}
	return &Notifier{client: client}, nil
}

// Send is a no-op if n is nil or token is empty.
func (n *Notifier) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	if n == nil || token == "" {
		return nil
	}
	// No custom Android ChannelID here — that requires the channel to
	// already exist on the device, and firebase_messaging's Flutter
	// plugin auto-registers its own default channel for exactly this.
	_, err := n.client.Send(ctx, &messaging.Message{
		Token:        token,
		Notification: &messaging.Notification{Title: title, Body: body},
		Data:         data,
		Android:      &messaging.AndroidConfig{Priority: "high"},
	})
	return err
}
