// Copyright 2026 The Lattice Authors, Inc.
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

package dto

import "time"

// PolicySpec mirrors v1alpha1.LatticePolicySpec using plain Go types.
// JSON field names are identical so DB records and wire format remain compatible.
type PolicySpec struct {
	Network      string        `json:"network,omitempty"`
	PeerSelector LabelSelector `json:"peerSelector,omitempty"`
	Ingress      []IngressRule `json:"ingress,omitempty"`
	Egress       []EgressRule  `json:"egress,omitempty"`
	Action       string        `json:"action,omitempty"`
	ExpiresAt    *time.Time    `json:"expiresAt,omitempty"`
}

// LabelSelector mirrors metav1.LabelSelector.
type LabelSelector struct {
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement mirrors metav1.LabelSelectorRequirement.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// IngressRule mirrors v1alpha1.IngressRule.
type IngressRule struct {
	From  []PeerSelection     `json:"from,omitempty"`
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// EgressRule mirrors v1alpha1.EgressRule.
type EgressRule struct {
	To    []PeerSelection     `json:"to,omitempty"`
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// PeerSelection mirrors v1alpha1.PeerSelection.
type PeerSelection struct {
	PeerSelector *LabelSelector `json:"peerSelector,omitempty"`
	IPBlock      *IPBlock       `json:"ipBlock,omitempty"`
}

// IPBlock mirrors v1alpha1.IPBlock.
type IPBlock struct {
	CIDR string `json:"cidr,omitempty"`
}

// NetworkPolicyPort mirrors v1alpha1.NetworkPolicyPort.
type NetworkPolicyPort struct {
	Port     int32  `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}
