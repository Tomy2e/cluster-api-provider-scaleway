package v1beta1

import (
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
