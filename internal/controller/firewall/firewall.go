package firewall

import (
	"fmt"
	"runtime"
	"strings"
)

// Rule 定义了抽象的过滤规则
type Rule struct {
	RemoteIP string
	Port     int
	Protocol string // "tcp" or "udp"
}

// GenerateCommands 根据操作系统生成对应的防火墙脚本
func GenerateCommands(rules []Rule) ([]string, error) {
	var cmds []string

	switch runtime.GOOS {
	case "linux":
		// 使用 nftables，建议先创建一个独立的 table 以便管理
		cmds = append(cmds, "nft add table inet wireflow")
		cmds = append(cmds, "nft add chain inet wireflow ingress { type filter hook input priority 0; policy drop; }")
		for _, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"nft add rule inet wireflow ingress ip saddr %s %s dport %d accept",
				r.RemoteIP, r.Protocol, r.Port,
			))
		}

	case "darwin": // macOS
		// macOS 使用 pfctl。注意：PF 通常需要先写入配置文件再 load
		for _, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"echo 'pass in proto %s from %s to any port %d' | sudo pfctl -ef -",
				r.Protocol, r.RemoteIP, r.Port,
			))
		}

	case "windows":
		// Windows 使用 PowerShell 的 New-NetFirewallRule
		for i, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"New-NetFirewallRule -DisplayName 'Wireflow-%d' -Direction Inbound -Protocol %s -LocalPort %d -RemoteAddress %s -Action Allow",
				i, strings.ToUpper(r.Protocol), r.Port, r.RemoteIP,
			))
		}

	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmds, nil
}
