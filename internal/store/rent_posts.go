package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

const rentPostCols = `id, unit_id, property_id, landlord_id, title, description, location, rent_amount,
	image_urls, created_at, is_active, property_type, beds, baths, area_sqft`

func scanRentPost(row pgx.Row) (*models.RentPost, error) {
	var p models.RentPost
	var imageURLs []byte
	if err := row.Scan(&p.ID, &p.UnitID, &p.PropertyID, &p.LandlordID, &p.Title, &p.Description,
		&p.Location, &p.RentAmount, &imageURLs, &p.CreatedAt, &p.IsActive, &p.PropertyType,
		&p.Beds, &p.Baths, &p.AreaSqft); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.ImageURLs = []string{}
	_ = json.Unmarshal(imageURLs, &p.ImageURLs)
	return &p, nil
}

func (s *Store) CreateRentPost(ctx context.Context, p models.RentPost) (*models.RentPost, error) {
	if p.ImageURLs == nil {
		p.ImageURLs = []string{}
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO rent_posts (unit_id, property_id, landlord_id, title, description, location, rent_amount,
			image_urls, is_active, property_type, beds, baths, area_sqft)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING `+rentPostCols,
		p.UnitID, p.PropertyID, p.LandlordID, p.Title, p.Description, p.Location, p.RentAmount,
		toJSON(p.ImageURLs), true, p.PropertyType, p.Beds, p.Baths, p.AreaSqft)
	return scanRentPost(row)
}

func (s *Store) GetRentPost(ctx context.Context, id string) (*models.RentPost, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+rentPostCols+` FROM rent_posts WHERE id = $1`, id)
	return scanRentPost(row)
}

func (s *Store) ListActiveRentPosts(ctx context.Context) ([]models.RentPost, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+rentPostCols+` FROM rent_posts WHERE is_active = true ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list active rent posts: %w", err)
	}
	defer rows.Close()

	var out []models.RentPost
	for rows.Next() {
		p, err := scanRentPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func (s *Store) GetActiveRentPostForUnit(ctx context.Context, unitID string) (*models.RentPost, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+rentPostCols+` FROM rent_posts WHERE unit_id = $1 AND is_active = true LIMIT 1`, unitID)
	return scanRentPost(row)
}

func (s *Store) UpdateRentPost(ctx context.Context, p models.RentPost) (*models.RentPost, error) {
	if p.ImageURLs == nil {
		p.ImageURLs = []string{}
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE rent_posts SET title=$2, description=$3, location=$4, rent_amount=$5, image_urls=$6,
			is_active=$7, property_type=$8, beds=$9, baths=$10, area_sqft=$11
		WHERE id = $1
		RETURNING `+rentPostCols,
		p.ID, p.Title, p.Description, p.Location, p.RentAmount, toJSON(p.ImageURLs),
		p.IsActive, p.PropertyType, p.Beds, p.Baths, p.AreaSqft)
	return scanRentPost(row)
}

func (s *Store) DeleteRentPost(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM rent_posts WHERE id = $1`, id)
	return err
}
