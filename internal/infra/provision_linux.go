// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infra

import "fmt"

func (r *routeProvisioner) ApplyRoute(action, address, name string) error {
	cidr := GetCidrFromIP(address)
	switch action {
	case "add":
		//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("iptables -A FORWARD -i %s -j ACCEPT; iptables -A FORWARD -o %s -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE", name, name))
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
		r.logger.Debug("add route", "cmd", fmt.Sprintf("add route %s -net %v dev %s", action, cidr, name))
	case "delete":
		ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
		r.logger.Debug("delete route", "cmd", fmt.Sprintf("delete route %s -net %v dev %s", action, cidr, name))
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
