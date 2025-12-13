package controller

import (
	"fmt"
	wireflowv1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal"
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
//func SpecEqual(old, new *wireflowcontrollerv1alpha1.NodeSpec) bool {
//	if old.Address != new.Address {
//		return false
//	}
//	if !stringSliceEqual(old.Network, new.Network) {
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

func transferToPeer(peer *wireflowv1alpha1.Node) *internal.Peer {
	return &internal.Peer{
		Name:       peer.Name,
		AppID:      peer.Spec.AppId,
		Address:    peer.Status.AllocatedAddress,
		PublicKey:  peer.Spec.PublicKey,
		AllowedIPs: fmt.Sprintf("%s/32", peer.Status.AllocatedAddress),
	}
}
