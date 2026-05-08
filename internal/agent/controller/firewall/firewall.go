package firewall

import (
	"fmt"
	"runtime"
	"strings"
)

// Rule defines an abstract firewall rule
type Rule struct {
	RemoteIP string
	Port     int
	Protocol string // "tcp" or "udp"
}

// GenerateCommands generates firewall commands for the current operating system
func GenerateCommands(rules []Rule) ([]string, error) {
	var cmds []string

	switch runtime.GOOS {
	case "linux":
		// Use nftables; it is recommended to create a separate table for management
		cmds = append(cmds, "nft add table inet lattice")
		cmds = append(cmds, "nft add chain inet lattice ingress { type filter hook input priority 0; policy drop; }")
		for _, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"nft add rule inet lattice ingress ip saddr %s %s dport %d accept",
				r.RemoteIP, r.Protocol, r.Port,
			))
		}

	case "darwin": // macOS
		// macOS uses pfctl. Note: PF typically requires writing a config file first, then loading
		for _, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"echo 'pass in proto %s from %s to any port %d' | sudo pfctl -ef -",
				r.Protocol, r.RemoteIP, r.Port,
			))
		}

	case "windows":
		// Windows uses PowerShell's New-NetFirewallRule
		for i, r := range rules {
			cmds = append(cmds, fmt.Sprintf(
				"New-NetFirewallRule -DisplayName 'Lattice-%d' -Direction Inbound -Protocol %s -LocalPort %d -RemoteAddress %s -Action Allow",
				i, strings.ToUpper(r.Protocol), r.Port, r.RemoteIP,
			))
		}

	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmds, nil
}
