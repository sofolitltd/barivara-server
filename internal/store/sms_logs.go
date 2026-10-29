package store

import (
	"context"
	"time"
)

type SMSLog struct {
	RenterID  string
	InvoiceID string
	Type      string
	Phone     string
	Message   string
	Status    string
	Error     string
	SentAt    time.Time
}

func (s *Store) LogSMS(ctx context.Context, log SMSLog) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sms_logs (renter_id, invoice_id, type, phone, message, status, error, sent_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, nullIfEmpty(log.RenterID), nullIfEmpty(log.InvoiceID), log.Type, log.Phone, log.Message,
		log.Status, nullIfEmpty(log.Error), log.SentAt)
	return err
}
