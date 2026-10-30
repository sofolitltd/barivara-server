package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const renterCols = `id, property_id, unit_id, name, phone, nid, advance_deposit, move_in_date, email,
	occupation, alternate_phone, photo_url, move_out_date, is_active, family_members, documents,
	sms_enabled, landlord_notes, join_code, linked_user_id, rent_history, deposit_history`

func scanRenter(row pgx.Row) (*models.Renter, error) {
	var r models.Renter
	var familyMembers, documents, rentHistory, depositHistory []byte

	if err := row.Scan(&r.ID, &r.PropertyID, &r.UnitID, &r.Name, &r.Phone, &r.Nid, &r.AdvanceDeposit,
		&r.MoveInDate, &r.Email, &r.Occupation, &r.AlternatePhone, &r.PhotoURL, &r.MoveOutDate,
		&r.IsActive, &familyMembers, &documents, &r.SmsEnabled, &r.LandlordNotes, &r.JoinCode, &r.LinkedUserID,
		&rentHistory, &depositHistory); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	r.FamilyMembers = []models.FamilyMember{}
	r.Documents = []models.RenterDocument{}
	_ = json.Unmarshal(familyMembers, &r.FamilyMembers)
	_ = json.Unmarshal(documents, &r.Documents)
	_ = json.Unmarshal(rentHistory, &r.RentHistory)
	_ = json.Unmarshal(depositHistory, &r.DepositHistory)
	if r.RentHistory == nil {
		r.RentHistory = []models.TenantRentConfig{}
	}
	if r.DepositHistory == nil {
		r.DepositHistory = []models.TenantDepositConfig{}
	}
	return &r, nil
}

func normalizeRenter(r *models.Renter) {
	if r.FamilyMembers == nil {
		r.FamilyMembers = []models.FamilyMember{}
	}
	if r.Documents == nil {
		r.Documents = []models.RenterDocument{}
	}
	if r.RentHistory == nil {
		r.RentHistory = []models.TenantRentConfig{}
	}
	if r.DepositHistory == nil {
		r.DepositHistory = []models.TenantDepositConfig{}
	}
}

