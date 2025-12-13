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

// NetworkPolicySpec defines the desired state of NetworkPolicy. which used to control the wireflow's traffic flow.
type NetworkPolicySpec struct {
	// NodeSelector is a label query over node that should be applied to the wireflow policy.
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`

	PolicyType PolicyType `json:"policyType,omitempty"`

	IngressRule []IngressRule `json:"ingressRule,omitempty"`
	EgressRule  []EgressRule  `json:"egressRule,omitempty"`
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

// NetworkPolicyStatus defines the observed state of NetworkPolicy.
type NetworkPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NetworkPolicy is the Schema for the networkpolicies API.
// +kubebuilder:resource:shortName=wfpolicy;wfp
type NetworkPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkPolicySpec   `json:"spec,omitempty"`
	Status NetworkPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkPolicyList contains a list of NetworkPolicy.
type NetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkPolicy{}, &NetworkPolicyList{})
}
