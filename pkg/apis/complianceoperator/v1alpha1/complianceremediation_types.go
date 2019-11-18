package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ComplianceRemediationSpec defines the desired state of ComplianceRemediation
// +k8s:openapi-gen=true
type ComplianceRemediationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Annotation string `json:"annotation,omitempty"`
}

// ComplianceRemediationStatus defines the observed state of ComplianceRemediation
// +k8s:openapi-gen=true
type ComplianceRemediationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComplianceRemediation is the Schema for the complianceremediations API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=complianceremediations,scope=Namespaced
type ComplianceRemediation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComplianceRemediationSpec   `json:"spec,omitempty"`
	Status ComplianceRemediationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComplianceRemediationList contains a list of ComplianceRemediation
type ComplianceRemediationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComplianceRemediation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComplianceRemediation{}, &ComplianceRemediationList{})
}
