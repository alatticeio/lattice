// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PolicyType string

const (
	PolicyTypeIngress PolicyType = "ingress"
	PolicyTypeEgress  PolicyType = "egress"
)

// WireflowPolicySpec defines the desired state of WireflowPolicy. which used to control the wireflow's traffic flow.
type WireflowPolicySpec struct {
	//
	Network string `json:"network"`

	// PeerSelector is a label query over node that should be applied to the wireflow policy.
	PeerSelector metav1.LabelSelector `json:"peerSelector,omitempty"`

	Ingress []IngressRule `json:"ingress,omitempty"`

	Egress []EgressRule `json:"egress,omitempty"`

	// default DENY
	Action string `json:"action,omitempty"` // DENY / ALLOW
}

// IngressRule and EgressRule are used to control the wireflow's traffic flow.
type IngressRule struct {
	From  []PeerSelection     `json:"from,omitempty"` // from what peers connect to the wireflow which selected by this policy
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

type EgressRule struct {
	To    []PeerSelection     `json:"to,omitempty"` // to what peers connect to the wireflow which selected by this policy
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

type PeerSelection struct {
	PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
	IPBlock      *IPBlock              `json:"ipBlock,omitempty"`
}

type IPBlock struct {
	CIDR string `json:"cidr,omitempty"`
}

type NetworkPolicyPort struct {
	Port     int32  `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

// NetworkPolicyStatus defines the observed state of WireflowPolicy.
type NetworkPolicyStatus struct {
	// 策略当前匹配到的节点数量
	TargetNodes int `json:"targetNodes"`
	// 规则条数（Ingress + Egress）
	RuleCount int `json:"ruleCount"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WireflowPolicy is the Schema for the networkpolicies API.
// +kubebuilder:resource:shortName=wfpolicy
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.policyType",description="The type of the network policy (ingress or egress)"
// +kubebuilder:printcolumn:name="NODE-SELECTOR",type="string",JSONPath=".spec.nodeSelector",description="The selector to identify nodes this policy applies to"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="TARGETS",type="integer",JSONPath=".status.targetNodes",description="Number of nodes targeted by this policy"
// +kubebuilder:printcolumn:name="RULES",type="integer",JSONPath=".status.ruleCount",description="Number of rules defined in this policy"
type WireflowPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WireflowPolicySpec  `json:"spec,omitempty"`
	Status NetworkPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WireflowPolicyList contains a list of WireflowPolicy.
type WireflowPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WireflowPolicy{}, &WireflowPolicyList{})
}
