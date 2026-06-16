package handler

import (
	"github.com/sofolitltd/barivara-server/internal/firebase"
	"github.com/sofolitltd/barivara-server/internal/sms"
)

type Handler struct {
	FB     *firebase.Client
	Sender *sms.Sender
}

func New(fb *firebase.Client, sender *sms.Sender) *Handler {
	return &Handler{FB: fb, Sender: sender}
}
