package internal

import (
	"fmt"
	"wireflow/pkg/log"
)

// example: route add -net 5.244.24.0/24 dev wireflow-xx
func SetRoute(logger *log.Logger) RouterPrintf {
	return func(action, address, name string) {
		switch action {
		case "add":
			//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE", name, name))
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, GetCidrFromIP(address), name))
			logger.Infof("add route %s -net %v dev %s", action, GetCidrFromIP(address), name)
		case "delete":
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, GetCidrFromIP(address), name))
			logger.Infof("delete route %s -net %v dev %s", action, GetCidrFromIP(address), name)
		}

	}
}

func SetDeviceIP() RouterPrintf {
	return func(action, address, name string) {
		switch action {
		case "add":
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip link set dev %s up", name))
		}
	}
}
