package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const tenantLinkCols = `id, renter_id, property_id, unit_id, tenant_user_id, tenant_name, coalesce(tenant_email, ''), tenant_phone, status, created_at`

func scanTenantLinkRequest(row pgx.Row) (*models.TenantLinkRequest, error) {
	var r models.TenantLinkRequest
	if err := row.Scan(&r.ID, &r.RenterID, &r.PropertyID, &r.UnitID, &r.TenantUserID, &r.TenantName,
		&r.TenantEmail, &r.TenantPhone, &r.Status, &r.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) CreateTenantLinkRequest(ctx context.Context, r models.TenantLinkRequest) (*models.TenantLinkRequest, error) {
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO tenant_link_requests (renter_id, property_id, unit_id, tenant_user_id, tenant_name, tenant_email, tenant_phone)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+tenantLinkCols,
		r.RenterID, r.PropertyID, r.UnitID, r.TenantUserID, r.TenantName, r.TenantEmail, r.TenantPhone)
	return scanTenantLinkRequest(row)
}

func (s *Store) GetTenantLinkRequest(ctx context.Context, id string) (*models.TenantLinkRequest, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+tenantLinkCols+` FROM tenant_link_requests WHERE id = $1`, id)
	return scanTenantLinkRequest(row)
}

func (s *Store) ListPendingLinkRequestsForRenter(ctx context.Context, renterID string) ([]models.TenantLinkRequest, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+tenantLinkCols+` FROM tenant_link_requests WHERE renter_id = $1 AND status = 'pending'`, renterID)
	if err != nil {
		return nil, fmt.Errorf("list pending link requests: %w", err)
	}
	defer rows.Close()

	var out []models.TenantLinkRequest
	for rows.Next() {
		r, err := scanTenantLinkRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) GetMyPendingLinkRequest(ctx context.Context, tenantUserID string) (*models.TenantLinkRequest, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+tenantLinkCols+` FROM tenant_link_requests WHERE tenant_user_id = $1 AND status = 'pending' LIMIT 1`, tenantUserID)
	return scanTenantLinkRequest(row)
}

func (s *Store) SetLinkRequestStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE tenant_link_requests SET status = $2 WHERE id = $1`, id, status)
	return err
}
