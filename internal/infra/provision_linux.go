package infra

import "fmt"

func (r *routeProvisioner) ApplyRoute(action, address, name string) error {
	cidr := GetCidrFromIP(address)
	switch action {
	case "add":
		//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("iptables -A FORWARD -i %s -j ACCEPT; iptables -A FORWARD -o %s -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE", name, name))
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
		r.logger.Info("add route", "cmd", fmt.Sprintf("add route %s -net %v dev %s", action, cidr, name))
	case "delete":
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
		r.logger.Info("delete route", "cmd", fmt.Sprintf("delete route %s -net %v dev %s", action, cidr, name))
	}
	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip link set dev %s up", name))
	}

	return nil
}

func (r *ruleProvisioner) ApplyRule(action, rule string) error {
	return nil
}
