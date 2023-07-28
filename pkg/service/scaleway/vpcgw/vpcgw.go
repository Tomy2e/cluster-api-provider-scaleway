package vpcgw

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
)

const defaultVPCGWType = "VPC-GW-S"

type Service struct {
	ClusterScope *scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{clusterScope}
}

func (s *Service) getOrCreateIP(ctx context.Context, zone scw.Zone, existingIP *string) (*vpcgw.IP, error) {
	if existingIP != nil {
		ip, err := s.ClusterScope.ScalewayClient.FindGatewayIP(ctx, zone, *s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.IP)
		if err != nil {
			return nil, fmt.Errorf("failed to find IP %q: %w", *s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.IP, err)
		}

		return ip, nil
	}

	ip, err := s.ClusterScope.ScalewayClient.FindGatewayIPByTags(ctx, zone, s.ClusterScope.Tags())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if ip == nil {
		ip, err = s.ClusterScope.ScalewayClient.VPCGW.CreateIP(&vpcgw.CreateIPRequest{
			Zone: zone,
			Tags: s.ClusterScope.Tags(),
		})
		if err != nil {
			return nil, err
		}
	}

	return ip, nil
}

func (s *Service) getOrCreateGateway(ctx context.Context, zone scw.Zone) (*vpcgw.Gateway, error) {
	gw, err := s.ClusterScope.ScalewayClient.FindGatewayByName(ctx, zone, s.ClusterScope.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, fmt.Errorf("failed to find Public Gateway by name: %w", err)
	}

	if gw == nil {
		ip, err := s.getOrCreateIP(ctx, zone, s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.IP)
		if err != nil {
			return nil, err
		}

		vpcgwType := s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.Type
		if vpcgwType == nil {
			vpcgwType = scw.StringPtr(defaultVPCGWType)
		}

		gw, err = s.ClusterScope.ScalewayClient.VPCGW.CreateGateway(&vpcgw.CreateGatewayRequest{
			Zone: zone,
			Name: s.ClusterScope.Name(),
			IPID: &ip.ID,
			Type: *vpcgwType,
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to create Public Gateway: %w", err)
		}
	}

	return gw, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	if !s.ClusterScope.HasPrivateNetwork() || !s.ClusterScope.HasPublicGateway() {
		return nil
	}

	zone := s.ClusterScope.PublicGatewayZone()
	gatewayID := s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.ID

	if gatewayID == nil {
		gw, err := s.getOrCreateGateway(ctx, zone)
		if err != nil {
			return err
		}

		gatewayID = &gw.ID
	}

	pnID, err := s.ClusterScope.PrivateNetworkID()
	if err != nil {
		return err
	}

	pn, err := s.ClusterScope.ScalewayClient.VPC.GetPrivateNetwork(&vpc.GetPrivateNetworkRequest{
		Region:           s.ClusterScope.Region(),
		PrivateNetworkID: pnID,
	})
	if err != nil {
		return fmt.Errorf("failed to get Private Network %q: %w", pnID, err)
	}

	idx := slices.IndexFunc(pn.Subnets, func(subnet *vpc.Subnet) bool {
		return subnet.Subnet.IP.To4() != nil
	})
	if idx == -1 {
		return errors.New("the Private Network has no ipv4 subnet")
	}

	// Check if gateway is already attached to the PN.
	gwNeworks, err := s.ClusterScope.ScalewayClient.VPCGW.ListGatewayNetworks(&vpcgw.ListGatewayNetworksRequest{
		Zone:             zone,
		GatewayID:        gatewayID,
		PrivateNetworkID: &pnID,
	}, scw.WithContext(ctx), scw.WithAllPages())
	if err != nil {
		return err
	}

	if gwNeworks.TotalCount == 0 {
		if _, err := s.ClusterScope.ScalewayClient.VPCGW.CreateGatewayNetwork(&vpcgw.CreateGatewayNetworkRequest{
			Zone:             zone,
			GatewayID:        *gatewayID,
			PrivateNetworkID: pnID,
			EnableDHCP:       scw.BoolPtr(true),
			EnableMasquerade: true,
			DHCP: &vpcgw.CreateDHCPRequest{
				PushDefaultRoute: scw.BoolPtr(true),
				ProjectID:        s.ClusterScope.ScalewayClient.ProjectID,
				Subnet:           pn.Subnets[idx].Subnet,
			},
		}, scw.WithContext(ctx)); err != nil {
			return err
		}
	}

	// TODO: set public gateway ID in status.

	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	if !s.ClusterScope.HasPrivateNetwork() || !s.ClusterScope.HasPublicGateway() {
		return nil
	}

	if s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.ID != nil {
		return nil
	}

	zone := s.ClusterScope.PublicGatewayZone()

	gw, err := s.ClusterScope.ScalewayClient.FindGatewayByName(ctx, zone, s.ClusterScope.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return fmt.Errorf("failed to find PublicGateway: %w", err)
	}

	if err == nil {
		if err := s.ClusterScope.ScalewayClient.PublicGateway.DeleteGateway(&vpcgw.DeleteGatewayRequest{
			Zone:      zone,
			GatewayID: gw.ID,
		}); err != nil {
			return fmt.Errorf("failed to delete PublicGateway: %w", err)
		}
	}

	// Release IP if an IP was automatically created.
	if s.ClusterScope.ScalewayCluster.Spec.Network.PublicGateway.IP == nil {
		ip, err := s.ClusterScope.ScalewayClient.FindGatewayIPByTags(ctx, zone, s.ClusterScope.Tags())
		if err != nil && !errors.Is(err, client.ErrNoItemFound) {
			return fmt.Errorf("failed to find Public Gateway IP: %w", err)
		}

		if err == nil {
			if err := s.ClusterScope.ScalewayClient.PublicGateway.DeleteIP(&vpcgw.DeleteIPRequest{
				Zone: zone,
				IPID: ip.ID,
			}); err != nil {
				return fmt.Errorf("failed to delete Public Gateway IP: %w", err)
			}
		}
	}

	return nil
}
