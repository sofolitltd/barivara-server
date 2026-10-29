package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Admin accounts are kept in their own table, separate from app users
// (renters/landlords) — email+password login, no phone, no role field
// (every row here is an admin).
type Admin struct {
	ID    string
	Name  string
	Email string
}

type AdminWithPassword struct {
	Admin
	PasswordHash string
}

func (s *Store) GetAdminByEmail(ctx context.Context, email string) (*AdminWithPassword, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash
		FROM admins WHERE email = $1
	`, email)

	var a AdminWithPassword
	if err := row.Scan(&a.ID, &a.Name, &a.Email, &a.PasswordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get admin by email: %w", err)
	}
	return &a, nil
}
