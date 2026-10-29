package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const unitCols = `id, property_id, unit_number, base_rent, is_occupied, default_utilities, rent_history,
	deposit_history, floor_level, unit_size, unit_type, security_deposit, status, meters, image_url, video_url`

func scanUnit(row pgx.Row) (*models.Unit, error) {
	var u models.Unit
	var defaultUtilities, rentHistory, depositHistory, meters []byte

	if err := row.Scan(&u.ID, &u.PropertyID, &u.UnitNumber, &u.BaseRent, &u.IsOccupied,
		&defaultUtilities, &rentHistory, &depositHistory,
		&u.FloorLevel, &u.UnitSize, &u.UnitType, &u.SecurityDeposit, &u.Status, &meters,
		&u.ImageURL, &u.VideoURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	u.DefaultUtilities = map[string]int{}
	u.Meters = map[string]string{}
	_ = json.Unmarshal(defaultUtilities, &u.DefaultUtilities)
	_ = json.Unmarshal(rentHistory, &u.RentHistory)
	_ = json.Unmarshal(depositHistory, &u.DepositHistory)
	_ = json.Unmarshal(meters, &u.Meters)
	return &u, nil
}

func normalizeUnit(u *models.Unit) {
	if u.DefaultUtilities == nil {
		u.DefaultUtilities = map[string]int{}
	}
	if u.Meters == nil {
		u.Meters = map[string]string{}
	}
	if u.RentHistory == nil {
		u.RentHistory = []models.RentConfig{}
	}
	if u.DepositHistory == nil {
		u.DepositHistory = []models.DepositConfig{}
	}
}

func (s *Store) CreateUnit(ctx context.Context, u models.Unit) (*models.Unit, error) {
	normalizeUnit(&u)
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO units (property_id, unit_number, base_rent, is_occupied, default_utilities, rent_history,
			deposit_history, floor_level, unit_size, unit_type, security_deposit, status, meters, image_url, video_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING `+unitCols,
		u.PropertyID, u.UnitNumber, u.BaseRent, u.IsOccupied, toJSON(u.DefaultUtilities), toJSON(u.RentHistory),
		toJSON(u.DepositHistory), u.FloorLevel, u.UnitSize, u.UnitType, u.SecurityDeposit,
		orStatus(u.Status), toJSON(u.Meters), u.ImageURL, u.VideoURL)
	return scanUnit(row)
}

func (s *Store) GetUnit(ctx context.Context, id string) (*models.Unit, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+unitCols+` FROM units WHERE id = $1`, id)
	return scanUnit(row)
}

func (s *Store) ListUnitsByProperty(ctx context.Context, propertyID string) ([]models.Unit, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+unitCols+` FROM units WHERE property_id = $1 ORDER BY unit_number`, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}
	defer rows.Close()

	out := []models.Unit{}
	for rows.Next() {
		u, err := scanUnit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, nil
}

func (s *Store) UpdateUnit(ctx context.Context, u models.Unit) (*models.Unit, error) {
	normalizeUnit(&u)
	row := s.Pool.QueryRow(ctx, `
		UPDATE units SET unit_number=$2, base_rent=$3, is_occupied=$4, default_utilities=$5, rent_history=$6,
			deposit_history=$7, floor_level=$8, unit_size=$9, unit_type=$10, security_deposit=$11, status=$12,
			meters=$13, image_url=$14, video_url=$15
		WHERE id = $1
		RETURNING `+unitCols,
		u.ID, u.UnitNumber, u.BaseRent, u.IsOccupied, toJSON(u.DefaultUtilities), toJSON(u.RentHistory),
		toJSON(u.DepositHistory), u.FloorLevel, u.UnitSize, u.UnitType, u.SecurityDeposit,
		orStatus(u.Status), toJSON(u.Meters), u.ImageURL, u.VideoURL)
	return scanUnit(row)
}

func (s *Store) SetUnitOccupancy(ctx context.Context, unitID string, occupied bool, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE units SET is_occupied = $2, status = $3 WHERE id = $1`, unitID, occupied, status)
	return err
}

func (s *Store) DeleteUnit(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM units WHERE id = $1`, id)
	return err
}

func orStatus(s string) string {
	if s == "" {
		return "vacant"
	}
	return s
}
