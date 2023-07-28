package client

import (
	"context"

	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindPrivateNetworkByName(ctx context.Context, region scw.Region, name string) (*vpc.PrivateNetwork, error) {
	pns, err := c.VPC.ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
		Region:    region,
		Name:      scw.StringPtr(name),
		ProjectID: &c.ProjectID,
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
