package v1

import (
	igntypes "github.com/coreos/ignition/config/v2_2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MachineConfigSpec defines the desired state of MachineConfig
// +k8s:openapi-gen=true
type MachineConfigSpec struct {
	// OSImageURL specifies the remote location that will be used to
	// fetch the OS.
	OSImageURL string `json:"osImageURL"`
	// Config is a Ignition Config object.
	Config igntypes.Config `json:"config"`

	KernelArguments []string `json:"kernelArguments"`

	FIPS bool `json:"fips"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineConfig is the Schema for the machineconfigs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=machineconfigs,scope=Namespaced
type MachineConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineConfigSpec   `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineConfigList contains a list of MachineConfig
type MachineConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineConfig{}, &MachineConfigList{})
}
