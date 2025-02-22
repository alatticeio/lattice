package iface

import (
	"fmt"
	"linkany/internal"
	"linkany/pkg/log"
)

// example: route add -net 5.244.24.0/24 dev linkany-xx
func SetRoute(logger *log.Logger) RouterPrintf {
	return func(action, address, name string) {
		switch action {
		case "add":
			internal.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
			internal.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip link set dev %s up", name))
			internal.ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, internal.GetCidrFromIP(address), name))
			logger.Infof("add route %s -net %v dev %s", action, internal.GetCidrFromIP(address), name)
		case "delete":
			internal.ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, internal.GetCidrFromIP(address), name))
			logger.Infof("delete route %s -net %v dev %s", action, internal.GetCidrFromIP(address), name)
		}

	}
}
