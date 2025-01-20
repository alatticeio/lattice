package internal

import "net"

func GetCidrFromIP(str string) string {

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	if err != nil {
		return ""
	}
	return ipNet.String()

}
