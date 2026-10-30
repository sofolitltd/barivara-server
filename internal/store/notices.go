package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

type NoticeRecipient struct {
	UserID string
	Token  string
}

func (s *Store) CreateNotice(ctx context.Context, landlordID, propertyID, title, body, priority string, expiresAt *time.Time) (*models.Notice, []NoticeRecipient, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var notice models.Notice
	err = tx.QueryRow(ctx, `
		INSERT INTO notices (landlord_id, property_id, title, body, priority, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, landlord_id, property_id, title, body, priority, published_at, expires_at, created_at`,
		landlordID, propertyID, title, body, priority, expiresAt,
	).Scan(&notice.ID, &notice.LandlordID, &notice.PropertyID, &notice.Title, &notice.Body, &notice.Priority,
		&notice.PublishedAt, &notice.ExpiresAt, &notice.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	rows, err := tx.Query(ctx, `
		SELECT r.id, r.linked_user_id, coalesce(u.fcm_token, '')
		FROM renters r
		JOIN users u ON u.id = r.linked_user_id
		WHERE r.property_id = $1 AND r.is_active = true AND r.linked_user_id IS NOT NULL`,
		propertyID)
	if err != nil {
		return nil, nil, err
	}
	recipients := []NoticeRecipient{}
	type recipientRow struct {
		renterID string
		userID   string
		token    string
	}
	recipientRows := []recipientRow{}
	for rows.Next() {
		var row recipientRow
		if err := rows.Scan(&row.renterID, &row.userID, &row.token); err != nil {
			rows.Close()
			return nil, nil, err
		}
		recipientRows = append(recipientRows, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	for _, row := range recipientRows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO notice_recipients (notice_id, renter_id, tenant_user_id)
			VALUES ($1,$2,$3)
			ON CONFLICT (notice_id, tenant_user_id) DO NOTHING`,
			notice.ID, row.renterID, row.userID); err != nil {
			return nil, nil, err
		}
		recipients = append(recipients, NoticeRecipient{UserID: row.userID, Token: row.token})
	}
	notice.RecipientCount = len(recipients)

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &notice, recipients, nil
}

func (s *Store) ListNoticesByProperty(ctx context.Context, propertyID string) ([]models.Notice, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT n.id, n.landlord_id, n.property_id, n.title, n.body, n.priority,
		       n.published_at, n.expires_at, n.created_at,
		       (SELECT count(*) FROM notice_recipients nr WHERE nr.notice_id = n.id)::int
		FROM notices n WHERE n.property_id = $1 ORDER BY n.published_at DESC`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Notice{}
	for rows.Next() {
		var n models.Notice
		if err := rows.Scan(&n.ID, &n.LandlordID, &n.PropertyID, &n.Title, &n.Body, &n.Priority,
			&n.PublishedAt, &n.ExpiresAt, &n.CreatedAt, &n.RecipientCount); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Store) ListNoticesForTenant(ctx context.Context, userID string) ([]models.Notice, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT n.id, n.landlord_id, n.property_id, coalesce(p.name, ''), n.title, n.body,
		       n.priority, n.published_at, n.expires_at, n.created_at, nr.read_at,
		       (SELECT count(*) FROM notice_recipients WHERE notice_id = n.id)::int
		FROM notices n
		JOIN notice_recipients nr ON nr.notice_id = n.id
		JOIN properties p ON p.id = n.property_id
		WHERE nr.tenant_user_id = $1
		  AND (n.expires_at IS NULL OR n.expires_at >= now())
		ORDER BY n.published_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Notice{}
	for rows.Next() {
		var n models.Notice
		if err := rows.Scan(&n.ID, &n.LandlordID, &n.PropertyID, &n.PropertyName, &n.Title, &n.Body,
			&n.Priority, &n.PublishedAt, &n.ExpiresAt, &n.CreatedAt, &n.ReadAt, &n.RecipientCount); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Store) MarkNoticeRead(ctx context.Context, userID, noticeID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE notice_recipients SET read_at = coalesce(read_at, now())
		WHERE notice_id = $1 AND tenant_user_id = $2`, noticeID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetNoticeForTenant(ctx context.Context, userID, noticeID string) (*models.Notice, error) {
	var n models.Notice
	err := s.Pool.QueryRow(ctx, `
		SELECT n.id, n.landlord_id, n.property_id, coalesce(p.name, ''), n.title, n.body,
		       n.priority, n.published_at, n.expires_at, n.created_at, nr.read_at,
		       (SELECT count(*) FROM notice_recipients WHERE notice_id = n.id)::int
		FROM notices n
		JOIN notice_recipients nr ON nr.notice_id = n.id
		JOIN properties p ON p.id = n.property_id
		WHERE n.id = $1 AND nr.tenant_user_id = $2`, noticeID, userID,
	).Scan(&n.ID, &n.LandlordID, &n.PropertyID, &n.PropertyName, &n.Title, &n.Body,
		&n.Priority, &n.PublishedAt, &n.ExpiresAt, &n.CreatedAt, &n.ReadAt, &n.RecipientCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get notice: %w", err)
	}
	return &n, nil
}
