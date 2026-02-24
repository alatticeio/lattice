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

package ipam

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"wireflow/api/v1alpha1"

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
func (m *IPAM) AllocateSubnet(ctx context.Context, networkName string, pool *v1alpha1.WireflowGlobalIPPool) (string, error) {
	ip, err := m.FindFirstFree(ctx, pool)
	if err != nil {
		return "", err
	}

	subnetCIDR := fmt.Sprintf("%s/%d", ip.String(), pool.Spec.SubnetMask)
	subnetName := fmt.Sprintf("subnet-%s", ipToHex(ip))

	// 3. 尝试原子创建索引对象
	alloc := &v1alpha1.WireflowSubnetAllocation{
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
		// 创建成功，意味着我们抢到了这个段
		return subnetCIDR, nil
	}

	if !errors.IsAlreadyExists(err) {
		return "", err // 发生了其他错误
	}
	// 如果 AlreadyExists，说明该段已被占用，循环继续尝试下一个

	return "", fmt.Errorf("no available subnet in pool")
}

func (m *IPAM) FindFirstFree(ctx context.Context, pool *v1alpha1.WireflowGlobalIPPool) (net.IP, error) {

	ip, ipnet, err := net.ParseCIDR(pool.Spec.CIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid pool CIDR: %v", err)
	}

	// 这里的 ip 实际上就是起始地址 (10.0.0.0)
	startIP := ip.Mask(ipnet.Mask)

	// 1. 从 Informer 缓存获取所有现有的分配
	var allAllocations v1alpha1.WireflowSubnetAllocationList
	if err = m.client.List(ctx, &allAllocations); err != nil {
		return nil, err
	}

	// 2. 将已占用的 Hex 后缀存入 Map
	used := make(map[string]struct{})
	for _, a := range allAllocations.Items {
		// 假设名称格式是 subnet-0a0a0100
		hexStr := strings.TrimPrefix(a.Name, "subnet-")
		used[hexStr] = struct{}{}
	}

	// 3. 迭代计算，遇到不在 used Map 里的第一个地址就返回
	for curr := startIP; ipnet.Contains(curr); curr = nextSubnet(curr, pool.Spec.SubnetMask) {
		if _, exists := used[ipToHex(curr)]; !exists {
			return curr, nil // 找到了回收后的空洞或全新的网段
		}
	}
	return nil, fmt.Errorf("no available subnet in pool")
}

func (m *IPAM) AllocateIP(ctx context.Context, network *v1alpha1.WireflowNetwork, peer *v1alpha1.WireflowPeer) (string, error) {
	// 1. 解析 Network 分配到的网段 (例如 10.10.1.0/24)
	ip, ipnet, err := net.ParseCIDR(network.Status.ActiveCIDR)
	if err != nil {
		return "", fmt.Errorf("invalid network CIDR: %v", err)
	}

	// 2. 获取该租户(Namespace)内已占用的 IP 对象
	var existing v1alpha1.WireflowEndpointList
	if err := m.client.List(ctx, &existing, client.InNamespace(peer.Namespace)); err != nil {
		return "", err
	}

	used := make(map[string]struct{})
	for _, a := range existing.Items {
		// peerName
		used[a.Name] = struct{}{}
	}

	// 3. 寻找空闲 IP
	// 起始点：网络地址 + 2 (跳过 .0 和 .1 网关)
	startInt := ipToUint32(ip.Mask(ipnet.Mask)) + 2

	// 结束点：广播地址 - 1
	ones, bits := ipnet.Mask.Size()
	totalIPs := uint32(1 << (bits - ones))
	endInt := ipToUint32(ip.Mask(ipnet.Mask)) + totalIPs - 2

	for i := startInt; i <= endInt; i++ {
		currentIP := uint32ToIP(i)
		hexName := fmt.Sprintf("ip-%s", ipToHex(currentIP))

		if _, ok := used[hexName]; ok {
			continue // 已占用
		}

		// 4. 原子创建 IPAllocation
		endpoint := &v1alpha1.WireflowEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hexName,
				Namespace: peer.Namespace,
			},
			Spec: v1alpha1.WireflowEndpointSpec{
				Address: currentIP.String(),
				PeerRef: peer.Name,
			},
		}

		if err := controllerutil.SetControllerReference(peer, endpoint, m.client.Scheme()); err != nil {
			return "", err
		}

		if err := m.client.Create(ctx, endpoint); err != nil {
			if errors.IsAlreadyExists(err) {
				continue // 刚才被别的并发请求抢走了，尝试下一个
			}
			return "", err
		}

		// 成功抢占 IP
		return currentIP.String(), nil
	}

	return "", fmt.Errorf("no available IP addresses in network %s", network.Name)
}

// 辅助函数：计算下一个子网地址
func nextSubnet(ip net.IP, maskBits int) net.IP {
	i := ipToUint32(ip)
	i += 1 << (32 - uint32(maskBits))
	return uint32ToIP(i)
}

// ipToHex 将 net.IP 转换为 8 位的十六进制字符串
func ipToHex(ip net.IP) string {
	// 确保处理的是 IPv4 的 4 字节表示
	ipv4 := ip.To4()
	if ipv4 == nil {
		return ""
	}
	// 使用 hex.EncodeToString 直接转换字节数组
	return hex.EncodeToString(ipv4)
}

// hexToIP 将 8 位十六进制字符串还原为 net.IP
// nolint:all
func hexToIP(h string) net.IP {
	bytes, err := hex.DecodeString(h)
	if err != nil || len(bytes) != 4 {
		return nil
	}
	return net.IP(bytes)
}

// ipToUint32 将 net.IP 转换为 uint32 数字
func ipToUint32(ip net.IP) uint32 {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	// 使用 BigEndian (大端序) 保证转换结果符合直觉
	// 例如 1.0.0.0 转换后大于 0.255.255.255
	return binary.BigEndian.Uint32(ipv4)
}

// uint32ToIP 将 uint32 数字还原为 net.IP
func uint32ToIP(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}
