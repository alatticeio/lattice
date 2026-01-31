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
	wireflowv1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal/infra"
)

// 辅助函数
func stringSet(list []string) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}
	return set
}

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
	p := &infra.Peer{
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

	return p
}
