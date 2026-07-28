package openstack

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"

	"backend/internal/core/domain"
	"backend/internal/core/ports"
)

// Provider implements ports.ComputeProvider on top of Nova. It
// authenticates lazily, on first use, so an unreachable or misconfigured
// OpenStack cloud only breaks compute endpoints rather than the whole API
// (auth, /test, etc. don't depend on OpenStack at all).
type Provider struct {
	cloudName string

	mu     sync.Mutex
	client *gophercloud.ServiceClient
}

// NewProvider returns a ports.ComputeProvider that will authenticate
// against the named cloud entry (in clouds.yaml) the first time it's asked
// to do anything.
func NewProvider(cloudName string) *Provider {
	return &Provider{cloudName: cloudName}
}

var _ ports.ComputeProvider = (*Provider)(nil)

func (p *Provider) ensureClient(ctx context.Context) (*gophercloud.ServiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		return p.client, nil
	}

	client, err := NewClient(ctx, p.cloudName)
	if err != nil {
		return nil, err
	}
	p.client = client
	return client, nil
}

func (p *Provider) CreateServer(ctx context.Context, params domain.ComputeCreateParams) (*domain.ComputeServer, error) {
	client, err := p.ensureClient(ctx)
	if err != nil {
		return nil, err
	}

	opts := servers.CreateOpts{
		Name:      params.Name,
		ImageRef:  params.ImageRef,
		FlavorRef: params.FlavorRef,
	}
	if params.NetworkID != "" {
		opts.Networks = []servers.Network{{UUID: params.NetworkID}}
	}

	server, err := servers.Create(ctx, client, opts, nil).Extract()
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	return toDomainServer(server), nil
}

func (p *Provider) GetServer(ctx context.Context, serviceID string) (*domain.ComputeServer, error) {
	client, err := p.ensureClient(ctx)
	if err != nil {
		return nil, err
	}

	server, err := servers.Get(ctx, client, serviceID).Extract()
	if err != nil {
		if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
			return nil, domain.ErrComputeNotFound
		}
		return nil, fmt.Errorf("get server: %w", err)
	}
	return toDomainServer(server), nil
}

func (p *Provider) DeleteServer(ctx context.Context, serviceID string) error {
	client, err := p.ensureClient(ctx)
	if err != nil {
		return err
	}

	if err := servers.Delete(ctx, client, serviceID).ExtractErr(); err != nil {
		if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
			return domain.ErrComputeNotFound
		}
		return fmt.Errorf("delete server: %w", err)
	}
	return nil
}

func toDomainServer(s *servers.Server) *domain.ComputeServer {
	return &domain.ComputeServer{
		ID:        s.ID,
		Name:      s.Name,
		Status:    s.Status,
		Addresses: s.Addresses,
	}
}
