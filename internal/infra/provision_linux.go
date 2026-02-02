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

import (
	"fmt"
	"log"
)

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

func (r *ruleProvisioner) Name() string {
	return "iptables"
}

func (r *ruleProvisioner) Provision(rule *FirewallRule) error {
	inChain := "WIREFLOW-INGRESS"
	outChain := "WIREFLOW-EGRESS"

	// 1. 初始化链
	p.initChain(inChain, "INPUT", "-i")
	p.initChain(outChain, "OUTPUT", "-o")

	// 2. 清空旧规则 (Flush)
	exec.Command("iptables", "-F", inChain).Run()
	exec.Command("iptables", "-F", outChain).Run()

	// 3. 基础规则：允许 Established 流量（零信任回包保障）
	exec.Command("iptables", "-A", inChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run()
	exec.Command("iptables", "-A", outChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run()

	// 4. 应用 Ingress (源地址匹配 -s)
	for _, tr := range rule.Ingress {
		for _, ip := range tr.Peers {
			if err := p.addRule(inChain, "-s", ip, tr); err != nil {
				return err
			}
		}
	}

	// 5. 应用 Egress (目的地址匹配 -d)
	for _, tr := range rule.Egress {
		for _, ip := range tr.Peers {
			if err := p.addRule(outChain, "-d", ip, tr); err != nil {
				return err
			}
		}
	}

	// 6. 终极封口 (DROP)
	exec.Command("iptables", "-A", inChain, "-j", "DROP").Run()
	exec.Command("iptables", "-A", outChain, "-j", "DROP").Run()

	return nil
}

// 内部辅助：确保链存在并挂载
func (p *ruleProvisioner) initChain(chain, parent, flag string) {
	exec.Command("iptables", "-N", chain).Run()
	// 检查是否已挂载，未挂载则插入到第一条
	if err := exec.Command("iptables", "-C", parent, flag, p.interfaceName, "-j", chain).Run(); err != nil {
		exec.Command("iptables", "-I", parent, 1, flag, p.interfaceName, "-j", chain).Run()
	}
}

// 内部辅助：添加单条规则
func (p *ruleProvisioner) addRule(chain, dir, ip string, tr infra.TrafficRule) error {
	args := []string{"-A", chain, dir, ip, "-p", strings.ToLower(tr.Protocol), "--dport", fmt.Sprintf("%d", tr.Port), "-j", "ACCEPT"}
	return exec.Command("iptables", args...).Run()
}

func (p *ruleProvisioner) Cleanup() error {
	// 逻辑：删除挂载点 -> 清空链 -> 删除链
	return nil
}

// SetupNAT for run via docker using.
func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	// 定义需要执行的命令集
	// -t nat -A POSTROUTING -o wf0 -j MASQUERADE: 允许流量从 wf0 出去时进行地址伪装
	// -A FORWARD -j ACCEPT: 允许通过容器进行流量转发

	cmds := []string{
		fmt.Sprintf("iptables -t nat -A POSTROUTING -o wf0 -j MASQUERADE\n"),
		fmt.Sprintf("iptables -A FORWARD -j ACCEPT\n"),
		fmt.Sprintf("iptables -A FORWARD -i wf0 -o eth0 -m state --state RELATED,ESTABLISHED -j ACCEPT"),
	}

	for _, args := range cmds {
		ExecCommand("/bin/sh", "-c", args)
	}

	log.Printf("Successfully configured iptables for %s", interfaceName)
	return nil
}
