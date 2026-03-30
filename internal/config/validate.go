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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configField 描述单个配置项的校验状态。
type configField struct {
	name       string
	value      string
	status     string // "OK" | "MISSING" | "DEFAULT"
	suggestion string
}

// ValidateAndReport 是具备"环境感知"能力的启动前校验函数。
//
// isServer=true（wireflowd 服务端）：
//   - SignalingURL 为空时自动设为 nats://127.0.0.1:4222 并打印 Info 日志。
//   - DatabaseDSN 为空时明确退化为当前目录的 wireflow.db (SQLite)。
//   - 忽略 ServerUrl / Token 校验，始终返回 nil。
//
// isServer=false（wireflow agent 客户端）：
//   - 严格校验 SignalingURL、ServerUrl、Token 均非空。
//   - TTY 环境：向 stderr 输出美化诊断报告后返回错误。
//   - 非 TTY（Docker/K8s/CI）：直接返回简洁错误字符串。
//
// 建议在各子命令的 RunE 阶段显式调用，而非 PersistentPreRunE，
// 避免对 --help / --version / completion 等只读子命令产生干扰。
func ValidateAndReport(cfg *Config, isServer bool) error {
	if isServer {
		return applyServerDefaults(cfg)
	}
	return runClientValidation(cfg)
}

// applyServerDefaults 为服务端（All-in-One）模式补全缺失配置，始终返回 nil。
func applyServerDefaults(cfg *Config) error {
	if cfg.SignalingURL == "" {
		cfg.SignalingURL = "nats://127.0.0.1:4222"
		fmt.Println("[config] All-in-One: signaling-url → nats://127.0.0.1:4222")
	}
	if cfg.Database.DSN == "" {
		wd, _ := os.Getwd()
		cfg.Database.DSN = filepath.Join(wd, "wireflow.db")
		cfg.Database.Driver = "sqlite"
		fmt.Printf("[config] All-in-One: database.dsn → %s\n", cfg.Database.DSN)
	}
	return nil
}

// runClientValidation 对 agent 模式执行严格的字段校验。
func runClientValidation(cfg *Config) error {
	fields := []configField{
		{name: "signaling-url", value: cfg.SignalingURL, suggestion: "--signaling-url nats://<HOST>:4222"},
		{name: "server-url", value: cfg.ServerUrl, suggestion: "--server-url http://<HOST>:8080"},
		{name: "token", value: cfg.Token, suggestion: "--token <TOKEN>"},
	}

	var missing []string
	for i := range fields {
		if fields[i].value == "" {
			fields[i].status = "MISSING"
			missing = append(missing, fields[i].name)
		} else {
			fields[i].status = "OK"
			fields[i].suggestion = "-"
		}
	}

	if len(missing) == 0 {
		return nil
	}

	//if isStderrTTY() {
	printDiagnostic(fields, missing)
	//}

	return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
}

// isStderrTTY reports whether stderr is connected to a terminal.
// nolint:unused
func isStderrTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ─── Pretty Print ──────────────────────────────────────────────────────────────

// nolint:unused
const boxWidth = 80 // 框内字符宽度（不含两侧 ║）

// printDiagnostic 采用更简洁的分段式布局，移除冗余边框。
func printDiagnostic(fields []configField, missing []string) {
	w := os.Stderr

	// 1. 标题头：使用加粗或简单的分隔符
	fmt.Fprintln(w, "\n--- WIREFLOW SETUP ASSISTANT (Agent Mode) ---")                                //nolint:errcheck
	fmt.Fprintf(w, "Error: Required configuration is missing. [Config: %s]\n\n", GetConfigFilePath()) //nolint:errcheck

	// 2. 配置状态表：简单的列对齐
	fmt.Fprintf(w, "%-20s %-12s %s\n", "COMPONENT", "STATUS", "SUGGESTION") //nolint:errcheck
	fmt.Fprintln(w, strings.Repeat("-", 60))                                //nolint:errcheck

	for _, f := range fields {
		statusStr := f.status
		// 如果是 MISSING，可以加个提示符号
		if f.status == "MISSING" {
			statusStr = "[MISSING]"
		}
		fmt.Fprintf(w, "%-20s %-12s %s\n", f.name, statusStr, f.suggestion) //nolint:errcheck
	}

	// 3. 修复引导：直接给出 Copy-Paste 命令
	fmt.Fprintln(w, "\n QUICK FIX:")                                                                                  //nolint:errcheck
	fmt.Fprintln(w, "   Run the following command to initialize:")                                                    //nolint:errcheck
	fmt.Fprintf(w, "   %s\n", "wireflow up --signaling-url <NATS_URL> --server-url <API_URL> --token <TOKEN> --save") //nolint:errcheck

	// 4. 环境说明：简短的结语
	fmt.Fprintln(w, "\n To use environment variables instead, check the documentation.") //nolint:errcheck
}
