package internal

import (
	"fmt"
	"linkany/pkg/log"
)

func SetRoute(*log.Logger) RouterPrintf {
	return func(action, address, name string) {
		// example: netsh interface ipv4 set address name="linkany-xx" static 192.168.1.10
		ip := TrimCIDR(address)
		gateway := GetGatewayFromIP(ip)

		ExecCommand("cmd", "/C", fmt.Sprintf(`route %s %s mask 255.255.255.0 %s`, action, ip, gateway))
	}
}

func SetDeviceIP() RouterPrintf {
	return func(action, address, name string) {
		switch action {
		case "add":
			ip := TrimCIDR(address)
			ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface ipv4 set address name="%s" static %s 255.255.255.0`, name, ip))
			ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface set interface name="%s" admin=ENABLED`, name))
		}
	}
}
