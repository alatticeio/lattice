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

package ipam

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/alatticeio/lattice/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type IPAM struct {
	client client.Client
}

func NewIPAM(client client.Client) *IPAM {
	return &IPAM{client: client}
}

// AllocateSubnet allocate a subnet for new network
func (m *IPAM) AllocateSubnet(ctx context.Context, networkName string, pool *v1alpha1.LatticeGlobalIPPool) (string, error) {
	const maxRetries = 10
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Re-query on every attempt to observe new allocations made by concurrent requests.
		ip, err := m.FindFirstFree(ctx, pool)
		if err != nil {
			return "", err
		}

		subnetCIDR := fmt.Sprintf("%s/%d", ip.String(), pool.Spec.SubnetMask)
		subnetName := fmt.Sprintf("subnet-%s", ipToHex(ip))

		alloc := &v1alpha1.LatticeSubnetAllocation{
			ObjectMeta: metav1.ObjectMeta{
				Name: subnetName,
			},
			Spec: struct {
				NetworkName string `json:"networkName"`
				CIDR        string `json:"cidr"`
			}{
				NetworkName: networkName,
				CIDR:        subnetCIDR,
			},
		}

		if err = controllerutil.SetControllerReference(pool, alloc, m.client.Scheme()); err != nil {
			return "", err
		}

		err = m.client.Create(ctx, alloc)
		if err == nil {
			// Created successfully — we won the race for this subnet.
			return subnetCIDR, nil
		}

		if !errors.IsAlreadyExists(err) {
			return "", err // Unexpected error.
		}
		// AlreadyExists: a concurrent request claimed this subnet; retry to find the next free one.
	}

	return "", fmt.Errorf("no available subnet in pool")
}

func (m *IPAM) FindFirstFree(ctx context.Context, pool *v1alpha1.LatticeGlobalIPPool) (net.IP, error) {

	ip, ipnet, err := net.ParseCIDR(pool.Spec.CIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid pool CIDR: %v", err)
	}

	// ip is the network base address (e.g. 10.0.0.0); normalise it with the mask.
	startIP := ip.Mask(ipnet.Mask)

	// 1. List all existing allocations from the Informer cache.
	var allAllocations v1alpha1.LatticeSubnetAllocationList
	if err = m.client.List(ctx, &allAllocations); err != nil {
		return nil, err
	}

	// 2. Build a set of occupied hex suffixes for O(1) lookup.
	used := make(map[string]struct{})
	for _, a := range allAllocations.Items {
		// Name format: subnet-<8-hex-digits>, e.g. subnet-0a0a0100
		hexStr := strings.TrimPrefix(a.Name, "subnet-")
		used[hexStr] = struct{}{}
	}

	// 3. Iterate subnets and return the first one not in the used set.
	for curr := startIP; ipnet.Contains(curr); curr = nextSubnet(curr, pool.Spec.SubnetMask) {
		if _, exists := used[ipToHex(curr)]; !exists {
			return curr, nil // 找到了回收后的空洞或全新的网段
		}
	}
	return nil, fmt.Errorf("no available subnet in pool")
}

func (m *IPAM) AllocateIP(ctx context.Context, network *v1alpha1.LatticeNetwork, peer *v1alpha1.LatticePeer) (string, error) {
	// 1. Parse the network's assigned CIDR (e.g. 10.10.1.0/24).
	ip, ipnet, err := net.ParseCIDR(network.Status.ActiveCIDR)
	if err != nil {
		return "", fmt.Errorf("invalid network CIDR: %v", err)
	}

	// 2. List all occupied IP objects in the peer's namespace (tenant scope).
	var existing v1alpha1.LatticeEndpointList
	if err := m.client.List(ctx, &existing, client.InNamespace(peer.Namespace)); err != nil {
		return "", err
	}

	used := make(map[string]struct{})
	for _, a := range existing.Items {
		// If this peer already has an endpoint (e.g. a previous status update
		// failed after the endpoint was created), reuse that address instead of
		// allocating a second one.
		if a.Spec.PeerRef == peer.Name {
			return a.Spec.Address, nil
		}
		used[a.Name] = struct{}{}
	}

	// 3. Find a free IP.
	// Start at network base + 2 (skip .0 network address and .1 gateway).
	startInt := ipToUint32(ip.Mask(ipnet.Mask)) + 2

	// End at broadcast - 1.
	ones, bits := ipnet.Mask.Size()
	totalIPs := uint32(1 << (bits - ones))
	endInt := ipToUint32(ip.Mask(ipnet.Mask)) + totalIPs - 2

	for i := startInt; i <= endInt; i++ {
		currentIP := uint32ToIP(i)
		hexName := fmt.Sprintf("ip-%s", ipToHex(currentIP))

		if _, ok := used[hexName]; ok {
			continue // Already in use.
		}

		// 4. Atomically claim the IP by creating its LatticeEndpoint.
		endpoint := &v1alpha1.LatticeEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hexName,
				Namespace: peer.Namespace,
			},
			Spec: v1alpha1.LatticeEndpointSpec{
				Address: currentIP.String(),
				PeerRef: peer.Name,
			},
		}

		if err := controllerutil.SetControllerReference(peer, endpoint, m.client.Scheme()); err != nil {
			return "", err
		}

		if err := m.client.Create(ctx, endpoint); err != nil {
			if errors.IsAlreadyExists(err) {
				continue // Another request claimed this address concurrently; try the next one.
			}
			return "", err
		}

		// Successfully claimed the IP.
		return currentIP.String(), nil
	}

	return "", fmt.Errorf("no available IP addresses in network %s", network.Name)
}

// ReleaseIP deletes the LatticeEndpoint that holds the peer's allocated address,
// returning the IP to the pool. Safe to call when AllocatedAddress is nil.
func (m *IPAM) ReleaseIP(ctx context.Context, namespace, allocatedAddress string) error {
	if allocatedAddress == "" {
		return nil
	}
	ip := net.ParseIP(allocatedAddress)
	if ip == nil {
		return nil
	}
	hexName := fmt.Sprintf("ip-%s", ipToHex(ip))
	endpoint := &v1alpha1.LatticeEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hexName,
			Namespace: namespace,
		},
	}
	if err := m.client.Delete(ctx, endpoint); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// 辅助函数：计算下一个子网地址
func nextSubnet(ip net.IP, maskBits int) net.IP {
	i := ipToUint32(ip)
	i += 1 << (32 - uint32(maskBits))
	return uint32ToIP(i)
}

// ipToHex converts a net.IP to an 8-character lowercase hex string.
func ipToHex(ip net.IP) string {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return ""
	}
	return hex.EncodeToString(ipv4)
}

// ipToUint32 converts a net.IP to a uint32 using big-endian byte order,
// ensuring that e.g. 1.0.0.0 compares greater than 0.255.255.255.
func ipToUint32(ip net.IP) uint32 {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ipv4)
}

// uint32ToIP converts a uint32 back to a net.IP (big-endian).
func uint32ToIP(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}
