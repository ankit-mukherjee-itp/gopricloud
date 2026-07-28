package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"gopricloud/gopricloud/internal/domain"
	"gopricloud/gopricloud/internal/repository"
)

type refreshTokenRepository struct {
	db *sql.DB
}

// NewRefreshTokenRepository returns a Postgres-backed
// repository.RefreshTokenRepository.
func NewRefreshTokenRepository(db *sql.DB) repository.RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(ctx context.Context, t *domain.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, q,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.Revoked, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *refreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked, created_at
		FROM refresh_tokens WHERE token_hash = $1
	`
	var t domain.RefreshToken
	err := r.db.QueryRowContext(ctx, q, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Revoked, &t.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &t, nil
}

func (r *refreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`
	if _, err := r.db.ExecContext(ctx, q, userID); err != nil {
		return fmt.Errorf("revoke refresh tokens for user: %w", err)
	}
	return nil
}
