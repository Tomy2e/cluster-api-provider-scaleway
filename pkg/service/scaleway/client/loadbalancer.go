package client

import (
	"context"

	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindLoadBalancerByName(ctx context.Context, zone scw.Zone, name string) (*lb.LB, error) {
	lbs, err := c.LoadBalancer.ListLBs(&lb.ZonedAPIListLBsRequest{
		Zone: zone,
		Name: scw.StringPtr(name),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, lb := range lbs.LBs {
		if lb.Name == name {
			return lb, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindLoadBalancerBackendByNames(ctx context.Context, zone scw.Zone, lbName, backendName string) (*lb.Backend, error) {
	loadbalancer, err := c.FindLoadBalancerByName(ctx, zone, lbName)
	if err != nil {
		return nil, err
	}

	backends, err := c.LoadBalancer.ListBackends(&lb.ZonedAPIListBackendsRequest{
		Zone: zone,
		LBID: loadbalancer.ID,
		Name: scw.StringPtr(backendName),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, backend := range backends.Backends {
		if backend.Name == backendName {
			return backend, nil
		}
	}

	return nil, ErrNoItemFound
}
