package scope

import (
	"context"
	"fmt"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	scwClient "github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/pkg/errors"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
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

// Region returns the region of the cluster.
func (c *Cluster) Region() scw.Region {
	return scw.Region(c.ScalewayCluster.Spec.Region)
}

// DefaultZone returns the first zone of the region. It's useful when no zone is
// provided but at least one is needed.
func (c *Cluster) DefaultZone() scw.Zone {
	return scw.Zone(fmt.Sprintf("%s-1", c.ScalewayCluster.Spec.Region))
}

// Zones returns all available zones for the cluster Region that are compatible
// with the provided zones. If no compatible zones are provided, all zones are
// returned. If no zones are found, the default zone is returned so that there
// is always at least one zone returned.
func (c *Cluster) Zones(compatible []scw.Zone) (zones []scw.Zone) {
	for _, z := range c.Region().GetZones() {
		if slices.Contains(compatible, z) || len(compatible) == 0 {
			zones = append(zones, z)
		}
	}

	if len(zones) == 0 {
		zones = append(zones, c.DefaultZone())
	}

	return zones
}

// FailureDomains returns the Failure Domains for this cluster.
func (c *Cluster) FailureDomains() v1beta1.FailureDomains {
	zones := c.Zones(nil)
	failureDomains := make(v1beta1.FailureDomains, len(zones))

	for _, zone := range zones {
		if len(c.ScalewayCluster.Spec.FailureDomains) > 0 {
			for _, fd := range c.ScalewayCluster.Spec.FailureDomains {
				if fd == zone.String() {
					failureDomains[zone.String()] = v1beta1.FailureDomainSpec{
						ControlPlane: true,
					}
				}
			}
		} else {
			failureDomains[zone.String()] = v1beta1.FailureDomainSpec{
				ControlPlane: true,
			}
		}
	}

	return failureDomains
}

// LoadBalancerZone returns the zone where the LoadBalancer should be created.
func (c *Cluster) LoadBalancerZone() scw.Zone {
	if c.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil &&
		c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Zone != nil {
		return scw.Zone(*c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Zone)
	}

	// TODO: if first failure domain is not compatible this will fail.
	if len(c.ScalewayCluster.Spec.FailureDomains) > 0 {
		return scw.Zone(c.ScalewayCluster.Spec.FailureDomains[0])
	}

	return c.DefaultZone()
}

func (c *Cluster) PublicGatewayZone() scw.Zone {
	if c.ScalewayCluster.Spec.Network.PublicGateway != nil &&
		c.ScalewayCluster.Spec.Network.PublicGateway.Zone != nil {
		return scw.Zone(*c.ScalewayCluster.Spec.Network.PublicGateway.Zone)
	}

	// TODO: if first failure domain is not compatible this will fail.
	if len(c.ScalewayCluster.Spec.FailureDomains) > 0 {
		return scw.Zone(c.ScalewayCluster.Spec.FailureDomains[0])
	}

	return c.DefaultZone()
}

// LoadBalancerType returns the type of the control-plane Load Balancer.
func (c *Cluster) LoadBalancerType() string {
	if c.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil {
		return c.ScalewayCluster.Spec.ControlPlaneLoadBalancer.Type
	}

	return DefaultLoadBalancerType
}

// ShouldManagePrivateNetwork returns true if PrivateNetwork is enabled and
// no existing PrivateNetwork is provided.
func (c *Cluster) ShouldManagePrivateNetwork() bool {
	return c.HasPrivateNetwork() &&
		c.ScalewayCluster.Spec.Network.PrivateNetwork.ID == nil
}

// HasPrivateNetwork returns true if the Cluster has a Private Network (either
// managed by the cluster or an existing one).
func (c *Cluster) HasPrivateNetwork() bool {
	return c.ScalewayCluster.Spec.Network.PrivateNetwork != nil &&
		c.ScalewayCluster.Spec.Network.PrivateNetwork.Enabled
}

func (c *Cluster) HasPublicGateway() bool {
	return c.ScalewayCluster.Spec.Network.PublicGateway != nil &&
		c.ScalewayCluster.Spec.Network.PublicGateway.Enabled
}

// Name returns the name that resources created for the cluster should have.
func (c *Cluster) Name() string {
	return fmt.Sprintf("caps-%s", c.ScalewayCluster.Name)
}

func (c *Cluster) PrivateNetworkID() (string, error) {
	if !c.HasPrivateNetwork() {
		return "", errors.New("cluster has no Private Network")
	}

	if c.ScalewayCluster.Status.Network.PrivateNetworkID == nil {
		return "", errors.New("PrivateNetworkID not found in ScalewayCluster status")
	}

	return *c.ScalewayCluster.Status.Network.PrivateNetworkID, nil
}

// SetStatusPrivateNetworkID sets the Private Network ID in the status of the
// ScalewayCluster object.
func (c *Cluster) SetStatusPrivateNetworkID(pnID string) {
	if c.ScalewayCluster.Status.Network == nil {
		c.ScalewayCluster.Status.Network = &infrastructurev1beta1.NetworkStatus{
			PrivateNetworkID: &pnID,
		}
	} else {
		c.ScalewayCluster.Status.Network.PrivateNetworkID = &pnID
	}
}
