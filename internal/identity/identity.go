package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("canonical user not found")

type Principal struct {
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
}

type Resolver interface {
	Resolve(ctx context.Context, userID string) (Principal, error)
}

type PostgresResolver struct {
	db *pgxpool.Pool
}

func NewPostgresResolver(db *pgxpool.Pool) *PostgresResolver {
	return &PostgresResolver{db: db}
}

func (r *PostgresResolver) Resolve(ctx context.Context, userID string) (Principal, error) {
	var p Principal
	err := r.db.QueryRow(ctx, `
		SELECT tenant_id::text, id::text, COALESCE(display_name, '')
		FROM users
		WHERE id = $1
	`, userID).Scan(&p.TenantID, &p.UserID, &p.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUserNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	return p, nil
}

var _ Resolver = (*PostgresResolver)(nil)
