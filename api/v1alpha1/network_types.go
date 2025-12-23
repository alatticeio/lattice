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

// NetworkSpec defines the desired state of Network.
type NetworkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// name of network
	Name string `json:"name,omitempty"`

	NetworkId string `json:"networkId,omitempty"`

	Owner string `json:"owner,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	CIDR string `json:"cidr,omitempty"`

	Mtu int `json:"mtu,omitempty"`

	Dns DNSConfig `json:"dns,omitempty"`

	Nodes []string `json:"nodes,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Policies []string `json:"policies,omitempty"`
}

// NetworkStatus defines the observed state of Network.
type NetworkStatus struct {
	Phase NetworkPhase `json:"phase,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`

	ActiveCIDR string `json:"activeCIDR,omitempty"`

	// 已分配的 IP 列表
	AllocatedIPs []IPAllocation `json:"allocatedIPs,omitempty"`

	// 可用 IP 数量
	AvailableIPs int `json:"availableIPs,omitempty"`

	//加入的节点数量
	AddedNodes int `json:"addedNodes,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type NetworkPhase string

const (
	NetworkPhaseCreating NetworkPhase = "Pending"
	NetworkPhaseReady    NetworkPhase = "Ready"
	NetworkPhaseFailed   NetworkPhase = "Failed"
)

// IPAllocation IP 分配记录
type IPAllocation struct {
	IP          string      `json:"ip"`
	Node        string      `json:"node"`
	AllocatedAt metav1.Time `json:"allocatedAt"`
}

type DNSConfig struct {
	Enabled bool     `json:"enabled"`
	Servers []string `json:"servers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Network is the Schema for the networks API.
// +kubebuilder:resource:shortName=wfnet;wfnetwork
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="The current phase of the network"
// +kubebuilder:printcolumn:name="CIDR",type="string",JSONPath=".spec.cidr",description="The CIDR block of the network"
// +kubebuilder:printcolumn:name="NODES",type="integer",JSONPath=".status.addedNodes",description="Number of nodes in the network"
// +kubebuilder:printcolumn:name="IPS-AVAIL",type="integer",JSONPath=".status.availableIPs",description="Number of available IP addresses"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
