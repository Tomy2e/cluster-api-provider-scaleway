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
	// +optional
	PrivateNetwork *PrivateNetworkSpec `json:"privateNetwork,omitempty"`
}

type PrivateNetworkSpec struct {
	Enabled bool `json:"enabled"`
	// Set the ID to reuse an existing PrivateNetwork.
	// +optional
	ID *string `json:"id,omitempty"`
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
