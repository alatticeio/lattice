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
	switch action {
	case "add":
		//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", interfaceName, address, address))
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command issued", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	case "delete":
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command command", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	}

	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		if err := ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", name, address, address)); err != nil {
			return err
		}

	}

	return nil
}

func (r *ruleProvisioner) ApplyRule(action, rule string) error {
	return nil
}

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	return nil
}
