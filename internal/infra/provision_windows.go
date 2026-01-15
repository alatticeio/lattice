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

func (r *routeProvisioner) ApplyRoute(action, address, interfaceName string) error {
	//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
	// example: netsh interface ipv4 set address name="linkany-xx" static 192.168.1.10
	ip := TrimCIDR(address)
	gateway := GetGatewayFromIP(ip)

	ExecCommand("cmd", "/C", fmt.Sprintf(`route %s %s mask 255.255.255.0 %s`, action, ip, gateway))

	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		ip := TrimCIDR(address)
		ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface ipv4 set address name="%s" static %s 255.255.255.0`, name, ip))
		ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface set interface name="%s" admin=ENABLED`, name))
	}

	return nil
}

func (r *ruleProvisioner) ApplyRule(action, rule string) error {
	return nil
}
