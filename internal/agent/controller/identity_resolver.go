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

package controller

import (
	"context"
	"fmt"

	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// IdentityResolver 解析 PeerIdentity 到 IP 地址
type IdentityResolver struct {
	client client.Client
	log    logr.Logger
}

// NewIdentityResolver 创建一个新的 IdentityResolver（无 client，仅用于测试）
func NewIdentityResolver() *IdentityResolver {
	return &IdentityResolver{
		log: logf.Log.WithName("identity-resolver"),
	}
}

// NewIdentityResolverWithClient 创建一个带 client 的 IdentityResolver
func NewIdentityResolverWithClient(c client.Client) *IdentityResolver {
	return &IdentityResolver{
		client: c,
		log:    logf.Log.WithName("identity-resolver"),
	}
}

// ResolveIdentity 解析 PeerIdentity 名称到 IP 地址列表
// 返回 resolvedIP 和 previousIP（如果宽限期有效）
func (r *IdentityResolver) ResolveIdentity(ctx context.Context, network, identityName string) ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("identity resolver: client not initialized")
	}

	if identityName == "" {
		return nil, nil
	}

	// 查找 PeerIdentity
	peerIdentity := &v1alpha1.PeerIdentity{}
	key := types.NamespacedName{
		Name: identityName,
	}

	if err := r.client.Get(ctx, key, peerIdentity); err != nil {
		r.log.V(1).Info("PeerIdentity not found", "name", identityName, "error", err)
		return nil, nil // 安全失败：返回空列表
	}

	// 验证网络归属
	if peerIdentity.Spec.Network != network {
		r.log.V(1).Info("PeerIdentity belongs to different network",
			"name", identityName,
			"expectedNetwork", network,
			"actualNetwork", peerIdentity.Spec.Network)
		return nil, nil
	}

	var ips []string

	// 添加当前 peer IP
	if peerIdentity.Status.ResolvedPeerIP != "" {
		ips = append(ips, peerIdentity.Status.ResolvedPeerIP)
	}

	// 添加宽限期内的旧 peer IP
	if peerIdentity.Status.PreviousPeerIP != "" && peerIdentity.Status.GracePeriodExpiresAt != nil {
		ips = append(ips, peerIdentity.Status.PreviousPeerIP)
	}

	return ips, nil
}

// ResolveIdentities 批量解析多个 PeerIdentity 名称
func (r *IdentityResolver) ResolveIdentities(ctx context.Context, network string, identityNames []string) []string {
	var allIPs []string

	for _, name := range identityNames {
		ips, err := r.ResolveIdentity(ctx, network, name)
		if err != nil {
			r.log.Error(err, "Failed to resolve identity", "name", name)
			continue
		}
		allIPs = append(allIPs, ips...)
	}

	return allIPs
}
