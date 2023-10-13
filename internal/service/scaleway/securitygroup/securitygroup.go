package securitygroup

import (
	"context"
	"fmt"
	"net"

	"github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/scope"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Service struct {
	*scope.Cluster
}

func NewService(clusterScope *scope.Cluster) *Service {
	return &Service{clusterScope}
}

// defaultPolicy returns the default policy of the security group policy
func defaultPolicy(securityGroupPolicy *v1beta1.SecurityGroupPolicy) (instance.SecurityGroupPolicy, error) {
	if securityGroupPolicy == nil || securityGroupPolicy.Default == nil {
		return instance.SecurityGroupPolicyAccept, nil
	}

	return securityGroupPolicy.Default.ToInstancePolicy()
}

// compareRules compares a list of cluster security group rules with a list of
// instance security group rules. It returns true if both lists are equal.
func compareRules(a []v1beta1.SecurityGroupRule, b []*instance.SecurityGroupRule) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}

	for i := range a {
		expectedProtocol, err := a[i].Protocol.ToInstance()
		if err != nil {
			return false, err
		}

		if expectedProtocol != b[i].Protocol {
			return false, nil
		}

		expectedAction, err := a[i].Action.ToInstanceAction()
		if err != nil {
			return false, err
		}

		if expectedAction != b[i].Action {
			return false, nil
		}

		var expectedIPRange string
		if a[i].IPRange == nil {
			expectedIPRange = "0.0.0.0/0"
		} else {
			expectedIPRange = *a[i].IPRange
		}

		if expectedIPRange != b[i].IPRange.IPNet.String() {
			return false, nil
		}

		from, to, err := a[i].Ports.ToRange()
		if err != nil {
			return false, err
		}

		if from != b[i].DestPortFrom {
			return false, nil
		}

		if to != b[i].DestPortTo {
			return false, nil
		}
	}

	return true, nil
}

func toInstanceRequestRule(position uint32, rule v1beta1.SecurityGroupRule, direction instance.SecurityGroupRuleDirection) (*instance.SetSecurityGroupRulesRequestRule, error) {
	action, err := rule.Action.ToInstanceAction()
	if err != nil {
		return nil, err
	}

	protocol, err := rule.Protocol.ToInstance()
	if err != nil {
		return nil, err
	}

	from, to, err := rule.Ports.ToRange()
	if err != nil {
		return nil, err
	}

	ipRange := "0.0.0.0/0"
	if rule.IPRange != nil {
		ipRange = *rule.IPRange

	}

	_, ipNet, err := net.ParseCIDR(ipRange)
	if err != nil {
		return nil, err
	}

	return &instance.SetSecurityGroupRulesRequestRule{
		Action:       action,
		Protocol:     protocol,
		Direction:    direction,
		IPRange:      scw.IPNet{IPNet: *ipNet},
		DestPortFrom: from,
		DestPortTo:   to,
		Position:     position,
	}, nil
}

