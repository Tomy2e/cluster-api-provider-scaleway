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
