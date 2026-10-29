package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sofolitltd/barivara-server/internal/models"
)

func scanProperty(row pgx.Row) (*models.Property, error) {
	var p models.Property
	var imageURLs []byte
	if err := row.Scan(&p.ID, &p.OwnerID, &p.Name, &p.Address, &p.ReminderDay, &p.ReminderHour,
		&p.SmsEnabled, &p.GoogleMapsURL, &p.PropertyType, &p.ImageURL, &imageURLs, &p.VideoURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.ImageURLs = []string{}
	_ = json.Unmarshal(imageURLs, &p.ImageURLs)
	return &p, nil
}

const propertyCols = `id, owner_id, name, address, reminder_day, reminder_hour, sms_enabled, google_maps_url, property_type, image_url, image_urls, video_url`

func (s *Store) CreateProperty(ctx context.Context, ownerID string, p models.Property) (*models.Property, error) {
	if p.ImageURLs == nil {
		p.ImageURLs = []string{}
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO properties (owner_id, name, address, reminder_day, reminder_hour, sms_enabled, google_maps_url, property_type, image_url, image_urls, video_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+propertyCols,
		ownerID, p.Name, p.Address, orDefault(p.ReminderDay, 5), orDefault(p.ReminderHour, 8), p.SmsEnabled,
		p.GoogleMapsURL, p.PropertyType, p.ImageURL, toJSON(p.ImageURLs), p.VideoURL)
	return scanProperty(row)
}

func (s *Store) GetProperty(ctx context.Context, id string) (*models.Property, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+propertyCols+` FROM properties WHERE id = $1`, id)
	return scanProperty(row)
}

func (s *Store) ListPropertiesByOwner(ctx context.Context, ownerID string) ([]models.Property, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+propertyCols+` FROM properties WHERE owner_id = $1 ORDER BY name`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}
	defer rows.Close()

	out := []models.Property{}
	for rows.Next() {
		p, err := scanProperty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func (s *Store) UpdateProperty(ctx context.Context, p models.Property) (*models.Property, error) {
	if p.ImageURLs == nil {
		p.ImageURLs = []string{}
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE properties SET name=$2, address=$3, reminder_day=$4, reminder_hour=$5, sms_enabled=$6,
			google_maps_url=$7, property_type=$8, image_url=$9, image_urls=$10, video_url=$11
		WHERE id = $1
		RETURNING `+propertyCols,
		p.ID, p.Name, p.Address, p.ReminderDay, p.ReminderHour, p.SmsEnabled,
		p.GoogleMapsURL, p.PropertyType, p.ImageURL, toJSON(p.ImageURLs), p.VideoURL)
	return scanProperty(row)
}

func (s *Store) DeleteProperty(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM properties WHERE id = $1`, id)
	return err
}

func (s *Store) QueryPropertiesByReminderDay(ctx context.Context, day int) ([]models.Property, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+propertyCols+` FROM properties WHERE reminder_day = $1`, day)
	if err != nil {
		return nil, fmt.Errorf("query properties by reminder day: %w", err)
	}
	defer rows.Close()

	out := []models.Property{}
	for rows.Next() {
		p, err := scanProperty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
