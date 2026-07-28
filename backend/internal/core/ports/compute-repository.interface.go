package repository

import (
	"context"

	"github.com/google/uuid"
	"gopricloud/gopricloud/internal/domain"
)

// ComputeRepository is the persistence port for domain.Compute records.
type ComputeRepository interface {
	Create(ctx context.Context, c *domain.Compute) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error)
	GetByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) (*domain.Compute, error)
	DeleteByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) error
}
