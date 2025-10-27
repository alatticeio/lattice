package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Rule is a specification for a wireflow Rule resource
type Rule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status"`
}

// PolicySpec is the spec for a Policy resource
type RuleSpec struct {
	// name of network
	Name string `json:"name,omitempty"`

	Effect string `json:"effect,omitempty"`

	Owner string `json:"owner,omitempty"`
}

// RuleStatus is the status for a Rule resource
type RuleStatus struct {
	// Node status
	Status Status `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuleList is a list of Rule resources
type RuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Rule `json:"items"`
}
