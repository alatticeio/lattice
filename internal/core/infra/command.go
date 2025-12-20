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
	"os/exec"
	"strings"
)

func ExecCommand(name string, commands ...string) error {
	cmd := exec.Command(name, commands...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Print(string(output))
	return nil
}

type CommandExecutor interface {
	ExecCommand(...string) error
}

type linuxExecutor struct {
}

func NewLinuxExecutor() CommandExecutor {
	return &linuxExecutor{}
}

func (executor *linuxExecutor) ExecCommand(args ...string) error {
	for _, arg := range args {
		return ExecCommand("/bin/sh", "-c", arg)
	}
	return nil
}

type windowsExecutor struct{}

func NewWindowsExecutor() CommandExecutor {
	return &windowsExecutor{}
}

func (executor *windowsExecutor) ExecCommand(args ...string) error {
	for _, arg := range args {
		return ExecCommand(arg)
	}

	return nil
}

type macExecutor struct{}

func NewMacExecutor() CommandExecutor {
	return &macExecutor{}
}

func (executor *macExecutor) ExecCommand(args ...string) error {
	for _, arg := range args {
		return ExecCommand("/bin/sh", "-c", arg)
	}
	return nil
}

// Platform 类型常量，用于避免字符串错误
const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformMacOS   = "darwin"
	// 可以在此添加更多平台，如 FreeBSD, Android等
)

// NewRuleGenerator 是工厂函数，根据平台名称返回相应的 RuleGenerator 实例。
func NewExecutor(platform string) (CommandExecutor, error) {
	// 将输入转换为小写，确保健壮性
	p := strings.ToLower(platform)

	switch p {
	case PlatformLinux:
		// 返回 Linux/iptables 的生成器实例
		return &linuxExecutor{}, nil

	case PlatformWindows:
		// 返回 Windows/PowerShell 的生成器实例
		return &windowsExecutor{}, nil

	case PlatformMacOS:
		// 如果您决定实现 macOS 的 pf/ipfw 生成器，可以在这里返回
		return &macExecutor{}, nil

	default:
		return nil, fmt.Errorf("unsupported platform type: %s", platform)
	}
}
