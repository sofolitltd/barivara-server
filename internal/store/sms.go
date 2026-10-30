package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

func scanSMSPackage(row pgx.Row) (*models.SMSPackage, error) {
	var p models.SMSPackage
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.MessageCount, &p.Price, &p.IsActive, &p.CreatedAt)
	return &p, err
}

func collectSMSPackages(rows pgx.Rows) ([]models.SMSPackage, error) {
	out := []models.SMSPackage{}
	for rows.Next() {
		p, err := scanSMSPackage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) ListSMSPackages(ctx context.Context, activeOnly bool) ([]models.SMSPackage, error) {
	query := `SELECT id, name, description, message_count, price, is_active, created_at FROM sms_packages`
	if activeOnly {
		query += ` WHERE is_active=true`
	}
	query += ` ORDER BY price, message_count`
	rows, err := s.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSMSPackages(rows)
}

func (s *Store) CreateSMSPackage(ctx context.Context, p models.SMSPackage) (*models.SMSPackage, error) {
	row := s.Pool.QueryRow(ctx, `INSERT INTO sms_packages (name, description, message_count, price, is_active)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, name, description, message_count, price, is_active, created_at`,
		p.Name, p.Description, p.MessageCount, p.Price, p.IsActive)
	return scanSMSPackage(row)
}

func (s *Store) UpdateSMSPackage(ctx context.Context, p models.SMSPackage) (*models.SMSPackage, error) {
	row := s.Pool.QueryRow(ctx, `UPDATE sms_packages SET name=$2, description=$3, message_count=$4, price=$5, is_active=$6
		WHERE id=$1 RETURNING id, name, description, message_count, price, is_active, created_at`,
		p.ID, p.Name, p.Description, p.MessageCount, p.Price, p.IsActive)
	return scanSMSPackage(row)
}

func (s *Store) DeleteSMSPackage(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM sms_packages WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SMSBalance(ctx context.Context, landlordID string) (int, error) {
	var balance int
	err := s.Pool.QueryRow(ctx, `SELECT balance FROM sms_wallets WHERE landlord_id=$1`, landlordID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return balance, err
}

func (s *Store) ConsumeSMS(ctx context.Context, landlordID string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE sms_wallets SET balance=balance-1, updated_at=now()
		WHERE landlord_id=$1 AND balance>0`, landlordID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insufficient SMS balance")
	}
	return nil
}

func (s *Store) RequestSMSPackage(ctx context.Context, landlordID, packageID, note string) (*models.SMSPurchaseRequest, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var name string
	var count, price int
	if err := tx.QueryRow(ctx, `SELECT name, message_count, price FROM sms_packages WHERE id=$1 AND is_active=true`, packageID).
		Scan(&name, &count, &price); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("package not found")
		}
		return nil, err
	}
	row := tx.QueryRow(ctx, `INSERT INTO sms_purchase_requests
		(landlord_id, package_id, package_name, message_count, amount, note)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, landlord_id, package_id, package_name, message_count, amount, note, status, created_at, processed_at`,
		landlordID, packageID, name, count, price, note)
	req, err := scanSMSPurchase(row)
	if err != nil {
		return nil, err
	}
	return req, tx.Commit(ctx)
}

func scanSMSPurchase(row pgx.Row) (*models.SMSPurchaseRequest, error) {
	var r models.SMSPurchaseRequest
	err := row.Scan(&r.ID, &r.LandlordID, &r.PackageID, &r.PackageName, &r.MessageCount, &r.Amount, &r.Note, &r.Status, &r.CreatedAt, &r.ProcessedAt)
	return &r, err
}

func (s *Store) ListSMSPurchaseRequests(ctx context.Context, landlordID string) ([]models.SMSPurchaseRequest, error) {
	query := `SELECT id, landlord_id, package_id, package_name, message_count, amount, note, status, created_at, processed_at FROM sms_purchase_requests`
	args := []any{}
	if landlordID != "" {
		query += ` WHERE landlord_id=$1`
		args = append(args, landlordID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SMSPurchaseRequest{}
	for rows.Next() {
		r, err := scanSMSPurchase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) ProcessSMSPurchase(ctx context.Context, requestID, status string) (*models.SMSPurchaseRequest, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var req models.SMSPurchaseRequest
	row := tx.QueryRow(ctx, `SELECT id, landlord_id, package_id, package_name, message_count, amount, note, status, created_at, processed_at
		FROM sms_purchase_requests WHERE id=$1 FOR UPDATE`, requestID)
	scanned, err := scanSMSPurchase(row)
	if err != nil {
		return nil, err
	}
	req = *scanned
	if req.Status != "pending" {
		return nil, fmt.Errorf("purchase request already processed")
	}
	if status == "approved" {
		_, err = tx.Exec(ctx, `INSERT INTO sms_wallets (landlord_id, balance)
			VALUES ($1,$2) ON CONFLICT (landlord_id) DO UPDATE SET balance=sms_wallets.balance+$2, updated_at=now()`,
			req.LandlordID, req.MessageCount)
		if err != nil {
			return nil, err
		}
	}
	row = tx.QueryRow(ctx, `UPDATE sms_purchase_requests SET status=$2, processed_at=now() WHERE id=$1
		RETURNING id, landlord_id, package_id, package_name, message_count, amount, note, status, created_at, processed_at`,
		requestID, status)
	updated, err := scanSMSPurchase(row)
	if err != nil {
		return nil, err
	}
	return updated, tx.Commit(ctx)
}
