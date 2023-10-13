package v1beta1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const ClusterFinalizer = "scalewaycluster.infrastructure.cluster.x-k8s.io"

// ScalewayClusterSpec defines the desired state of ScalewayCluster
type ScalewayClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// FailureDomains is a list of failure domains where the control-plane nodes
	// and resources (loadbalancer, public gateway, etc.) will be created.
	// +optional
	FailureDomains []string `json:"failureDomains,omitempty"`

	// Region represents the region where the cluster will be hosted.
	Region string `json:"region"`

	// Network contains network related options for the cluster.
	// +optional
	Network *NetworkSpec `json:"network,omitempty"`

	// ControlPlaneLoadBalancer contains loadbalancer options.
	// +optional
	ControlPlaneLoadBalancer *LoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`

	// Name of the secret that contains the Scaleway client parameters.
	// The following keys must be set: accessKey, secretKey, projectID.
	// The following key is optional: apiURL.
	ScalewaySecretName string `json:"scalewaySecretName"`
}

// NetworkSpec defines network specific settings.
type NetworkSpec struct {
	// PrivateNetwork allows attaching machines of the cluster to a Private
	// Network.
	// +optional
	PrivateNetwork *PrivateNetworkSpec `json:"privateNetwork,omitempty"`

	// Use this field to create or use an existing Public Gateway and attach
	// it to the Private Network. Do not set this field if the Private Network
	// already has an attached Public Gateway.
	// +optional
	PublicGateway *PublicGatewaySpec `json:"publicGateway,omitempty"`

	// A list of security groups that will be created in all zones of the region
	// of the cluster. A security group can be referenced by its name in the
	// ScalewayMachine object. If a security group is in use by at least one
	// machine, it MUST NOT be removed from this list: remove the machines first.
	SecurityGroups []SecurityGroup `json:"securityGroups,omitempty"`
}

// SecurityGroup contains a name and inbound/outbound policies.
type SecurityGroup struct {
	// Name of the security group. Must be unique in a list of security groups.
	Name string `json:"name"`

	// Inbound policy. If not set, all inbound traffic is allowed.
	// +optional
	Inbound *SecurityGroupPolicy `json:"inbound,omitempty"`

	// Oubound policy. If not set, all outbound traffic is allowed.
	// +optional
	Outbound *SecurityGroupPolicy `json:"outbound,omitempty"`
}

// SecurityGroupPolicy defines a policy for inbound or outbound traffic.
type SecurityGroupPolicy struct {
	// Default policy. If unset, defaults to "Accept".
	// +optional
	Default *SecurityGroupAction `json:"default,omitempty"`

	// A list of rules for this policy.
	// +optional
	Rules []SecurityGroupRule `json:"rules,omitempty"`
}

// SecurityGroupRule is a rule for the security group policy.
type SecurityGroupRule struct {
	// Action to apply when the rule matches a packet.
	Action SecurityGroupAction `json:"action"`

	// Protocol family this rule applies to. Can be ANY, TCP, UDP or ICMP.
	// If unset, defaults to ANY.
	// +optional
	Protocol *SecurityGroupProtocol `json:"protocol,omitempty"`

	// Port or range of ports this rule applies to. Not applicable for ICMP or ANY.
	// +optional
	Ports *PortOrPortRange `json:"ports,omitempty"`

	// IP range this rule applies to. Defaults to 0.0.0.0/0.
	// +optional
	IPRange *string `json:"ipRange,omitempty"`
}

// PortOrPortRange is a string representation of a port or a port range (e.g. 0-1024).
type PortOrPortRange string

// ToRange returns the range of ports, with the first return value being the lower
// port (called "from") and the second return value being the higher port (called
// "to"). If only a single port is set, the higher port is nil.
func (p *PortOrPortRange) ToRange() (*uint32, *uint32, error) {
	if p == nil {
		return nil, nil, nil
	}

	s := strings.Split(string(*p), "-")

	switch len(s) {
	case 1:
		n, err := strconv.Atoi(s[0])
		if err != nil {
			return nil, nil, fmt.Errorf("port is not a valid number: %w", err)
		}

		return scw.Uint32Ptr(uint32(n)), nil, nil
	case 2:
		from, err := strconv.Atoi(s[0])
		if err != nil {
			return nil, nil, fmt.Errorf("'from' port is not a valid number: %w", err)
		}

		to, err := strconv.Atoi(s[1])
		if err != nil {
			return nil, nil, fmt.Errorf("'to' port is not a valid number: %w", err)
		}

		if to < from {
			return nil, nil, fmt.Errorf("invalid port range: 'from' is higher than 'to'")
		}

		return scw.Uint32Ptr(uint32(from)), scw.Uint32Ptr(uint32(to)), nil
	default:
		return nil, nil, fmt.Errorf("port or port range is not correctly formatted")
	}
}

// SecurityGroupProtocol is a network protocol.
type SecurityGroupProtocol string

const (
	// SecurityGroupProtocolANY matches a packet of any protocol.
	SecurityGroupProtocolANY SecurityGroupProtocol = "ANY"
	// SecurityGroupProtocolTCP matches a TCP packet.
	SecurityGroupProtocolTCP SecurityGroupProtocol = "TCP"
	// SecurityGroupProtocolUDP matches an UDP packet.
	SecurityGroupProtocolUDP SecurityGroupProtocol = "UDP"
	// SecurityGroupProtocolICMP matches an ICMP packet.
	SecurityGroupProtocolICMP SecurityGroupProtocol = "ICMP"
)

// ToInstance returns the instance SecurityGroupRuleProtocol that matches the
// SecurityGroupProtocol value of the pointer receiver. Defaults to SecurityGroupRuleProtocolANY
// if the value is nil.
func (s *SecurityGroupProtocol) ToInstance() (instance.SecurityGroupRuleProtocol, error) {
	if s == nil {
		return instance.SecurityGroupRuleProtocolANY, nil
	}

	switch *s {
	case SecurityGroupProtocolANY:
		return instance.SecurityGroupRuleProtocolANY, nil
	case SecurityGroupProtocolTCP:
		return instance.SecurityGroupRuleProtocolTCP, nil
	case SecurityGroupProtocolUDP:
		return instance.SecurityGroupRuleProtocolUDP, nil
	case SecurityGroupProtocolICMP:
		return instance.SecurityGroupRuleProtocolICMP, nil
	default:
		return "", fmt.Errorf("unknown security group protocol: %s", *s)
	}
}

// SecurityGroupAction is an action to apply when a packet matches a rule. It can
// also be used as a default policy.
type SecurityGroupAction string

const (
	// SecurityGroupActionDrop drops all matching packets.
	SecurityGroupActionDrop SecurityGroupAction = "Drop"
	// SecurityGroupActionAccept accepts all matching packets.
	SecurityGroupActionAccept SecurityGroupAction = "Accept"
)

// ToInstancePolicy returns the instance SecurityGroupPolicy that matches the
// SecurityGroupAction value of the pointer receiver. Defaults to SecurityGroupPolicyAccept
// if the value is nil.
func (s *SecurityGroupAction) ToInstancePolicy() (instance.SecurityGroupPolicy, error) {
	if s == nil {
		return instance.SecurityGroupPolicyAccept, nil
	}

	switch *s {
	case SecurityGroupActionAccept:
		return instance.SecurityGroupPolicyAccept, nil
	case SecurityGroupActionDrop:
		return instance.SecurityGroupPolicyDrop, nil
	default:
		return "", fmt.Errorf("unknown action: %s", *s)
	}
}

// ToInstancePolicy returns the instance SecurityGroupRuleAction that matches the
// SecurityGroupAction value of the pointer receiver. Defaults to SecurityGroupRuleActionAccept
// if the value is nil.
func (s *SecurityGroupAction) ToInstanceAction() (instance.SecurityGroupRuleAction, error) {
	if s == nil {
		return instance.SecurityGroupRuleActionAccept, nil
	}

	switch *s {
	case SecurityGroupActionAccept:
		return instance.SecurityGroupRuleActionAccept, nil
	case SecurityGroupActionDrop:
		return instance.SecurityGroupRuleActionDrop, nil
	default:
		return "", fmt.Errorf("unknown action: %s", *s)
	}
}

// PrivateNetworkSpec defines Private Network settings for the cluster.
type PrivateNetworkSpec struct {
	// Set to true to automatically attach machines to a Private Network.
	// The Private Network is automatically created if no existing Private
	// Network ID is provided.
	Enabled bool `json:"enabled"`

	// Set a Private Network ID to reuse an existing Private Network. This
	// Private Network must have DHCP enabled.
	// +optional
	ID *string `json:"id,omitempty"`

	// Optional subnet for the Private Network. Only used on newly created
	// Private Networks.
	// +optional
	Subnet *string `json:"subnet,omitempty"`
}

// PublicGatewaySpec defines Public Gateway settings for the cluster.
type PublicGatewaySpec struct {
	// Set to true to attach a Public Gateway to the Private Network.
	// The Public Gateway will automatically be created if no existing Public
	// Gateway ID is provided.
	Enabled bool `json:"enabled"`

	// ID of an existing Public Gateway that will be attached to the Private
	// Network. You should also specify the zone field.
	ID *string `json:"id,omitempty"`

	// Public Gateway commercial offer type.
	// +kubebuilder:default="VPC-GW-S"
	// +optional
	Type *string `json:"type,omitempty"`

	// IP to use when creating a Public Gateway.
	// +kubebuilder:validation:Format=ipv4
	// +optional
	IP *string `json:"ip,omitempty"`

	// Zone where to create the Public Gateway. Must be in the same region as the
	// cluster. Defaults to the first zone of the region.
	// +optional
	Zone *string `json:"zone,omitempty"`
}

// LoadBalancerSpec defines control-plane loadbalancer settings for the cluster.
type LoadBalancerSpec struct {
	// Zone where to create the loadbalancer. Must be in the same region as the
	// cluster. Defaults to the first zone of the region.
	// +optional
	Zone *string `json:"zone,omitempty"`

	// Load Balancer commercial offer type.
	// +kubebuilder:default="LB-S"
	// +optional
	Type *string `json:"type,omitempty"`

	// IP to use when creating a loadbalancer.
	// +kubebuilder:validation:Format=ipv4
	// +optional
	IP *string `json:"ip,omitempty"`

	// AllowedRanges allows to set a list of allowed IP ranges that can access
	// the cluster through the load balancer. When unset, all IP ranges are allowed.
	// To allow the cluster to work properly, public IPs of nodes and Public
	// Gateways will automatically be allowed. However, if this field is set,
	// you MUST manually allow IPs of the nodes of your management cluster.
	// +optional
	AllowedRanges []string `json:"allowedRanges,omitempty"`
}

// ScalewayClusterStatus defines the observed state of ScalewayCluster
type ScalewayClusterStatus struct {
	// Ready is true when all cloud resources are created and ready.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// List of failure domains for this cluster.
	FailureDomains clusterv1beta1.FailureDomains `json:"failureDomains,omitempty"`

	// Network status.
	// +optional
	Network *NetworkStatus `json:"network,omitempty"`
}

// NetworkStatus contains network status related data.
type NetworkStatus struct {
	// ID of the Private Network if available.
	// +optional
	PrivateNetworkID *string `json:"privateNetworkID,omitempty"`

	// ID of the Public Gateway if available.
	// +optional
	PublicGatewayID *string `json:"publicGatewayID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScalewayCluster is the Schema for the scalewayclusters API
type ScalewayCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalewayClusterSpec   `json:"spec,omitempty"`
	Status ScalewayClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScalewayClusterList contains a list of ScalewayCluster
type ScalewayClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalewayCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalewayCluster{}, &ScalewayClusterList{})
}
