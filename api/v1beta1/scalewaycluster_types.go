package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ScalewayClusterSpec defines the desired state of ScalewayCluster
type ScalewayClusterSpec struct {
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// ScalewayClusterStatus defines the observed state of ScalewayCluster
type ScalewayClusterStatus struct {
	Ready bool `json:"ready"`
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
