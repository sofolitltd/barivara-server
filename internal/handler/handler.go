package handler

import (
	"github.com/sofolitltd/barivara-server/internal/auth"
	"github.com/sofolitltd/barivara-server/internal/push"
	"github.com/sofolitltd/barivara-server/internal/sms"
	"github.com/sofolitltd/barivara-server/internal/storage"
	"github.com/sofolitltd/barivara-server/internal/store"
	"github.com/sofolitltd/barivara-server/internal/telegram"
)

type Handler struct {
	Store    *store.Store
	Sender   *sms.Sender
	Tokens   *auth.TokenIssuer
	Storage  *storage.Client
	Telegram *telegram.Notifier
	Push     *push.Notifier
}

func New(st *store.Store, sender *sms.Sender, tokens *auth.TokenIssuer, storageClient *storage.Client, telegramNotifier *telegram.Notifier, pushNotifier *push.Notifier) *Handler {
	return &Handler{Store: st, Sender: sender, Tokens: tokens, Storage: storageClient, Telegram: telegramNotifier, Push: pushNotifier}
}
