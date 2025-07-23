package internal

import (
	"net"
	"strings"
)

func GetCidrFromIP(str string) string {

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	if err != nil {
		return ""
	}
	return ipNet.String()

}

func GetGatewayFromIP(str string) string {
	_, ipNet, err := net.ParseCIDR(str + "/24")
	if err != nil {
		return ""
	}
	return ipNet.IP.String()
}

func TrimCIDR(addr string) string {
	if idx := strings.Index(addr, "/"); idx > 0 {
		return addr[:idx]
	}
	return addr
}
