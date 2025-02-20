package iface

import (
	"fmt"
	"k8s.io/klog/v2"
	"linkany/internal"
)

func SetRoute() RouterPrintf {
	return func(action, address, interfaceName string) {
		//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
		switch action {
		case "add":
			internal.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", interfaceName, address, address))
			rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
			internal.ExecCommand("/bin/sh", "-c", rule)
			klog.Infof("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		case "delete":
			rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
			internal.ExecCommand("/bin/sh", "-c", rule)
			klog.Infof("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		}

	}
}
