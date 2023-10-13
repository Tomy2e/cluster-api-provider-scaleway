package vpc

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/client"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

type Service struct {
	*scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{clusterScope}
}

func (s *Service) getOrCreatePN(ctx context.Context) (*vpc.PrivateNetwork, error) {
	region := s.Region()

	pn, err := s.ScalewayClient.FindPrivateNetworkByName(ctx, region, s.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if pn == nil {
		var subnets []scw.IPNet

		if s.ScalewayCluster.Spec.Network.PrivateNetwork.Subnet != nil {
			_, ipNet, err := net.ParseCIDR(*s.ScalewayCluster.Spec.Network.PrivateNetwork.Subnet)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PrivateNetwork subnet: %w", err)
			}

			subnets = append(subnets, scw.IPNet{IPNet: *ipNet})
		}

		pn, err = s.ScalewayClient.VPC.CreatePrivateNetwork(&vpc.CreatePrivateNetworkRequest{
			Region:  region,
			Name:    s.Name(),
			Subnets: subnets,
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	}

	if !pn.DHCPEnabled {
		return nil, errors.New("DHCP is not enabled in the specified Private Network")
	}

	return pn, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	if !s.ShouldManagePrivateNetwork() {
		return nil
	}

	pn, err := s.getOrCreatePN(ctx)
	if err != nil {
		return fmt.Errorf("failed to get or create Private Network: %w", err)
	}

	s.SetStatusPrivateNetworkID(pn.ID)

	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	if !s.ShouldManagePrivateNetwork() {
		return nil
	}

	region := s.Region()

	pn, err := s.ScalewayClient.FindPrivateNetworkByName(ctx, region, s.Name())
	if err != nil {
		if errors.Is(err, client.ErrNoItemFound) {
			return nil
		}

		return err
	}

	return s.ScalewayClient.VPC.DeletePrivateNetwork(&vpc.DeletePrivateNetworkRequest{
		Region:           region,
		PrivateNetworkID: pn.ID,
	}, scw.WithContext(ctx))
}
