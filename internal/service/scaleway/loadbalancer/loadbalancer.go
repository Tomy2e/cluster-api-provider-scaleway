package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/client"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
)

const (
	ControlPlaneBackendName         = "control-plane"
	ControlPlaneFrontendName        = "control-plane"
	BackendControlPlanePort         = 6443
	DefaultFrontendControlPlanePort = 6443
)

var ErrLoadBalancerNotReady = errors.New("loadbalancer is not ready")

type Service struct {
	*scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{
		Cluster: clusterScope,
	}
}

// TODO: allow migrating the load balancer to other types.
func (s *Service) getOrCreateLB(ctx context.Context, zone scw.Zone) (*lb.LB, error) {
	loadbalancer, err := s.ScalewayClient.FindLoadBalancerByName(ctx, zone, s.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if loadbalancer == nil {
		var ipID *string

		if s.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil &&
			s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.IP != nil {
			ip, err := s.ScalewayClient.FindLoadBalancerIP(ctx, zone, *s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.IP)
			if err != nil {
				return nil, fmt.Errorf("failed to find IP %q: %w", *s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.IP, err)
			}

			ipID = &ip.ID
		}

		loadbalancer, err = s.ScalewayClient.LoadBalancer.CreateLB(&lb.ZonedAPICreateLBRequest{
			Zone: zone,
			Name: s.Name(),
			Type: s.LoadBalancerType(),
			IPID: ipID,
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	}

	return loadbalancer, nil
}

func (s *Service) ensurePrivateNetwork(ctx context.Context, loadbalancer *lb.LB, pnID *string) error {
	if pnID == nil {
		return nil
	}

	lbPNs, err := s.ScalewayClient.LoadBalancer.ListLBPrivateNetworks(&lb.ZonedAPIListLBPrivateNetworksRequest{
		Zone: loadbalancer.Zone,
		LBID: loadbalancer.ID,
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return err
	}

	found := slices.IndexFunc(lbPNs.PrivateNetwork, func(lbPN *lb.PrivateNetwork) bool {
		return lbPN.PrivateNetworkID == *pnID
	})

	if found == -1 {
		if _, err := s.ScalewayClient.LoadBalancer.AttachPrivateNetwork(&lb.ZonedAPIAttachPrivateNetworkRequest{
			Zone:             loadbalancer.Zone,
			LBID:             loadbalancer.ID,
			PrivateNetworkID: *pnID,
			IpamConfig:       &lb.PrivateNetworkIpamConfig{},
		}, scw.WithContext(ctx)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ensureBackend(ctx context.Context, loadbalancer *lb.LB) (*lb.Backend, error) {
	backends, err := s.ScalewayClient.LoadBalancer.ListBackends(&lb.ZonedAPIListBackendsRequest{
		Zone: loadbalancer.Zone,
		LBID: loadbalancer.ID,
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	var backend *lb.Backend
	for _, backendCandidate := range backends.Backends {
		if backendCandidate.Name == ControlPlaneBackendName {
			backend = backendCandidate
			continue
		}

		if err := s.ScalewayClient.LoadBalancer.DeleteBackend(&lb.ZonedAPIDeleteBackendRequest{
			Zone:      loadbalancer.Zone,
			BackendID: backendCandidate.ID,
		}, scw.WithContext(ctx)); err != nil {
			return nil, err
		}
	}

	if backend == nil {
		backend, err = s.ScalewayClient.LoadBalancer.CreateBackend(&lb.ZonedAPICreateBackendRequest{
			Zone:            loadbalancer.Zone,
			LBID:            loadbalancer.ID,
			Name:            ControlPlaneBackendName,
			ForwardProtocol: lb.ProtocolTCP,
			ForwardPort:     BackendControlPlanePort,
			HealthCheck: &lb.HealthCheck{
				Port:            BackendControlPlanePort,
				CheckMaxRetries: 5,
				TCPConfig:       &lb.HealthCheckTCPConfig{},
			},
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	}

	return backend, nil
}

func (s *Service) ensureFrontend(ctx context.Context, loadbalancer *lb.LB, backend *lb.Backend) (*lb.Frontend, error) {
	frontends, err := s.ScalewayClient.LoadBalancer.ListFrontends(&lb.ZonedAPIListFrontendsRequest{
		Zone: loadbalancer.Zone,
		LBID: loadbalancer.ID,
	}, scw.WithAllPages(), scw.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	var frontend *lb.Frontend
	for _, frontendCandidate := range frontends.Frontends {
		if frontendCandidate.Name == ControlPlaneFrontendName {
			frontend = frontendCandidate
			continue
		}

		if err := s.ScalewayClient.LoadBalancer.DeleteFrontend(&lb.ZonedAPIDeleteFrontendRequest{
			Zone:       loadbalancer.Zone,
			FrontendID: frontendCandidate.ID,
		}, scw.WithContext(ctx)); err != nil {
			return nil, err
		}
	}

	if frontend == nil {
		frontend, err = s.ScalewayClient.LoadBalancer.CreateFrontend(&lb.ZonedAPICreateFrontendRequest{
			Zone:        loadbalancer.Zone,
			LBID:        loadbalancer.ID,
			Name:        ControlPlaneFrontendName,
			InboundPort: DefaultFrontendControlPlanePort,
			BackendID:   backend.ID,
		})
		if err != nil {
			return nil, err
		}
	}

	return frontend, nil
}

// ensureACL ensures the ACL with specified parameters exists or doesn't exist.
// If the ACL doesn't contain any IP, this method will ensure the ACL doesn't exist.
func (s *Service) ensureACL(ctx context.Context, frontendID, name string, ips []string, deny bool, index int32) error {
	acl, err := s.ScalewayClient.FindLoadBalancerACLByName(ctx, s.LoadBalancerZone(), frontendID, name)
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return err
	}

	// Remove ACL / Do nothing if there is no IP in it.
	if len(ips) == 0 {
		if acl != nil {
			if err := s.ScalewayClient.LoadBalancer.DeleteACL(&lb.ZonedAPIDeleteACLRequest{
				Zone:  s.LoadBalancerZone(),
				ACLID: acl.ID,
			}, scw.WithContext(ctx)); err != nil {
				return err
			}
		}

		return nil
	}

	action := lb.ACLActionTypeAllow
	if deny {
		action = lb.ACLActionTypeDeny
	}

	// Create ACL if it does not exist.
	if acl == nil {
		_, err := s.ScalewayClient.LoadBalancer.CreateACL(&lb.ZonedAPICreateACLRequest{
			Zone:       s.LoadBalancerZone(),
			FrontendID: frontendID,
			Name:       name,
			Index:      index,
			Action:     &lb.ACLAction{Type: action},
			Match:      &lb.ACLMatch{IPSubnet: scw.StringSlicePtr(ips)},
		}, scw.WithContext(ctx))

		return err
	}

	// Update ACL if ips are different.
	if acl.Match == nil || !slices.Equal(scw.StringSlicePtr(ips), acl.Match.IPSubnet) {
		_, err = s.ScalewayClient.LoadBalancer.UpdateACL(&lb.ZonedAPIUpdateACLRequest{
			Zone:   s.LoadBalancerZone(),
			ACLID:  acl.ID,
			Name:   name,
			Action: &lb.ACLAction{Type: action},
			Index:  index,
			Match:  &lb.ACLMatch{IPSubnet: scw.StringSlicePtr(ips)},
		}, scw.WithContext(ctx))
		return err
	}

	return nil
}

func (s *Service) ensureACLs(ctx context.Context, frontend *lb.Frontend, pnID *string) error {
	// Set the Allowed Ranges ACL.
	var (
		allowedRanges []string
		denyAll       []string
	)

	if s.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil && len(s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.AllowedRanges) > 0 {
		allowedRanges = s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.AllowedRanges
		denyAll = []string{"0.0.0.0/0", "::/0"}
	}

	if err := s.ensureACL(ctx, frontend.ID, "allowed-ranges", allowedRanges, false, 1); err != nil {
		return fmt.Errorf("failed to set allowed-ranges ACL: %w", err)
	}

	// Set the Public Gateway ACL.
	if pnID != nil && s.HasPrivateNetwork() {
		gws, err := s.ScalewayClient.FindGatewaysByPrivateNetworkID(ctx, s.Zones(s.ScalewayClient.VPCGW.Zones()), *pnID)
		if err != nil {
			return err
		}

		var ips []string

		for _, gw := range gws {
			if gw.IP != nil {
				ips = append(ips, gw.IP.Address.String())
			}
		}

		if err := s.ensureACL(ctx, frontend.ID, "public-gateway", ips, false, 2); err != nil {
			return err
		}
	}

	// Set the Deny All ACL. If denyAll is empty, it will not be created (or it
	// will be deleted if it exists).
	if err := s.ensureACL(ctx, frontend.ID, "deny-all", denyAll, true, 4); err != nil {
		return err
	}

	return nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	loadbalancer, err := s.getOrCreateLB(ctx, s.LoadBalancerZone())
	if err != nil {
		return err
	}

	if loadbalancer.Status != lb.LBStatusReady {
		return ErrLoadBalancerNotReady
	}

	var pnID *string

	if s.HasPrivateNetwork() {
		tmpPNID, err := s.PrivateNetworkID()
		if err != nil {
			return err
		}

		pnID = &tmpPNID
	}

	if err := s.ensurePrivateNetwork(ctx, loadbalancer, pnID); err != nil {
		return err
	}

	backend, err := s.ensureBackend(ctx, loadbalancer)
	if err != nil {
		return fmt.Errorf("failed to ensure LoadBalancer backend: %w", err)
	}

	frontend, err := s.ensureFrontend(ctx, loadbalancer, backend)
	if err != nil {
		return fmt.Errorf("failed to ensure LoadBalancer frontend: %w", err)
	}

	if err := s.ensureACLs(ctx, frontend, pnID); err != nil {
		return fmt.Errorf("failed to ensure LoadBalancer ACLs: %w", err)
	}

	var found bool
	for _, lbIP := range loadbalancer.IP {
		ip, err := netip.ParseAddr(lbIP.IPAddress)
		if err != nil {
			return fmt.Errorf("failed to parse loadbalancer IP %q: %w", lbIP.IPAddress, err)
		}

		if ip.Is4() {
			s.ScalewayCluster.Spec.ControlPlaneEndpoint.Host = lbIP.IPAddress
			s.ScalewayCluster.Spec.ControlPlaneEndpoint.Port = frontend.InboundPort
			found = true
			break
		}
	}

	if !found {
		return errors.New("loadbalancer has no IPs")
	}

	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	loadbalancer, err := s.ScalewayClient.FindLoadBalancerByName(ctx, s.LoadBalancerZone(), s.Name())
	if err != nil {
		if errors.Is(err, client.ErrNoItemFound) {
			return nil
		}

		return err
	}

	// Do not release IP if an IP was provided by the user.
	releaseIP := !(s.ScalewayCluster.Spec.ControlPlaneLoadBalancer != nil &&
		s.ScalewayCluster.Spec.ControlPlaneLoadBalancer.IP != nil)

	if err := s.ScalewayClient.LoadBalancer.DeleteLB(&lb.ZonedAPIDeleteLBRequest{
		Zone:      loadbalancer.Zone,
		LBID:      loadbalancer.ID,
		ReleaseIP: releaseIP,
	}); err != nil {
		return fmt.Errorf("failed to delete load balancer: %w", err)
	}

	return nil
}
