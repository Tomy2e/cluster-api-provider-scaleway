package v1beta1

import (
	"fmt"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const MachineFinalizer = "scalewaymachine.infrastructure.cluster.x-k8s.io"

// ScalewayMachineSpec defines the desired state of ScalewayMachine
type ScalewayMachineSpec struct {
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

	// Type of the root volume. Can be local or block. Note that not all types
	// of instances support local volumes.
	// +kubebuilder:validation:Enum=local;block
	// +optional
	RootVolumeType *string `json:"rootVolumeType,omitempty"`

	// Set to true to create and attach a public IP to the instance.
	// Defaults to false.
	// +optional
	PublicIP *bool `json:"publicIP,omitempty"`

	// Name of the security group as specified in the ScalewayCluster object.
	// If not set, the instance will be attached to the default security group.
	// +optional
	SecurityGroupName *string `json:"securityGroupName,omitempty"`
}

// ScalewayRootVolumeType returns the volume type to use for the root volume.
func (s *ScalewayMachineSpec) ScalewayRootVolumeType() (instance.VolumeVolumeType, error) {
	if s.RootVolumeType == nil {
		return instance.VolumeVolumeTypeBSSD, nil
	}

	switch *s.RootVolumeType {
	case "local":
		return instance.VolumeVolumeTypeLSSD, nil
	case "block":
		return instance.VolumeVolumeTypeBSSD, nil
	default:
		return "", fmt.Errorf("unsupported root volume type: %s", *s.RootVolumeType)
	}
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
//+kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="Type of instance"

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
