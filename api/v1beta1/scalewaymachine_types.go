package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const MachineFinalizer = "scalewaymachine.infrastructure.cluster.x-k8s.io"

// ScalewayMachineSpec defines the desired state of ScalewayMachine
type ScalewayMachineSpec struct {
	// TODO: enforce immutable field(s)

	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Label (e.g. ubuntu_jammy) or UUID of an image that will be used to
	// create the instance.
	Image string `json:"image"`

	// Type of instance (e.g. PRO2-S).
	Type string `json:"type"`

	// Size of the root volume in GB. Defaults to 20 GB.
	// +optional
	RootVolumeSize *int64 `json:"rootVolumeSize,omitempty"`

	// Set to true to create and attach a public IP to the instance.
	// Defaults to false.
	// +optional
	PublicIP *bool `json:"publicIP,omitempty"`
}

// ScalewayMachineStatus defines the observed state of ScalewayMachine
type ScalewayMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses of the node.
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ScalewayMachine is the Schema for the scalewaymachines API
type ScalewayMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalewayMachineSpec   `json:"spec,omitempty"`
	Status ScalewayMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScalewayMachineList contains a list of ScalewayMachine
type ScalewayMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalewayMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalewayMachine{}, &ScalewayMachineList{})
}
