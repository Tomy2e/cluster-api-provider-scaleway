package client

import (
	"context"

	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindPrivateNetworkByName(ctx context.Context, zone scw.Zone, name string) (*vpc.PrivateNetwork, error) {
	pns, err := c.VPC.ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
		Zone: zone,
		Name: scw.StringPtr(name),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, pn := range pns.PrivateNetworks {
		if pn.Name == name {
			return pn, nil
		}
	}

	return nil, ErrNoItemFound
}