// ensureSecurityGroups ensures the provided security groups exist (or don't exist)
// and are up-to-date.
func (s *Service) ensureSecurityGroups(ctx context.Context, securityGroups []v1beta1.SecurityGroup) error {
	l := log.FromContext(ctx)

	// List existing SGs in all zones.
	existingSGs, err := s.ScalewayClient.Instance.ListSecurityGroups(&instance.ListSecurityGroupsRequest{
		Zone: scw.ZoneFrPar1,
		Tags: s.Tags(),
	}, scw.WithContext(ctx), scw.WithAllPages(), scw.WithZones(s.Zones(s.ScalewayClient.Instance.Zones())...))
	if err != nil {
		return fmt.Errorf("failed to list security groups: %w", err)
	}

	// Remove security groups that should not exist.
	for _, existingSG := range existingSGs.SecurityGroups {
		if !slices.ContainsFunc(securityGroups, func(sg v1beta1.SecurityGroup) bool {
			return s.SecurityGroupName(sg.Name) == existingSG.Name
		}) {
			if err := s.ScalewayClient.Instance.DeleteSecurityGroup(&instance.DeleteSecurityGroupRequest{
				Zone:            existingSG.Zone,
				SecurityGroupID: existingSG.ID,
			}, scw.WithContext(ctx)); err != nil {
				// TODO: catch error if SG is currently in use by some instances.
				return fmt.Errorf("failed to delete existing security group with ID %s: %w", existingSG.ID, err)
			}
		}
	}

	// Create/Update security groups in all zones.
	for _, sg := range securityGroups {
		for _, zone := range s.Zones(s.ScalewayClient.Instance.Zones()) {
			// Check if the SG exists.
			existingSGIndex := slices.IndexFunc(existingSGs.SecurityGroups, func(existingSG *instance.SecurityGroup) bool {
				return existingSG.Name == s.SecurityGroupName(sg.Name) && existingSG.Zone == zone
			})

			inboundDefaultPolicy, err := defaultPolicy(sg.Inbound)
			if err != nil {
				return err
			}

			outboundDefaultPolicy, err := defaultPolicy(sg.Outbound)
			if err != nil {
				return err
			}

			var instanceSG *instance.SecurityGroup
			if existingSGIndex == -1 {
				// Create the SG as it does not exist.
				newInstanceSG, err := s.ScalewayClient.Instance.CreateSecurityGroup(&instance.CreateSecurityGroupRequest{
					Zone:                  zone,
					Name:                  s.SecurityGroupName(sg.Name),
					Tags:                  s.Tags(),
					InboundDefaultPolicy:  inboundDefaultPolicy,
					OutboundDefaultPolicy: outboundDefaultPolicy,
					EnableDefaultSecurity: scw.BoolPtr(false),
					Stateful:              true,
				}, scw.WithContext(ctx))
				if err != nil {
					return fmt.Errorf("failed to create security group: %w", err)
				}

				instanceSG = newInstanceSG.SecurityGroup

				l.Info("security group was created", "securityGroupName", s.SecurityGroupName(sg.Name))
			} else {
				// Check if SG spec matches what is expected.
				instanceSG = existingSGs.SecurityGroups[existingSGIndex]

				if instanceSG.InboundDefaultPolicy != inboundDefaultPolicy ||
					instanceSG.OutboundDefaultPolicy != outboundDefaultPolicy ||
					instanceSG.EnableDefaultSecurity {
					if _, err := s.ScalewayClient.Instance.UpdateSecurityGroup(&instance.UpdateSecurityGroupRequest{
						Zone:                  zone,
						SecurityGroupID:       instanceSG.ID,
						InboundDefaultPolicy:  &inboundDefaultPolicy,
						OutboundDefaultPolicy: &outboundDefaultPolicy,
						EnableDefaultSecurity: scw.BoolPtr(false),
					}, scw.WithContext(ctx)); err != nil {
						return fmt.Errorf("failed to update security group: %w", err)
					}

					l.Info("security group was updated", "securityGroupName", s.SecurityGroupName(sg.Name))
				}
			}

			// Check if rules match what is expected.
			rules, err := s.ScalewayClient.Instance.ListSecurityGroupRules(&instance.ListSecurityGroupRulesRequest{
				Zone:            instanceSG.Zone,
				SecurityGroupID: instanceSG.ID,
			}, scw.WithContext(ctx), scw.WithAllPages())
			if err != nil {
				return fmt.Errorf("failed to list security group rules: %w", err)
			}

			var instanceInboundRules, instanceOutboundRules []*instance.SecurityGroupRule

			for _, rule := range rules.Rules {
				switch rule.Direction {
				case instance.SecurityGroupRuleDirectionInbound:
					instanceInboundRules = append(instanceInboundRules, rule)
				case instance.SecurityGroupRuleDirectionOutbound:
					instanceOutboundRules = append(instanceOutboundRules, rule)
				}
			}

			var inboundRules, outboundRules []v1beta1.SecurityGroupRule

			if sg.Inbound != nil {
				inboundRules = sg.Inbound.Rules
			}

			if sg.Outbound != nil {
				outboundRules = sg.Outbound.Rules
			}

			compareInbound, err := compareRules(inboundRules, instanceInboundRules)
			if err != nil {
				return fmt.Errorf("failed to compare inbound rules with instance inbound rules: %w", err)
			}

			compareOutbound, err := compareRules(outboundRules, instanceOutboundRules)
			if err != nil {
				return fmt.Errorf("failed to compare outbound rules with instance outbound rules: %w", err)
			}

			if !compareInbound || !compareOutbound {
				var newRules []*instance.SetSecurityGroupRulesRequestRule

				for i, rule := range inboundRules {
					newRule, err := toInstanceRequestRule(uint32(i+1), rule, instance.SecurityGroupRuleDirectionInbound)
					if err != nil {
						return err
					}

					newRules = append(newRules, newRule)
				}

				for i, rule := range outboundRules {
					newRule, err := toInstanceRequestRule(uint32(i+1), rule, instance.SecurityGroupRuleDirectionOutbound)
					if err != nil {
						return err
					}

					newRules = append(newRules, newRule)
				}

				if _, err := s.ScalewayClient.Instance.SetSecurityGroupRules(&instance.SetSecurityGroupRulesRequest{
					Zone:            zone,
					SecurityGroupID: instanceSG.ID,
					Rules:           newRules,
				}, scw.WithContext(ctx)); err != nil {
					return fmt.Errorf("failed to set security group rules: %w", err)
				}

				l.Info("security group rules were updated", "securityGroupName", s.SecurityGroupName(sg.Name))
			}
		}
	}

	return nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	var securityGroups []v1beta1.SecurityGroup
	if s.ScalewayCluster.Spec.Network != nil {
		securityGroups = s.ScalewayCluster.Spec.Network.SecurityGroups
	}

	return s.ensureSecurityGroups(ctx, securityGroups)
}

func (s *Service) Delete(ctx context.Context) error {
	return s.ensureSecurityGroups(ctx, nil)
}
