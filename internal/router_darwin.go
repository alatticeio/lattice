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
