package instance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/loadbalancer"
	"github.com/google/uuid"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/marketplace/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
)

var (
	ErrPrivateIPNotFound     = errors.New("private IP not found in IPAM")
	ErrInstanceNotStoppedYet = errors.New("instance is being stopped")
)

type Service struct {
	*scope.Machine
}

func NewService(machineScope *scope.Machine) *Service {
	return &Service{machineScope}
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// getOrCreateIP gets or creates a public IP for the instance. If no IP is needed
// it returns nil.
func (s *Service) getOrCreateIP(ctx context.Context) (*instance.IP, error) {
	if !s.NeedsPublicIP() {
		return nil, nil
	}

	ip, err := s.ScalewayClient.FindIPByTags(ctx, s.Zone(), s.Tags())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if ip == nil {
		ipResp, err := s.ScalewayClient.Instance.CreateIP(&instance.CreateIPRequest{
			Type: instance.IPTypeRoutedIPv4,
			Zone: s.Zone(),
			Tags: s.Tags(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Instance IP: %w", err)
		}

		ip = ipResp.IP
	}

	return ip, nil
}

func (s *Service) getOrCreateServer(ctx context.Context, ip *instance.IP) (*instance.Server, error) {
	server, err := s.ScalewayClient.FindInstanceByName(ctx, s.Zone(), s.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if server == nil {
		rootSize := 20 * scw.GB
		if s.ScalewayMachine.Spec.RootVolumeSize != nil {
			rootSize = scw.Size(*s.ScalewayMachine.Spec.RootVolumeSize) * scw.GB
		}

		imageID := s.ScalewayMachine.Spec.Image
		if !isValidUUID(imageID) {
			imageID, err = s.ScalewayClient.Marketplace.GetLocalImageIDByLabel(&marketplace.GetLocalImageIDByLabelRequest{
				CommercialType: s.ScalewayMachine.Spec.Type,
				Zone:           s.Zone(),
				ImageLabel:     s.ScalewayMachine.Spec.Image,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to find image with label %q: %w", s.ScalewayMachine.Spec.Image, err)
			}
		}

		// Find security group ID if needed.
		var sgID *string
		if s.ScalewayMachine.Spec.SecurityGroupName != nil {
			sgName := s.SecurityGroupName(*s.ScalewayMachine.Spec.SecurityGroupName)
			sg, err := s.ScalewayClient.FindSecurityGroupByName(ctx, s.Zone(), sgName)
			if err != nil {
				return nil, fmt.Errorf("failed to find security group %q: %w", sgName, err)
			}

			sgID = &sg.ID
		}

		req := &instance.CreateServerRequest{
			Zone:              s.Zone(),
			Name:              s.Name(),
			CommercialType:    s.ScalewayMachine.Spec.Type,
			DynamicIPRequired: scw.BoolPtr(false),
			RoutedIPEnabled:   scw.BoolPtr(true),
			Image:             imageID,
			SecurityGroup:     sgID,
			Volumes: map[string]*instance.VolumeServerTemplate{
				"0": {
					Size:       scw.SizePtr(rootSize),
					VolumeType: instance.VolumeVolumeTypeBSSD,
					Boot:       scw.BoolPtr(true),
				},
			},
		}

		if ip != nil {
			req.PublicIP = &ip.ID
		}

		serverResp, err := s.ScalewayClient.Instance.CreateServer(req)
		if err != nil {
			return nil, fmt.Errorf("failed to create server: %w", err)
		}

		server = serverResp.Server
	}

	return server, nil
}

func (s *Service) getOrCreatePrivateNIC(ctx context.Context, server *instance.Server) (*instance.PrivateNIC, error) {
	if !s.HasPrivateNetwork() {
		return nil, nil
	}

	pnID, err := s.PrivateNetworkID()
	if err != nil {
		return nil, err
	}

	pnic, err := s.ScalewayClient.FindPrivateNICByPNID(ctx, server, pnID)
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return nil, err
	}

	if pnic == nil {
		p, err := s.ScalewayClient.Instance.CreatePrivateNIC(&instance.CreatePrivateNICRequest{
			Zone:             s.Zone(),
			ServerID:         server.ID,
			PrivateNetworkID: pnID,
		}, scw.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to create private NIC: %w", err)
		}

		pnic = p.PrivateNic
	}

	return pnic, nil
}

type machineIPs struct {
	Internal *string
	External *string
}

// NodeIP returns the main IP of the node. This IP will be used for communications
// between the LoadBalancer and the control-plane nodes. It can also be used as an
// IP to advertise in the cluster.
func (m *machineIPs) NodeIP() string {
	if m.Internal != nil {
		return *m.Internal
	}

	// Panics if machineIPs has no IP (should never happen).
	return *m.External
}

func (s *Service) getMachineIPs(ctx context.Context, server *instance.Server, pnic *instance.PrivateNIC) (*machineIPs, error) {
	m := &machineIPs{}

	if pnic != nil {
		privateIP, err := s.ScalewayClient.FindIPv4ByInstancePrivateNICID(ctx, s.Cluster.Region(), pnic.ID)
		if err != nil {
			if errors.Is(err, client.ErrNoItemFound) {
				return nil, ErrPrivateIPNotFound
			}

			return nil, err
		}

		m.Internal = scw.StringPtr(privateIP.IP.String())
	}

	if server.PublicIP != nil {
		m.External = scw.StringPtr(server.PublicIP.Address.String())
	}

	if m.External == nil && m.Internal == nil {
		return nil, errors.New("machine has no IP")
	}

	return m, nil
}

type bootstrapValues struct {
	NodeIP     string
	ProviderID string
}

func patchBootstrapData(data []byte, values *bootstrapValues) ([]byte, error) {
	tmpl, err := template.New("bootstrap").Delims("[[[", "]]]").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("unable to parse bootstrap data template: %w", err)
	}
	render := bytes.Buffer{}

	if err := tmpl.Execute(&render, values); err != nil {
		return nil, fmt.Errorf("failed to execute bootstrap data template: %w", err)
	}

	return render.Bytes(), nil
}

func (s *Service) ensureCloudInit(ctx context.Context, server *instance.Server, machineIPs *machineIPs) error {
	if server.State != instance.ServerStateStopped {
		return nil
	}

	userdata, err := s.ScalewayClient.Instance.GetAllServerUserData(&instance.GetAllServerUserDataRequest{
		Zone:     server.Zone,
		ServerID: server.ID,
	})
	if err != nil {
		return err
	}

	if _, ok := userdata.UserData["cloud-init"]; !ok {
		bootstrapData, err := s.GetRawBootstrapData(ctx)
		if err != nil {
			return err
		}

		bootstrapData, err = patchBootstrapData(bootstrapData, &bootstrapValues{
			NodeIP:     machineIPs.NodeIP(),
			ProviderID: s.ProviderID(server.ID),
		})
		if err != nil {
			return err
		}

		if err := s.ScalewayClient.Instance.SetServerUserData(&instance.SetServerUserDataRequest{
			Zone:     server.Zone,
			ServerID: server.ID,
			Key:      "cloud-init",
			Content:  bytes.NewBuffer(bootstrapData),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) ensureServerStarted(ctx context.Context, server *instance.Server) error {
	if server.State != instance.ServerStateStopped {
		return nil
	}

	if _, err := s.ScalewayClient.Instance.ServerAction(&instance.ServerActionRequest{
		Zone:     server.Zone,
		ServerID: server.ID,
		Action:   instance.ServerActionPoweron,
	}, scw.WithContext(ctx)); err != nil {
		return err
	}

	return nil
}

func (s *Service) ensureLoadBalancerACL(ctx context.Context, publicIP *string) error {
	frontend, err := s.ScalewayClient.FindLoadBalancerFrontendByNames(
		ctx,
		s.Cluster.LoadBalancerZone(),
		s.Cluster.Name(),
		loadbalancer.ControlPlaneFrontendName,
	)
	if err != nil {
		return fmt.Errorf("failed to find load balancer frontend: %w", err)
	}

	acl, err := s.ScalewayClient.FindLoadBalancerACLByName(ctx, s.LoadBalancerZone(), frontend.ID, s.Name())
	if err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return fmt.Errorf("failed to find load balancer ACL: %w", err)
	}

	if publicIP == nil {
		if acl != nil {
			if err := s.ScalewayClient.LoadBalancer.DeleteACL(&lb.ZonedAPIDeleteACLRequest{
				Zone:  s.LoadBalancerZone(),
				ACLID: acl.ID,
			}, scw.WithContext(ctx)); err != nil {
				return fmt.Errorf("failed to delete load balancer ACL: %w", err)
			}
		}

		return nil
	}

	match := scw.StringSlicePtr([]string{*publicIP})

	if acl == nil {
		_, err := s.ScalewayClient.LoadBalancer.CreateACL(&lb.ZonedAPICreateACLRequest{
			Zone:       s.LoadBalancerZone(),
			FrontendID: frontend.ID,
			Name:       s.Name(),
			Action:     &lb.ACLAction{Type: lb.ACLActionTypeAllow},
			Match:      &lb.ACLMatch{IPSubnet: match},
			Index:      3,
		}, scw.WithContext(ctx))

		return err
	}

	if acl.Match == nil || !slices.Equal(match, acl.Match.IPSubnet) {
		_, err := s.ScalewayClient.LoadBalancer.UpdateACL(&lb.ZonedAPIUpdateACLRequest{
			Zone:   s.LoadBalancerZone(),
			ACLID:  acl.ID,
			Name:   s.Name(),
			Action: &lb.ACLAction{Type: lb.ACLActionTypeAllow},
			Match:  &lb.ACLMatch{IPSubnet: match},
			Index:  3,
		}, scw.WithContext(ctx))

		return err
	}

	return nil
}

func (s *Service) ensureControlPlaneLoadBalancer(ctx context.Context, server *instance.Server, pnic *instance.PrivateNIC, deletion bool) (*machineIPs, error) {
	// TODO: getMachineIPs out of this method
	ips, err := s.getMachineIPs(ctx, server, pnic)
	if err != nil {
		return nil, err
	}

	if !util.IsControlPlaneMachine(s.Machine.Machine) {
		return ips, nil
	}

	backend, err := s.ScalewayClient.FindLoadBalancerBackendByNames(
		ctx,
		s.Cluster.LoadBalancerZone(),
		s.Cluster.Name(),
		loadbalancer.ControlPlaneBackendName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find load balancer backend: %w", err)
	}

	switch {
	case deletion && slices.Contains(backend.Pool, ips.NodeIP()):
		if slices.Contains(backend.Pool, ips.NodeIP()) {
			if _, err := s.ScalewayClient.LoadBalancer.RemoveBackendServers(&lb.ZonedAPIRemoveBackendServersRequest{
				Zone:      s.Cluster.LoadBalancerZone(),
				BackendID: backend.ID,
				ServerIP:  []string{ips.NodeIP()},
			}); err != nil {
				return nil, err
			}
		}
	case !deletion && !slices.Contains(backend.Pool, ips.NodeIP()):
		if _, err := s.ScalewayClient.LoadBalancer.AddBackendServers(&lb.ZonedAPIAddBackendServersRequest{
			Zone:      s.Cluster.LoadBalancerZone(),
			BackendID: backend.ID,
			ServerIP:  []string{ips.NodeIP()},
		}); err != nil {
			return nil, err
		}
	}

	return ips, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	ip, err := s.getOrCreateIP(ctx)
	if err != nil {
		return err
	}

	server, err := s.getOrCreateServer(ctx, ip)
	if err != nil {
		return err
	}

	pnic, err := s.getOrCreatePrivateNIC(ctx, server)
	if err != nil {
		return err
	}

	machineIPs, err := s.ensureControlPlaneLoadBalancer(ctx, server, pnic, false)
	if err != nil {
		return err
	}

	if err := s.ensureLoadBalancerACL(ctx, machineIPs.External); err != nil {
		return err
	}

	if err := s.ensureCloudInit(ctx, server, machineIPs); err != nil {
		return err
	}

	if err := s.ensureServerStarted(ctx, server); err != nil {
		return err
	}

	s.ScalewayMachine.Spec.ProviderID = scw.StringPtr(s.ProviderID(server.ID))

	s.ScalewayMachine.Status.Addresses = []v1beta1.MachineAddress{}

	if machineIPs.External != nil {
		s.ScalewayMachine.Status.Addresses = append(s.ScalewayMachine.Status.Addresses, v1beta1.MachineAddress{
			Type:    v1beta1.MachineExternalIP,
			Address: *machineIPs.External,
		})
	}

	if machineIPs.Internal != nil {
		s.ScalewayMachine.Status.Addresses = append(s.ScalewayMachine.Status.Addresses, v1beta1.MachineAddress{
			Type:    v1beta1.MachineInternalIP,
			Address: *machineIPs.Internal,
		})
	}

	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	server, err := s.ScalewayClient.FindInstanceByName(ctx, s.Zone(), s.Name())
	if err != nil {
		if errors.Is(err, client.ErrNoItemFound) {
			return nil
		}

		return err
	}

	// Set publicIP to nil to force deletion.
	if err := s.ensureLoadBalancerACL(ctx, nil); err != nil && !errors.Is(err, client.ErrNoItemFound) {
		return err
	}

	// Remove this control-plane from the loadbalancer.
	if util.IsControlPlaneMachine(s.Machine.Machine) {
		var pnic *instance.PrivateNIC

		if s.HasPrivateNetwork() {
			pnID, err := s.PrivateNetworkID()
			if err != nil {
				return err
			}

			pnic, err = s.ScalewayClient.FindPrivateNICByPNID(ctx, server, pnID)
			if err != nil {
				return fmt.Errorf("failed to find PrivateNIC by PNID: %w", err)
			}
		}

		_, err := s.ensureControlPlaneLoadBalancer(ctx, server, pnic, true)
		if err != nil && !errors.Is(err, client.ErrNoItemFound) {
			return err
		}
	}

	// Delete flexible IP.
	if server.PublicIP != nil && !server.PublicIP.Dynamic {
		if err := s.ScalewayClient.Instance.DeleteIP(&instance.DeleteIPRequest{
			Zone: server.Zone,
			IP:   server.PublicIP.ID,
		}); err != nil {
			return err
		}
	}

	switch server.State {
	case instance.ServerStateRunning, instance.ServerStateStoppedInPlace:
		if _, err := s.ScalewayClient.Instance.ServerAction(&instance.ServerActionRequest{
			Zone:     server.Zone,
			ServerID: server.ID,
			Action:   instance.ServerActionPoweroff,
		}); err != nil {
			return fmt.Errorf("failed to poweroff server: %w", err)
		}

		return ErrInstanceNotStoppedYet
	case instance.ServerStateStopping:
		return ErrInstanceNotStoppedYet
	case instance.ServerStateLocked:
		return errors.New("instance is locked")
	}

	// Remove boot volume.
	if v, ok := server.Volumes["0"]; ok && v.Boot {
		if _, err := s.ScalewayClient.Instance.DetachVolume(&instance.DetachVolumeRequest{
			Zone:     server.Zone,
			VolumeID: v.ID,
		}); err != nil {
			return fmt.Errorf("failed to detach boot volume: %w", err)
		}

		if err := s.ScalewayClient.Instance.DeleteVolume(&instance.DeleteVolumeRequest{
			Zone:     server.Zone,
			VolumeID: v.ID,
		}); err != nil {
			return fmt.Errorf("failed to delete instance volume: %w", err)
		}
	}

	if err := s.ScalewayClient.Instance.DeleteServer(&instance.DeleteServerRequest{
		Zone:     server.Zone,
		ServerID: server.ID,
	}); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return nil
}
