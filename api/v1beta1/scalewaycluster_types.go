package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const ClusterFinalizer = "scalewaycluster.infrastructure.cluster.x-k8s.io"

// ScalewayClusterSpec defines the desired state of ScalewayCluster
type ScalewayClusterSpec struct {
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// TODO: enforce immutable field(s)

	FailureDomains []string `json:"failureDomains,omitempty"`

	Region string `json:"region"`

	// +optional
	Network NetworkSpec `json:"network"`

	// +optional
	ControlPlaneLoadBalancer *LoadBalancerSpec `json:"controlPlaneLoadBalancer"`

	ScalewaySecretName string `json:"scalewaySecretName"`
}

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
}

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

type PublicGatewaySpec struct {
	// Set to true to attach a Public Gateway to the Private Network.
	// The Public Gateway is automatically created if no existing Public Gateway
	// ID is provided.
	Enabled bool `json:"enabled"`
	// ID of an existing Public Gateway that will be attached to the Private
	// Network. You should also specify the zone field.
	ID *string `json:"id,omitempty"`
	// Public Gateway commercial offer type.
	// +kubebuilder:default="VPC-GW-S"
	// +optional
	Type string `json:"type,omitempty"`
	// ID of an existing IP.
	IPID *string `json:"ipID,omitempty"`
	// Zone where to create the Public Gateway. Must be in the same region as the
	// cluster. Defaults to the first zone of the region.
	// +optional
	Zone *string `json:"zone,omitempty"`
}

type LoadBalancerSpec struct {
	// Zone where to create the loadbalancer. Must be in the same region as the
	// cluster. Defaults to the first zone of the region.
	// +optional
	Zone *string `json:"zone,omitempty"`
	// Load Balancer commercial offer type.
	// +kubebuilder:default="LB-S"
	// +optional
	Type string `json:"type,omitempty"`
}

// ScalewayClusterStatus defines the observed state of ScalewayCluster
type ScalewayClusterStatus struct {
	// +kubebuilder:default=false
	Ready          bool                          `json:"ready"`
	FailureDomains clusterv1beta1.FailureDomains `json:"failureDomains,omitempty"`

	// +optional
	Network *NetworkStatus `json:"network,omitempty"`
}

type NetworkStatus struct {
	PrivateNetworkID *string `json:"privateNetworkID,omitempty"`
	PublicGatewayID  *string `json:"publicGatewayID,omitempty"`
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
