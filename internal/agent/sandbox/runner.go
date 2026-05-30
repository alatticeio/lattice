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

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// IsolationMode 定义隔离模式
type IsolationMode string

const (
	IsolationNone   IsolationMode = "none"
	IsolationGVisor IsolationMode = "gvisor"
	IsolationAuto   IsolationMode = "auto"
)

// Runner 负责运行沙箱
type Runner struct {
	installer *GVisorInstaller
	log       logr.Logger
}

// NewRunner 创建一个新的沙箱运行器
func NewRunner() *Runner {
	return &Runner{
		installer: NewGVisorInstaller(),
		log:       logf.Log.WithName("sandbox-runner"),
	}
}

// Run 运行命令，根据隔离模式选择运行方式
func (r *Runner) Run(isolation IsolationMode, args []string) error {
	switch isolation {
	case IsolationGVisor:
		return r.RunWithGVisor(args)
	case IsolationNone:
		return r.RunWithoutSandbox(args)
	case IsolationAuto:
		return r.RunWithAutoDetection(args)
	default:
		return fmt.Errorf("unknown isolation mode: %s", isolation)
	}
}

// RunWithGVisor 使用 gVisor 运行命令
func (r *Runner) RunWithGVisor(args []string) error {
	// 确保 gVisor 已安装
	runscPath, err := r.installer.EnsureInstalled()
	if err != nil {
		return fmt.Errorf("ensure gVisor installed: %w", err)
	}

	r.log.Info("Running with gVisor isolation", "runsc", runscPath, "args", args)

	// 构造 runsc 命令
	// runsc run --network=none --file-access=exclusive --hostname=lattice-sandbox <sandbox-id> <args...>
	sandboxID := "lattice-sandbox"
	runscArgs := []string{
		"run",
		"--network=none",          // 禁用 runsc 网络，使用 Lattice 的网络
		"--file-access=exclusive", // 独占文件访问
		"--hostname=" + sandboxID, // 设置主机名
		sandboxID,                 // 沙箱 ID
	}
	runscArgs = append(runscArgs, args...)

	cmd := exec.Command(runscPath, runscArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunWithoutSandbox 不使用沙箱运行命令
func (r *Runner) RunWithoutSandbox(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	r.log.Info("Running without sandbox", "args", args)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunWithAutoDetection 自动检测并使用最佳隔离模式
func (r *Runner) RunWithAutoDetection(args []string) error {
	// 检查 gVisor 是否可用
	if r.installer.IsInstalled() {
		r.log.Info("gVisor detected, using gVisor isolation")
		return r.RunWithGVisor(args)
	}

	// 检查系统是否有 runsc
	if _, err := exec.LookPath("runsc"); err == nil {
		r.log.Info("System gVisor detected, using gVisor isolation")
		return r.RunWithGVisor(args)
	}

	// 回退到无沙箱模式
	r.log.Info("gVisor not found, running without sandbox")
	return r.RunWithoutSandbox(args)
}

// GetIsolationMode 从字符串解析隔离模式
func GetIsolationMode(s string) (IsolationMode, error) {
	switch strings.ToLower(s) {
	case "none", "":
		return IsolationNone, nil
	case "gvisor":
		return IsolationGVisor, nil
	case "auto":
		return IsolationAuto, nil
	default:
		return IsolationNone, fmt.Errorf("unknown isolation mode: %s", s)
	}
}