func (s *Store) CreateRenter(ctx context.Context, r models.Renter) (*models.Renter, error) {
	normalizeRenter(&r)
	var unitRent int
	var unitDeposit *int
	if err := s.Pool.QueryRow(ctx, `SELECT base_rent, security_deposit FROM units WHERE id=$1`, r.UnitID).Scan(&unitRent, &unitDeposit); err != nil {
		return nil, fmt.Errorf("load unit rental terms: %w", err)
	}
	if len(r.RentHistory) == 0 {
		r.RentHistory = []models.TenantRentConfig{{Amount: unitRent, StartDate: r.MoveInDate}}
	}
	if len(r.DepositHistory) == 0 && unitDeposit != nil {
		r.DepositHistory = []models.TenantDepositConfig{{Amount: *unitDeposit, StartDate: r.MoveInDate}}
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO renters (property_id, unit_id, name, phone, nid, advance_deposit, move_in_date, email,
			occupation, alternate_phone, photo_url, is_active, family_members, documents, sms_enabled, landlord_notes,
			join_code, rent_history, deposit_history)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true,$12,$13,$14,$15,$16,$17,$18)
		RETURNING `+renterCols,
		r.PropertyID, r.UnitID, r.Name, r.Phone, r.Nid, r.AdvanceDeposit, r.MoveInDate, r.Email,
		r.Occupation, r.AlternatePhone, r.PhotoURL, toJSON(r.FamilyMembers), toJSON(r.Documents), r.SmsEnabled,
		r.LandlordNotes, r.JoinCode, toJSON(r.RentHistory), toJSON(r.DepositHistory))
	renter, err := scanRenter(row)
	if err != nil {
		return nil, err
	}

	if _, err := s.Pool.Exec(ctx, `UPDATE units SET is_occupied = true, status = 'occupied' WHERE id = $1`, r.UnitID); err != nil {
		return nil, fmt.Errorf("mark unit occupied: %w", err)
	}
	return renter, nil
}

func (s *Store) GetRenter(ctx context.Context, id string) (*models.Renter, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+renterCols+` FROM renters WHERE id = $1`, id)
	return scanRenter(row)
}

func (s *Store) ListActiveRentersByUnit(ctx context.Context, propertyID, unitID string) ([]models.Renter, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+renterCols+` FROM renters WHERE property_id=$1 AND unit_id=$2 AND is_active=true`, propertyID, unitID)
	if err != nil {
		return nil, fmt.Errorf("list active renters: %w", err)
	}
	defer rows.Close()
	return collectRenters(rows)
}

func (s *Store) ListActiveRentersByProperty(ctx context.Context, propertyID string) ([]models.Renter, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+renterCols+` FROM renters WHERE property_id=$1 AND is_active=true`, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list active renters by property: %w", err)
	}
	defer rows.Close()
	return collectRenters(rows)
}

func (s *Store) ListRenterHistory(ctx context.Context, propertyID, unitID string) ([]models.Renter, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+renterCols+` FROM renters WHERE property_id=$1 AND unit_id=$2 AND is_active=false`, propertyID, unitID)
	if err != nil {
		return nil, fmt.Errorf("list renter history: %w", err)
	}
	defer rows.Close()
	return collectRenters(rows)
}

func collectRenters(rows pgx.Rows) ([]models.Renter, error) {
	out := []models.Renter{}
	for rows.Next() {
		r, err := scanRenter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) UpdateRenter(ctx context.Context, r models.Renter) (*models.Renter, error) {
	normalizeRenter(&r)
	row := s.Pool.QueryRow(ctx, `
		UPDATE renters SET name=$2, phone=$3, nid=$4, advance_deposit=$5, move_in_date=$6, email=$7,
			occupation=$8, alternate_phone=$9, photo_url=$10, move_out_date=$11, is_active=$12,
			family_members=$13, documents=$14, sms_enabled=$15, landlord_notes=$16, rent_history=$17, deposit_history=$18
		WHERE id = $1
		RETURNING `+renterCols,
		r.ID, r.Name, r.Phone, r.Nid, r.AdvanceDeposit, r.MoveInDate, r.Email, r.Occupation,
		r.AlternatePhone, r.PhotoURL, r.MoveOutDate, r.IsActive, toJSON(r.FamilyMembers), toJSON(r.Documents),
		r.SmsEnabled, r.LandlordNotes, toJSON(r.RentHistory), toJSON(r.DepositHistory))
	return scanRenter(row)
}

func (s *Store) ScheduleRentChange(ctx context.Context, renterID string, amount int, startDate time.Time, reason *string) (*models.Renter, error) {
	renter, err := s.GetRenter(ctx, renterID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, fmt.Errorf("rent amount must be greater than zero")
	}
	renter.RentHistory = append(renter.RentHistory, models.TenantRentConfig{
		Amount: amount, StartDate: startDate, Reason: reason,
	})
	sort.Slice(renter.RentHistory, func(i, j int) bool {
		return renter.RentHistory[i].StartDate.Before(renter.RentHistory[j].StartDate)
	})
	row := s.Pool.QueryRow(ctx, `
		UPDATE renters SET rent_history=$2 WHERE id=$1 RETURNING `+renterCols,
		renterID, toJSON(renter.RentHistory))
	return scanRenter(row)
}

func (s *Store) VacateRenter(ctx context.Context, unitID, renterID string, moveOutDate interface{}) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE renters SET is_active=false, move_out_date=$2 WHERE id=$1`, renterID, moveOutDate); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE units SET is_occupied=false, status='vacant' WHERE id=$1`, unitID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) RestoreRenter(ctx context.Context, unitID, renterID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE renters SET is_active=true, move_out_date=NULL WHERE id=$1`, renterID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE units SET is_occupied=true, status='occupied' WHERE id=$1`, unitID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteRenter(ctx context.Context, unitID, renterID string, wasActive bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM invoices WHERE renter_id = $1`, renterID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM renters WHERE id = $1`, renterID); err != nil {
		return err
	}
	if wasActive {
		if _, err := tx.Exec(ctx, `UPDATE units SET is_occupied=false, status='vacant' WHERE id=$1`, unitID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// --- Tenant linking ---

func (s *Store) GenerateJoinCode(ctx context.Context, renterID, code string) (string, error) {
	var existing *string
	if err := s.Pool.QueryRow(ctx, `SELECT join_code FROM renters WHERE id = $1`, renterID).Scan(&existing); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE renters SET join_code = $2 WHERE id = $1`, renterID, code); err != nil {
		return "", err
	}
	return code, nil
}

func (s *Store) GetActiveRenterByJoinCode(ctx context.Context, code string) (*models.Renter, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+renterCols+` FROM renters WHERE join_code = $1 AND is_active = true`, code)
	return scanRenter(row)
}

func (s *Store) LinkRenterToUser(ctx context.Context, renterID, userID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE renters SET linked_user_id = $2 WHERE id = $1`, renterID, userID)
	return err
}

func (s *Store) UnlinkRenter(ctx context.Context, renterID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE renters SET linked_user_id = NULL WHERE id = $1`, renterID)
	return err
}

func (s *Store) GetLinkedRenter(ctx context.Context, userID string) (*models.Renter, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+renterCols+` FROM renters WHERE linked_user_id = $1 AND is_active = true LIMIT 1`, userID)
	return scanRenter(row)
}
