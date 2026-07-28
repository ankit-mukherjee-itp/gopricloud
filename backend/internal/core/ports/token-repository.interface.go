package repository

import (
	"context"

	"github.com/google/uuid"
	"gopricloud/gopricloud/internal/domain"
)

// RefreshTokenRepository is the persistence port for domain.RefreshToken.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}
