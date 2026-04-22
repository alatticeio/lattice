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
	"k8s.io/apimachinery/pkg/runtime"
)

// WireflowRelayServerSpec defines the desired state of a WRRP relay server.
type WireflowRelayServerSpec struct {
	// DisplayName is the human-readable label shown in the management UI.
	DisplayName string `json:"displayName"`

	// Description is an optional free-text note.
	Description string `json:"description,omitempty"`

	// TcpUrl is the host:port of the TCP WRRP relay endpoint.
	// Corresponds to the node flag --wrrp-url.
	TcpUrl string `json:"tcpUrl"`

	// QuicUrl is the host:port of the QUIC WRRP relay endpoint.
	// Corresponds to --wrrp-quic-url. Preferred over TCP when set.
	QuicUrl string `json:"quicUrl,omitempty"`

	// Enabled controls whether this relay is pushed to nodes.
	// Disabled relays are not propagated; nodes retain their last-configured URLs
	// until a new enabled relay takes effect.
	Enabled bool `json:"enabled"`

	// Namespaces is the list of Kubernetes namespaces (workspace namespaces)
	// whose WireflowPeers should be configured to use this relay.
	// An empty list means all namespaces.
	Namespaces []string `json:"namespaces,omitempty"`
}

// WireflowRelayServerStatus defines the observed state of a WireflowRelayServer.
type WireflowRelayServerStatus struct {
	// Phase summarises the lifecycle state of the relay.
	Phase RelayPhase `json:"phase,omitempty"`

	// Health is the result of the most recent connectivity probe.
	Health RelayHealth `json:"health,omitempty"`

	// LatencyMs is the round-trip latency measured by the last probe, in milliseconds.
	LatencyMs *int64 `json:"latencyMs,omitempty"`

	// ConnectedPeers is the number of WireflowPeers currently configured to use this relay.
	ConnectedPeers int `json:"connectedPeers,omitempty"`

	// LastProbeTime is when the relay was last connectivity-tested.
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`

	// Conditions holds standard Kubernetes condition records.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// RelayPhase represents the lifecycle phase of a relay server.
type RelayPhase string

const (
	RelayPhasePending  RelayPhase = "Pending"
	RelayPhaseActive   RelayPhase = "Active"
	RelayPhaseDisabled RelayPhase = "Disabled"
)

// RelayHealth represents the last observed connectivity health.
type RelayHealth string

const (
	RelayHealthHealthy  RelayHealth = "Healthy"
	RelayHealthDegraded RelayHealth = "Degraded"
	RelayHealthOffline  RelayHealth = "Offline"
	RelayHealthUnknown  RelayHealth = "Unknown"
)

// Relay condition types.
const (
	RelayConditionReady  = "Ready"
	RelayConditionSynced = "PeersSynced"
)

// Relay finalizer – used to clear peer relay URLs before the CRD is removed.
const RelayFinalizer = "relay.wireflowcontroller.wireflow.run/finalizer"

// RelayPeerLabel is added to every WireflowPeer that is configured to use a
// given relay, keyed by the relay's metadata.Name.
const RelayPeerLabel = "relay.wireflowcontroller.wireflow.run/name"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=wfrelay
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="DISPLAY",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="HEALTH",type="string",JSONPath=".status.health"
// +kubebuilder:printcolumn:name="PEERS",type="integer",JSONPath=".status.connectedPeers"
// +kubebuilder:printcolumn:name="TCP",type="string",JSONPath=".spec.tcpUrl"
// +kubebuilder:printcolumn:name="ENABLED",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// WireflowRelayServer is the Schema for managing WRRP relay servers.
// It is cluster-scoped because relay infrastructure is shared across workspaces.
type WireflowRelayServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WireflowRelayServerSpec   `json:"spec,omitempty"`
	Status WireflowRelayServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WireflowRelayServerList contains a list of WireflowRelayServer.
type WireflowRelayServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireflowRelayServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WireflowRelayServer{}, &WireflowRelayServerList{})
}

// DeepCopyObject implements runtime.Object.
func (in *WireflowRelayServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of the WireflowRelayServer.
func (in *WireflowRelayServer) DeepCopy() *WireflowRelayServer {
	if in == nil {
		return nil
	}
	out := new(WireflowRelayServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all fields into out.
func (in *WireflowRelayServer) DeepCopyInto(out *WireflowRelayServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyInto copies all fields of WireflowRelayServerSpec into out.
func (in *WireflowRelayServerSpec) DeepCopyInto(out *WireflowRelayServerSpec) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy returns a deep copy of WireflowRelayServerSpec.
func (in *WireflowRelayServerSpec) DeepCopy() *WireflowRelayServerSpec {
	if in == nil {
		return nil
	}
	out := new(WireflowRelayServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all fields of WireflowRelayServerStatus into out.
func (in *WireflowRelayServerStatus) DeepCopyInto(out *WireflowRelayServerStatus) {
	*out = *in
	if in.LatencyMs != nil {
		x := *in.LatencyMs
		out.LatencyMs = &x
	}
	if in.LastProbeTime != nil {
		in, out := &in.LastProbeTime, &out.LastProbeTime
		*out = (*in).DeepCopy()
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a deep copy of WireflowRelayServerStatus.
func (in *WireflowRelayServerStatus) DeepCopy() *WireflowRelayServerStatus {
	if in == nil {
		return nil
	}
	out := new(WireflowRelayServerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object for WireflowRelayServerList.
func (in *WireflowRelayServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of the WireflowRelayServerList.
func (in *WireflowRelayServerList) DeepCopy() *WireflowRelayServerList {
	if in == nil {
		return nil
	}
	out := new(WireflowRelayServerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all fields into out.
func (in *WireflowRelayServerList) DeepCopyInto(out *WireflowRelayServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WireflowRelayServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}
