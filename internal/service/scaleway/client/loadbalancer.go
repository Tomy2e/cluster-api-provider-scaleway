package client

import (
	"context"
	"fmt"

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

func (c *Client) FindLoadBalancerFrontendByNames(ctx context.Context, zone scw.Zone, lbName, frontendName string) (*lb.Frontend, error) {
	loadbalancer, err := c.FindLoadBalancerByName(ctx, zone, lbName)
	if err != nil {
		return nil, err
	}

	frontends, err := c.LoadBalancer.ListFrontends(&lb.ZonedAPIListFrontendsRequest{
		Zone: zone,
		LBID: loadbalancer.ID,
		Name: scw.StringPtr(frontendName),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, frontend := range frontends.Frontends {
		if frontend.Name == frontendName {
			return frontend, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindLoadBalancerIP(ctx context.Context, zone scw.Zone, ip string) (*lb.IP, error) {
	ips, err := c.LoadBalancer.ListIPs(&lb.ZonedAPIListIPsRequest{
		Zone:      zone,
		IPAddress: &ip,
		ProjectID: &c.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list loadbalancer IPs: %w", err)
	}

	for _, lbIP := range ips.IPs {
		if lbIP.IPAddress == ip {
			return lbIP, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindLoadBalancerACLByName(ctx context.Context, zone scw.Zone, frontendID, name string) (*lb.ACL, error) {
	acls, err := c.LoadBalancer.ListACLs(&lb.ZonedAPIListACLsRequest{
		Name:       scw.StringPtr(name),
		Zone:       zone,
		FrontendID: frontendID,
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list ACLs: %w", err)
	}

	for _, acl := range acls.ACLs {
		if acl.Name == name {
			return acl, nil
		}
	}

	return nil, ErrNoItemFound
}
