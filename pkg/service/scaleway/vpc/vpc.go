package vpc

import (
	"context"
	"errors"

	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

type Service struct {
	*scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{clusterScope}
}

func (s *Service) getOrCreatePN(ctx context.Context, zone scw.Zone) (*vpc.PrivateNetwork, error) {
	pn, err := s.ScalewayClient.FindPrivateNetworkByName(ctx, zone, s.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if pn == nil {
		pn, err = s.ScalewayClient.VPC.CreatePrivateNetwork(&vpc.CreatePrivateNetworkRequest{
			Zone: zone,
			Name: s.Name(),
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	}

	return pn, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	if !s.ShouldManagePrivateNetwork() {
		return nil
	}

	// TODO: create regional PN.
	_, err := s.getOrCreatePN(ctx, s.LoadBalancerZone())
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	if !s.ShouldManagePrivateNetwork() {
		return nil
	}

	zone := s.LoadBalancerZone()

	pn, err := s.ScalewayClient.FindPrivateNetworkByName(ctx, zone, s.Name())
	if err != nil {
		if errors.Is(err, client.ErrNoItemFound) {
			return nil
		}

		return err
	}

	return s.ScalewayClient.VPC.DeletePrivateNetwork(&vpc.DeletePrivateNetworkRequest{
		Zone:             zone,
		PrivateNetworkID: pn.ID,
	}, scw.WithContext(ctx))
}
