package client

import (
	"context"
	"fmt"

	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindGatewayByName(ctx context.Context, zone scw.Zone, name string) (*vpcgw.Gateway, error) {
	gws, err := c.VPCGW.ListGateways(&vpcgw.ListGatewaysRequest{
		Zone:      zone,
		Name:      &name,
		ProjectID: &c.ProjectID,
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list Public Gateways: %w", err)
	}

	for _, gw := range gws.Gateways {
		if gw.Name == name {
			return gw, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindGatewayIP(ctx context.Context, zone scw.Zone, ip string) (*vpcgw.IP, error) {
	ips, err := c.VPCGW.ListIPs(&vpcgw.ListIPsRequest{
		Zone:      zone,
		IsFree:    scw.BoolPtr(true),
		ProjectID: &c.ProjectID,
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list vpcgw IPs: %w", err)
	}

	for _, vpcgwIP := range ips.IPs {
		if vpcgwIP.Address.String() == ip {
			return vpcgwIP, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindGatewayIPByTags(ctx context.Context, zone scw.Zone, tags []string) (*vpcgw.IP, error) {
	ips, err := c.VPCGW.ListIPs(&vpcgw.ListIPsRequest{
		Zone:      zone,
		Tags:      tags,
		ProjectID: &c.ProjectID,
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return nil, fmt.Errorf("failed to list IPs: %w", err)
	}

	if len(ips.IPs) == 0 {
		return nil, ErrNoItemFound
	}

	if len(ips.IPs) > 1 {
		return nil, fmt.Errorf("%w: found %d IPs", ErrTooManyItemsFound, len(ips.IPs))
	}

	return ips.IPs[0], nil
}
