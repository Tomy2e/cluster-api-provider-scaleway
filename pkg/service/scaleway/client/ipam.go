package client

import (
	"context"

	ipam "github.com/scaleway/scaleway-sdk-go/api/ipam/v1alpha1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindIPv4ByInstancePrivateNICID(ctx context.Context, region scw.Region, pnicID string) (*scw.IPNet, error) {
	ips, err := c.IPAM.ListIPs(&ipam.ListIPsRequest{
		Region:       region,
		ProjectID:    &c.ProjectID,
		ResourceType: ipam.ResourceTypeInstancePrivateNic,
		ResourceID:   &pnicID,
		IsIPv6:       scw.BoolPtr(false),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if len(ips.IPs) > 0 {
		return &ips.IPs[0].Address, nil
	}

	return nil, ErrNoItemFound
}
