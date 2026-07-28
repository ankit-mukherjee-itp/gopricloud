// Package openstack is the OpenStack Nova adapter: it only knows how to talk
// to the compute service via gophercloud, using the cloud entry in
// clouds.yaml. It has no knowledge of HTTP, the database, or users - that
// wiring happens in the composition root via the ports.ComputeProvider port.
package openstack

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

// NewClient authenticates against OpenStack using the named cloud entry
// from clouds.yaml (searched for in the current working directory, then
// the standard OpenStack config locations) and returns a Nova v2 service
// client.
func NewClient(ctx context.Context, cloudName string) (*gophercloud.ServiceClient, error) {
	client, err := clientconfig.NewServiceClient(ctx, "compute", &clientconfig.ClientOpts{
		Cloud: cloudName,
	})
	if err != nil {
		return nil, fmt.Errorf("create openstack compute client: %w", err)
	}
	return client, nil
}
