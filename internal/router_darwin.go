package internal

import (
	"fmt"
	"wireflow/pkg/log"
)

func SetRoute(logger *log.Logger) RouterPrintf {
	return func(action, address, interfaceName string) {
		//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
		switch action {
		case "add":
			//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", interfaceName, address, address))
			rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
			ExecCommand("/bin/sh", "-c", rule)
			logger.Infof("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		case "delete":
			rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
			ExecCommand("/bin/sh", "-c", rule)
			logger.Infof("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		}

	}
}

func SetDeviceIP() RouterPrintf {
	return func(action, address, name string) {
		switch action {
		case "add":
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", name, address, address))

		}
	}
}
