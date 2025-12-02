package controller

import (
	"fmt"
	"net"
	"sync"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPAllocator IP 地址分配器
type IPAllocator struct {
	mu sync.Mutex
}

// NewIPAllocator 创建 IP 分配器
func NewIPAllocator() *IPAllocator {
	return &IPAllocator{}
}

// AllocatedIP 表示已分配的 IP 信息
type AllocatedIP struct {
	IP          string
	Node        string
	AllocatedAt metav1.Time
}

// AllocateIP 为节点分配 IP 地址
// 从 Network 的 CIDR 中分配一个未使用的 IP
func (a *IPAllocator) AllocateIP(network *wireflowcontrollerv1alpha1.Network, nodeName string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 解析 CIDR
	_, ipNet, err := net.ParseCIDR(network.Spec.CIDR)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %s: %v", network.Spec.CIDR, err)
	}

	// 获取已分配的 IP 集合
	allocatedIPs := make(map[string]bool)
	for _, allocated := range network.Status.AllocatedIPs {
		allocatedIPs[allocated.IP] = true
	}

	// 遍历 CIDR 中的所有可用 IP
	ip := incrementIP(ipNet.IP)
	for ipNet.Contains(ip) {
		ipStr := ip.String()

		// 跳过网络地址和广播地址
		if isNetworkOrBroadcast(ip, ipNet) {
			ip = incrementIP(ip)
			continue
		}

		// 检查 IP 是否已分配
		if !allocatedIPs[ipStr] {
			return ipStr, nil
		}

		ip = incrementIP(ip)
	}

	return "", fmt.Errorf("no available IP in CIDR %s", network.Spec.CIDR)
}

// ReleaseIP 释放 IP 地址
func (a *IPAllocator) ReleaseIP(network *wireflowcontrollerv1alpha1.Network, ip string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 从已分配列表中移除
	newAllocatedIPs := []wireflowcontrollerv1alpha1.IPAllocation{}
	for _, allocated := range network.Status.AllocatedIPs {
		if allocated.IP != ip {
			newAllocatedIPs = append(newAllocatedIPs, allocated)
		}
	}

	network.Status.AllocatedIPs = newAllocatedIPs
	return nil
}

// GetNodeIP 获取节点在指定网络中的 IP
func (a *IPAllocator) GetNodeIP(network *wireflowcontrollerv1alpha1.Network, nodeName string) string {
	for _, allocated := range network.Status.AllocatedIPs {
		if allocated.Node == nodeName {
			return allocated.IP
		}
	}
	return ""
}

// IsIPAllocated 检查 IP 是否已被分配
func (a *IPAllocator) IsIPAllocated(network *wireflowcontrollerv1alpha1.Network, ip string) bool {
	for _, allocated := range network.Status.AllocatedIPs {
		if allocated.IP == ip {
			return true
		}
	}
	return false
}

// CountAvailableIPs 计算可用 IP 数量
func (a *IPAllocator) CountAvailableIPs(network *wireflowcontrollerv1alpha1.Network) (int, error) {
	_, ipNet, err := net.ParseCIDR(network.Spec.CIDR)
	if err != nil {
		return 0, err
	}

	// 计算总 IP 数量
	ones, bits := ipNet.Mask.Size()
	totalIPs := 1 << uint(bits-ones)

	// 减去网络地址和广播地址
	usableIPs := totalIPs - 2

	// 减去已分配的 IP
	allocatedCount := len(network.Status.AllocatedIPs)

	available := usableIPs - allocatedCount
	if available < 0 {
		available = 0
	}

	return available, nil
}

// ValidateIP 验证 IP 是否在网络 CIDR 范围内
func (a *IPAllocator) ValidateIP(cidr, ip string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %v", err)
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	if !ipNet.Contains(parsedIP) {
		return fmt.Errorf("IP %s is not in CIDR %s", ip, cidr)
	}

	return nil
}

// incrementIP 递增 IP 地址
func incrementIP(ip net.IP) net.IP {
	// 复制 IP 以避免修改原始值
	newIP := make(net.IP, len(ip))
	copy(newIP, ip)

	// 从最后一个字节开始递增
	for i := len(newIP) - 1; i >= 0; i-- {
		newIP[i]++
		if newIP[i] != 0 {
			break
		}
	}

	return newIP
}

// isNetworkOrBroadcast 检查是否是网络地址或广播地址
func isNetworkOrBroadcast(ip net.IP, ipNet *net.IPNet) bool {
	// 网络地址检查
	if ip.Equal(ipNet.IP) {
		return true
	}

	// 广播地址检查
	broadcast := make(net.IP, len(ip))
	for i := range ip {
		broadcast[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}

	return ip.Equal(broadcast)
}

// GetIPRange 获取 CIDR 的 IP 范围信息
func (a *IPAllocator) GetIPRange(cidr string) (firstIP, lastIP string, total int, err error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", 0, err
	}

	// 计算第一个可用 IP (网络地址 + 1)
	firstIP = incrementIP(ipNet.IP).String()

	// 计算最后一个可用 IP (广播地址 - 1)
	broadcast := make(net.IP, len(ipNet.IP))
	for i := range ipNet.IP {
		broadcast[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}

	lastIPBytes := make(net.IP, len(broadcast))
	copy(lastIPBytes, broadcast)
	for i := len(lastIPBytes) - 1; i >= 0; i-- {
		lastIPBytes[i]--
		if lastIPBytes[i] != 255 {
			break
		}
	}
	lastIP = lastIPBytes.String()

	// 计算总数
	ones, bits := ipNet.Mask.Size()
	total = (1 << uint(bits-ones)) - 2

	return firstIP, lastIP, total, nil
}
