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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PeerIdentitySpec defines the desired state of PeerIdentity.
type PeerIdentitySpec struct {
	// Network 是此 PeerIdentity 归属的 LatticeNetwork 名称。
	// (Network, metadata.name) 构成复合唯一键。
	Network string `json:"network"`

	// PeerRef 是当前绑定的 LatticePeer 名称。
	PeerRef string `json:"peerRef"`

	// PreviousPeerRef 在设备替换时设置，宽限期内旧设备与新设备同时有效。
	// +optional
	PreviousPeerRef string `json:"previousPeerRef,omitempty"`

	// GracePeriodSeconds 是宽限期时长，默认 300s。
	// 宽限期到期后 controller 自动清空 PreviousPeerRef。
	// +kubebuilder:default=300
	// +optional
	GracePeriodSeconds int32 `json:"gracePeriodSeconds,omitempty"`

	// Description 是人类可读的身份描述。
	// +optional
	Description string `json:"description,omitempty"`
}

// PeerIdentityStatus defines the observed state of PeerIdentity.
type PeerIdentityStatus struct {
	// ResolvedPeerIP 是当前 PeerRef 对应的 overlay IP（controller 维护，只读）。
	ResolvedPeerIP string `json:"resolvedPeerIP,omitempty"`

	// PreviousPeerIP 是宽限期内旧设备的 overlay IP。
	PreviousPeerIP string `json:"previousPeerIP,omitempty"`

	// GracePeriodExpiresAt 是宽限期截止时间。
	GracePeriodExpiresAt *metav1.Time `json:"gracePeriodExpiresAt,omitempty"`

	// Conditions 反映控制面同步状态。
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PeerIdentityConditionType 定义 PeerIdentity 条件类型
const (
	// PeerIdentityConditionPeerBound 表示 PeerRef 是否成功绑定到 LatticePeer
	PeerIdentityConditionPeerBound = "PeerBound"
	// PeerIdentityConditionGracePeriodActive 表示宽限期是否激活
	PeerIdentityConditionGracePeriodActive = "GracePeriodActive"
)

// PeerIdentityConditionReason 定义条件原因
const (
	PeerIdentityReasonPeerFound     = "PeerFound"
	PeerIdentityReasonPeerNotFound  = "PeerNotFound"
	PeerIdentityReasonPeerInvalid   = "PeerInvalid"
	PeerIdentityReasonGraceExpired  = "GraceExpired"
	PeerIdentityReasonGraceActive   = "GraceActive"
	PeerIdentityReasonNoGracePeriod = "NoGracePeriod"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=peerid
// +kubebuilder:printcolumn:name="NETWORK",type="string",JSONPath=".spec.network",description="The network this identity belongs to"
// +kubebuilder:printcolumn:name="PEER",type="string",JSONPath=".spec.peerRef",description="Current bound peer"
// +kubebuilder:printcolumn:name="RESOLVED-IP",type="string",JSONPath=".status.resolvedPeerIP",description="Resolved peer IP"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// PeerIdentity 是设备的稳定逻辑身份 CRD。
// 它将逻辑角色名（如 prod-db）绑定到具体的 LatticePeer，
// 实现策略与网络拓扑的解耦。
type PeerIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeerIdentitySpec   `json:"spec,omitempty"`
	Status PeerIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PeerIdentityList contains a list of PeerIdentity.
type PeerIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeerIdentity{}, &PeerIdentityList{})
}
