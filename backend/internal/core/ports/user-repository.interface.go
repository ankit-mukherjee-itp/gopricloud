package ports

import (
	"context"

	"backend/internal/core/domain"
	"github.com/google/uuid"
)

// UserRepository is the persistence port for domain.User. Implementations
// live under internal/infrastructure/.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}
