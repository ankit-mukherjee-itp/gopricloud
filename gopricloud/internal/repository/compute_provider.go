package repository

import (
	"context"

	"gopricloud/gopricloud/internal/domain"
)

// ComputeProvider is the port for provisioning compute instances against an
// IaaS backend. The OpenStack/gophercloud implementation lives outside
// internal/, under openstack/compute.
type ComputeProvider interface {
	CreateServer(ctx context.Context, params domain.ComputeCreateParams) (*domain.ComputeServer, error)
	GetServer(ctx context.Context, serviceID string) (*domain.ComputeServer, error)
	DeleteServer(ctx context.Context, serviceID string) error
}
