package store

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

// toJSON marshals a value for a jsonb column, defaulting to its zero-value
// JSON shape so callers never pass nil into pgx.
func toJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}
