package scope

import (
	"context"
	"fmt"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	scwClient "github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DefaultLoadBalancerType = "LB-S"

type Cluster struct {
	Client          client.Client
	ScalewayClient  *scwClient.Client
	ScalewayCluster *infrastructurev1beta1.ScalewayCluster
	Cluster         *v1beta1.Cluster
	patchHelper     *patch.Helper
}

type ClusterParams struct {
	Client          client.Client
	ScalewayClient  *scwClient.Client
	ScalewayCluster *infrastructurev1beta1.ScalewayCluster
	Cluster         *v1beta1.Cluster
}

func NewCluster(params *ClusterParams) (*Cluster, error) {
	helper, err := patch.NewHelper(params.ScalewayCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &Cluster{
		Client:          params.Client,
		ScalewayClient:  params.ScalewayClient,
		ScalewayCluster: params.ScalewayCluster,
		Cluster:         params.Cluster,
		patchHelper:     helper,
	}, nil
}

func (c *Cluster) PatchObject(ctx context.Context) error {
	return c.patchHelper.Patch(ctx, c.ScalewayCluster)
}

func (c *Cluster) Close(ctx context.Context) error {
	return c.PatchObject(ctx)
}

func (c *Cluster) Region() scw.Region {
	return scw.Region(c.ScalewayCluster.Spec.Region)
}

// LoadBalancerZone returns the zone where the LoadBalancer should be created.
func (c *Cluster) LoadBalancerZone() scw.Zone {
	if c.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil &&
		c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Zone != nil {
		return scw.Zone(*c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Zone)
	}

	if len(c.ScalewayCluster.Spec.FailureDomains) > 0 {
		return scw.Zone(c.ScalewayCluster.Spec.FailureDomains[0])
	}

	return scw.Zone(fmt.Sprintf("%s-1", c.ScalewayCluster.Spec.Region))
}

func (c *Cluster) LoadBalancerType() string {
	if c.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil {
		return c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Type
	}

	return DefaultLoadBalancerType
}

// ShouldManagePrivateNetwork returns true if PrivateNetwork is enabled and
// no existing PrivateNetwork is provided.
func (c *Cluster) ShouldManagePrivateNetwork() bool {
	return c.HasPrivateNetwork() && c.ScalewayCluster.Spec.Network.PrivateNetwork.ID == nil
}

func (c *Cluster) HasPrivateNetwork() bool {
	return c.ScalewayCluster.Spec.Network.PrivateNetwork != nil &&
		c.ScalewayCluster.Spec.Network.PrivateNetwork.Enabled
}

// Name returns the name that resources created for the cluster should have.
func (c *Cluster) Name() string {
	return fmt.Sprintf("caps-%s", c.ScalewayCluster.Name)
}

func (c *Cluster) PrivateNetworkID(ctx context.Context, zone scw.Zone) (string, error) {
	var pnID string

	if c.ScalewayCluster.Spec.Network.PrivateNetwork.ID != nil {
		pnID = *c.ScalewayCluster.Spec.Network.PrivateNetwork.ID
	} else {
		pn, err := c.ScalewayClient.FindPrivateNetworkByName(ctx, zone, c.Name())
		if err != nil {
			return "", fmt.Errorf("could not find PrivateNetwork by name: %w", err)
		}

		pnID = pn.ID
	}

	return pnID, nil
}
