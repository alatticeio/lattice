package iface

import (
	"fmt"
	internal2 "linkany/internal"
)

// example: route add -net 5.244.24.0/24 dev linkany-xx
func SetRoute() RouterPrintf {
	return func(action, address, name string) {
		internal2.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
		internal2.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip link set dev %s up", name))
		internal2.ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, internal2.GetCidrFromIP(address), name))
	}
}
