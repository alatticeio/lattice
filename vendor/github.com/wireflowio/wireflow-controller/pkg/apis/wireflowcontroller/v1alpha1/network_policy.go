package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkPolicy is a specification for a wireflow NetworkPolicy resource
type NetworkPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkPolicySpec   `json:"spec"`
	Status NetworkPolicyStatus `json:"status"`
}

// NetworkPolicySpec is the spec for a NetworkPolicy resource
type NetworkPolicySpec struct {
	// name of network
	Name string `json:"name,omitempty"`

	// Allow / Deny
	Action string `json:"action,omitempty"`

	Owner string `json:"owner,omitempty"`

	Description string `json:"description,omitempty"`

	Priority int `json:"priority,omitempty"`

	// If true, rule evaluation is skipped
	Disabled bool `json:"disabled,omitempty"`

	Rules []Rule `json:"rules,omitempty"`
}

type Rule struct {
	Name string `json:"name,omitempty"`

	Action string `json:"action,omitempty"`

	Protocols string `json:"protocols,omitempty"`

	Source RuleSelector `json:"source,omitempty"`

	Destination RuleSelector `json:"destination,omitempty"`

	TimeWindow
}

type RuleSelector struct {
	NodeName []string `json:"nodeName,omitempty"`

	NodeSelector []metav1.LabelSelector `json:"nodeSelector,omitempty"`

	LabelSelctor []string `json:"labelSelector,omitempty"`

	IPBlocks []string `json:"ipBlocks,omitempty"`

	except []string `json:"except,omitempty"`

	// If true, Matched all resources
	Any bool `json:"any,omitempty"`
}

type TimeWindow struct {
	StartTime metav1.Time `json:"startTime,omitempty"`
	EndTime   metav1.Time `json:"endTime,omitempty"`
	Days      []string    `json:"days,omitempty"`
	TimeZone  string      `json:"timeZone,omitempty"`
	Cron      string      `json:"cron,omitempty"`
}

// NetworkPolicyStatus is the status for a Node resource
type NetworkPolicyStatus struct {
	// Node status
	Status Status `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkPolicyList is a list of NetworkPolicy resources
type NetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetworkPolicy `json:"items"`
}
