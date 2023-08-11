package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScalewayMachineTemplateSpec defines the desired state of ScalewayMachineTemplate
type ScalewayMachineTemplateSpec struct {
	Template ScalewayMachineTemplateResource `json:"template"`
}

type ScalewayMachineTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta metav1.ObjectMeta   `json:"metadata,omitempty"`
	Spec       ScalewayMachineSpec `json:"spec"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=scalewaymachinetemplates,scope=Namespaced,categories=cluster-api,shortName=smt
//+kubebuilder:storageversion

// ScalewayMachineTemplate is the Schema for the scalewaymachinetemplates API
type ScalewayMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScalewayMachineTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ScalewayMachineTemplateList contains a list of ScalewayMachineTemplate
type ScalewayMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalewayMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalewayMachineTemplate{}, &ScalewayMachineTemplateList{})
}
