package client

import (
	"context"
	"fmt"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func (c *Client) FindInstanceByName(ctx context.Context, zone scw.Zone, name string) (*instance.Server, error) {
	instances, err := c.Instance.ListServers(&instance.ListServersRequest{
		Zone: zone,
		Name: scw.StringPtr(name),
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	for _, server := range instances.Servers {
		if server.Name == name {
			return server, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindIPByTags(ctx context.Context, zone scw.Zone, tags []string) (*instance.IP, error) {
	ips, err := c.Instance.ListIPs(&instance.ListIPsRequest{
		Zone: zone,
		Tags: tags,
	}, scw.WithAllPages(), scw.WithContext(ctx))
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

func (c *Client) FindPrivateNICByPNID(ctx context.Context, server *instance.Server, pnID string) (*instance.PrivateNIC, error) {
	pnics, err := c.Instance.ListPrivateNICs(&instance.ListPrivateNICsRequest{
		Zone:     server.Zone,
		ServerID: server.ID,
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, p := range pnics.PrivateNics {
		if p.PrivateNetworkID == pnID {
			return p, nil
		}
	}

	return nil, ErrNoItemFound
}

func (c *Client) FindSecurityGroupByName(ctx context.Context, zone scw.Zone, name string) (*instance.SecurityGroup, error) {
	sgs, err := c.Instance.ListSecurityGroups(&instance.ListSecurityGroupsRequest{
		Zone: zone,
		Name: scw.StringPtr(name),
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return nil, err
	}

	for _, sg := range sgs.SecurityGroups {
		if sg.Name == name {
			return sg, nil
		}
	}

	return nil, ErrNoItemFound
}
