// Copyright 2026 The Lattice Authors, Inc.
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

package security

import (
	"fmt"
	"regexp"
	"strings"
)

// dangerousCommandPatterns 包含危险的命令模式
var dangerousCommandPatterns = []string{
	"rm\\s+-rf",
	"rm\\s+-fr",
	"dd\\s+if=",
	"mkfs",
	"chmod\\s+777",
	"curl\\s+.*\\|\\s*sh",
	"curl\\s+.*\\|\\s*bash",
	"wget\\s+.*\\|\\s*sh",
	"wget\\s+.*\\|\\s*bash",
	"eval\\s*\\(",
	"sudo",
	"su\\s+-",
	"passwd",
	"useradd",
	"userdel",
	"groupadd",
	"groupdel",
	"chown\\s+root",
	"kill\\s+-9",
	"pkill",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init\\s+0",
	"init\\s+6",
	"mkfs\\.ext[234]",
	"fdisk",
	"parted",
	"mount",
	"umount",
	"iptables",
	"ip\\s+route",
	"ifconfig",
	"dhclient",
	"systemctl",
	"service",
	"journalctl",
	"dmesg",
}

// CheckCommand 检查命令是否包含危险模式
func CheckCommand(cmd string) error {
	if cmd == "" {
		return nil
	}

	// 检查命令注入（多个命令连接）
	if err := checkCommandInjection(cmd); err != nil {
		return err
	}

	// 检查危险命令模式
	for _, pattern := range dangerousCommandPatterns {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			continue
		}

		if re.MatchString(cmd) {
			return fmt.Errorf("dangerous command detected: pattern '%s' found in command", pattern)
		}
	}

	return nil
}

// checkCommandInjection 检查命令注入攻击
func checkCommandInjection(cmd string) error {
	// 检查命令分隔符
	injectionPatterns := []string{
		";",
		"&&",
		"||",
		"|",
		"`",
		"$(",
		"${",
		"\n",
		"\r",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(cmd, pattern) {
			return fmt.Errorf("command injection detected: '%s' found in command", pattern)
		}
	}

	return nil
}
