package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sofolitltd/barivara-server/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicate = errors.New("already exists")

type UserWithPassword struct {
	models.User
	PasswordHash string
}

func (s *Store) CreateUser(ctx context.Context, name, phone, email, passwordHash, role, status string) (*models.User, error) {
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO users (name, phone, email, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, phone, coalesce(email, ''), role, status, plan, photo_url, created_at
	`, name, phone, nullIfEmpty(email), passwordHash, role, status)

	var u models.User
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &u.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*UserWithPassword, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, name, phone, coalesce(email, ''), role, status, plan, photo_url, contacts, created_at, password_hash
		FROM users WHERE phone = $1
	`, phone)

	var u UserWithPassword
	var contacts []byte
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &contacts, &u.CreatedAt, &u.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by phone: %w", err)
	}
	if err := json.Unmarshal(contacts, &u.Contacts); err != nil {
		return nil, fmt.Errorf("decode user contacts: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, name, phone, coalesce(email, ''), role, status, plan, photo_url, created_at
		FROM users WHERE email = $1
	`, email)

	var u models.User
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, name, phone, coalesce(email, ''), role, status, plan, photo_url, contacts, created_at
		FROM users WHERE id = $1
	`, id)

	var u models.User
	var contacts []byte
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &contacts, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if err := json.Unmarshal(contacts, &u.Contacts); err != nil {
		return nil, fmt.Errorf("decode user contacts: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserPasswordHash(ctx context.Context, id string) (string, error) {
	var passwordHash string
	err := s.Pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get user password hash: %w", err)
	}
	return passwordHash, nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserProfile(ctx context.Context, id, name, phone, email string, contacts []models.Contact) (*models.User, error) {
	contactsJSON, err := json.Marshal(contacts)
	if err != nil {
		return nil, fmt.Errorf("encode user contacts: %w", err)
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE users
		SET name = $2, phone = $3, email = NULLIF($4, ''), contacts = $5::jsonb
		WHERE id = $1
		RETURNING id, name, phone, coalesce(email, ''), role, status, plan, photo_url, contacts, created_at
	`, id, name, phone, email, contactsJSON)

	var u models.User
	var storedContacts []byte
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &storedContacts, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("update user profile: %w", err)
	}
	u.Contacts = contacts
	return &u, nil
}

func (s *Store) GetUserPlan(ctx context.Context, id string) (string, error) {
	u, err := s.GetUserByID(ctx, id)
	if err != nil {
		return "", err
	}
	if u.Plan == "" {
		return "free", nil
	}
	return u.Plan, nil
}

func (s *Store) UpdateUserRoleStatus(ctx context.Context, id, role, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET role = $2, status = $3 WHERE id = $1`, id, role, status)
	return err
}

func (s *Store) SetUserPlan(ctx context.Context, id, plan string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET plan = $2 WHERE id = $1`, id, plan)
	return err
}

func (s *Store) UpdateFcmToken(ctx context.Context, id, token string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET fcm_token = $2 WHERE id = $1`, id, token)
	return err
}

func (s *Store) GetFcmToken(ctx context.Context, id string) (string, error) {
	var token string
	err := s.Pool.QueryRow(ctx, `SELECT coalesce(fcm_token, '') FROM users WHERE id = $1`, id).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return token, err
}

func (s *Store) ListLandlords(ctx context.Context) ([]models.User, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, phone, coalesce(email, ''), role, status, plan, photo_url, created_at
		FROM users WHERE role = 'landlord'
	`)
	if err != nil {
		return nil, fmt.Errorf("list landlords: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Role, &u.Status, &u.Plan, &u.PhotoURL, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
