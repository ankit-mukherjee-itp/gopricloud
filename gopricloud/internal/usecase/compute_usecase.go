package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gopricloud/gopricloud/internal/domain"
	"gopricloud/gopricloud/internal/repository"
)

// ComputeUsecase provisions and tears down OpenStack compute instances on
// behalf of a user, keeping a per-user record of what was provisioned.
type ComputeUsecase struct {
	computes repository.ComputeRepository
	provider repository.ComputeProvider
}

func NewComputeUsecase(computes repository.ComputeRepository, provider repository.ComputeProvider) *ComputeUsecase {
	return &ComputeUsecase{computes: computes, provider: provider}
}

// Create boots a new server via Nova and records it against userID. Nova's
// create response only carries the new server's ID (not its name or
// status), so the record is seeded from params and the known initial
// state; callers can poll GET /instances/{id} for live status.
func (u *ComputeUsecase) Create(ctx context.Context, userID uuid.UUID, params domain.ComputeCreateParams) (*domain.Compute, error) {
	server, err := u.provider.CreateServer(ctx, params)
	if err != nil {
		return nil, err
	}

	record := &domain.Compute{
		ID:               uuid.New(),
		UserID:           userID,
		ComputeServiceID: server.ID,
		Name:             params.Name,
		Status:           domain.ComputeStatusBuilding,
		CreatedAt:        time.Now().UTC(),
	}
	if err := u.computes.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("store compute record: %w", err)
	}

	return record, nil
}

// List returns every compute instance provisioned by userID, so a client
// can repopulate the user's dashboard after signing back in.
func (u *ComputeUsecase) List(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error) {
	return u.computes.ListByUserID(ctx, userID)
}

// Get fetches live state for a compute instance from Nova, after confirming
// serviceID belongs to userID.
func (u *ComputeUsecase) Get(ctx context.Context, userID uuid.UUID, serviceID string) (*domain.ComputeServer, error) {
	if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
		return nil, err
	}
	return u.provider.GetServer(ctx, serviceID)
}

// Delete destroys a compute instance in Nova and removes its record, after
// confirming serviceID belongs to userID.
func (u *ComputeUsecase) Delete(ctx context.Context, userID uuid.UUID, serviceID string) error {
	if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
		return err
	}
	if err := u.provider.DeleteServer(ctx, serviceID); err != nil {
		return err
	}
	return u.computes.DeleteByServiceIDAndUserID(ctx, serviceID, userID)
}
