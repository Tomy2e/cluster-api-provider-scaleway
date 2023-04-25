package vpcgw

import (
	"context"

	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
)

type Service struct {
	ClusterScope *scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{clusterScope}
}

func (s *Service) Reconcile(ctx context.Context) error {
	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	return nil
}
