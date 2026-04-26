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

package controller

import (
	"fmt"
	"strconv"
	"strings"
	wireflowv1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal/infra"
)

const (
	// LabelGateway marks a peer as the designated workspace gateway for peering.
	LabelGateway = "wireflow.run/gateway"

	// LabelShadow marks a peer as a synthetic shadow peer managed by
	// NetworkPeeringReconciler. PeerReconciler skips shadow peers.
	LabelShadow = "wireflow.run/shadow"

	// AnnotationShadowAllowedIPs is set on shadow peers and contains the CIDR
	// of the remote network that should be routed through this peer.
	// Example: "10.0.1.0/24"
	AnnotationShadowAllowedIPs = "wireflow.run/shadow-allowed-ips"

	// AnnotationPeeringRoutePrefix is the prefix for per-peering route annotations
	// on gateway peers. The suffix is the WireflowNetworkPeering name.
	// Example: "wireflow.run/peering-route-ws-a-to-ws-b" = "10.0.2.0/24"
	// When other local peers build their WireGuard config, they see this gateway
	// with AllowedIPs expanded to include all annotated CIDRs.
	AnnotationPeeringRoutePrefix = "wireflow.run/peering-route-"

	// PeeringFinalizer is the finalizer added to WireflowNetworkPeering resources.
	PeeringFinalizer = "wireflow.run/peering-finalizer"
)

// 辅助函数
func stringSet(list []string) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}
	return set
}

// nolint:all
func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, exists := b[k]; !exists {
			return false
		}
	}
	return true
}

// nolint:all
func setsDifference(a, b map[string]struct{}) map[string]struct{} {
	diff := make(map[string]struct{}, len(a))
	if len(a) == 0 {
		return b
	}

	if len(b) == 0 {
		return a
	}
	for k := range a {
		if _, exists := b[k]; !exists {
			diff[k] = struct{}{}
		}
	}
	return diff
}

// nolint:all
func setsToSlice(set map[string]struct{}) []string {
	slice := make([]string, 0, len(set))
	for k := range set {
		slice = append(slice, k)
	}
	return slice
}

// SpecEqual 比较两个 Spec 是否相等
//func SpecEqual(old, new *wireflowcontrollerv1alpha1.WireflowPeerSpec) bool {
//	if old.Address != new.Address {
//		return false
//	}
//	if !stringSliceEqual(old.WireflowNetwork, new.WireflowNetwork) {
//		return false
//	}
//	// 根据需要添加其他字段比较
//	return true
//}

// nolint:all
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func transferToPeer(peer *wireflowv1alpha1.WireflowPeer) *infra.Peer {
	var peerID uint64
	if peer.Spec.PeerId != "" {
		peerID, _ = strconv.ParseUint(peer.Spec.PeerId, 10, 64)
	}
	p := &infra.Peer{
		PeerID:        peerID,
		Name:          peer.Name,
		AppID:         peer.Spec.AppId,
		Platform:      peer.Spec.Platform,
		InterfaceName: peer.Spec.InterfaceName,
		Address:       peer.Status.AllocatedAddress,
		PublicKey:     peer.Spec.PublicKey,
		Labels:        peer.GetLabels(),
	}

	if peer.Status.AllocatedAddress != nil {
		p.AllowedIPs = fmt.Sprintf("%s/32", *peer.Status.AllocatedAddress)
	}

	// Shadow peers carry the remote network CIDR in an annotation so that any
	// peer routing through them gets a route for the entire remote subnet.
	if shadowCIDR := peer.GetAnnotations()[AnnotationShadowAllowedIPs]; shadowCIDR != "" {
		if p.AllowedIPs != "" {
			p.AllowedIPs += "," + shadowCIDR
		} else {
			p.AllowedIPs = shadowCIDR
		}
	}

	// Gateway peers carry per-peering route annotations. When other local peers
	// include this gateway in their WireGuard config, they route the listed CIDRs
	// through the gateway's tunnel, enabling cross-workspace forwarding.
	var extraRoutes []string
	for k, v := range peer.GetAnnotations() {
		if strings.HasPrefix(k, AnnotationPeeringRoutePrefix) && v != "" {
			extraRoutes = append(extraRoutes, v)
		}
	}
	if len(extraRoutes) > 0 {
		extra := strings.Join(extraRoutes, ",")
		if p.AllowedIPs != "" {
			p.AllowedIPs += "," + extra
		} else {
			p.AllowedIPs = extra
		}
	}

	return p
}
