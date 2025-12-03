// Copyright 2025 wireflowio.com, Inc.
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

// example: route add -net 5.244.24.0/24 dev wireflow-xx
func SetRoute(logger *log.Logger) RouterPrintf {
	return func(action, address, name string) {
		cidr := GetCidrFromIP(address)
		switch action {
		case "add":
			//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address add dev %s %s", name, address))
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE", name, name))
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
			logger.Infof("add route %s -net %v dev %s", action, cidr, name)
		case "delete":
			ExecCommand("/bin/sh", "-c", fmt.Sprintf("route %s -net %v dev %s", action, cidr, name))
			logger.Infof("delete route %s -net %v dev %s", action, cidr, name)
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
