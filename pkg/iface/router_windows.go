package iface

import (
	"fmt"
	internal2 "linkany/internal"
)

func SetRoute() RouterPrintf {
	return func(action, address, name string) {
		// example: netsh interface ipv4 set address name="linkany-xx" static 192.168.1.10
		internal2.ExecCommand("cmd", "/C", fmt.Sprintf("netsh interface ipv4 set address name=\"%s\" static %s", name, address))
		internal2.ExecCommand("cmd", "/C", fmt.Sprintf("netsh interface set interface \"%s\" enable", name))
		// example: route add 192.168.1.0 mask 255.255.255.0 192.168.1.1
		internal2.ExecCommand("cmd", "/C", fmt.Sprintf("route %s %s mask %s %s", action, address, "255.255.255.0", internal2.GetGatewayFromIP(address)))
	}
}
