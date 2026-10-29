package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const landlordReqCols = `id, user_id, user_name, phone, nid_number, coalesce(owner_proof_image_url, ''), status, created_at`

func scanLandlordRequest(row pgx.Row) (*models.LandlordRequest, error) {
	var r models.LandlordRequest
	if err := row.Scan(&r.ID, &r.UserID, &r.UserName, &r.Phone, &r.NidNumber, &r.OwnerProofImageURL, &r.Status, &r.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateLandlordRequest(ctx context.Context, r models.LandlordRequest) (*models.LandlordRequest, error) {
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO landlord_requests (user_id, user_name, phone, nid_number, owner_proof_image_url)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+landlordReqCols,
		r.UserID, r.UserName, r.Phone, r.NidNumber, r.OwnerProofImageURL)
	return scanLandlordRequest(row)
}

func (s *Store) GetLandlordRequest(ctx context.Context, id string) (*models.LandlordRequest, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+landlordReqCols+` FROM landlord_requests WHERE id = $1`, id)
	return scanLandlordRequest(row)
}

func (s *Store) ListPendingLandlordRequests(ctx context.Context) ([]models.LandlordRequest, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+landlordReqCols+` FROM landlord_requests WHERE status = 'pending' ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list pending landlord requests: %w", err)
	}
	defer rows.Close()

	var out []models.LandlordRequest
	for rows.Next() {
		r, err := scanLandlordRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

// ListAllLandlordRequests returns every landlord application regardless of
// its approve/reject/pending status — the admin UI filters by status
// client-side instead of hitting the DB once per tab.
func (s *Store) ListAllLandlordRequests(ctx context.Context) ([]models.LandlordRequest, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+landlordReqCols+` FROM landlord_requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all landlord requests: %w", err)
	}
	defer rows.Close()

	var out []models.LandlordRequest
	for rows.Next() {
		r, err := scanLandlordRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) SetLandlordRequestStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE landlord_requests SET status = $2 WHERE id = $1`, id, status)
	return err
}

func (s *Store) CountPendingLandlordRequests(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM landlord_requests WHERE status = 'pending'`).Scan(&n)
	return n, err
}
