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
	"strings"
)

// Checker 提供工具调用参数的安全检查
type Checker interface {
	// CheckSQL 检查 SQL 语句
	CheckSQL(sql string) error

	// CheckPath 检查文件路径
	CheckPath(path string) error

	// CheckCommand 检查命令
	CheckCommand(cmd string) error

	// Check 根据工具类型检查参数
	Check(tool string, params map[string]any) error
}

type checker struct{}

// NewChecker 创建一个新的安全检查器
func NewChecker() Checker {
	return &checker{}
}

func (c *checker) CheckSQL(sql string) error {
	return CheckSQL(sql)
}

func (c *checker) CheckPath(path string) error {
	return CheckPath(path)
}

func (c *checker) CheckCommand(cmd string) error {
	return CheckCommand(cmd)
}

func (c *checker) Check(tool string, params map[string]any) error {
	if params == nil {
		return nil
	}

	switch {
	case strings.HasPrefix(tool, "db:") || strings.HasPrefix(tool, "sql:"):
		return c.checkDBParams(params)
	case strings.HasPrefix(tool, "file:"):
		return c.checkFileParams(params)
	case strings.HasPrefix(tool, "shell:") || strings.HasPrefix(tool, "exec:"):
		return c.checkCommandParams(params)
	case strings.HasPrefix(tool, "http:") || strings.HasPrefix(tool, "api:"):
		return c.checkHTTPParams(params)
	default:
		return nil
	}
}

func (c *checker) checkDBParams(params map[string]any) error {
	// 检查 sql 参数
	if sql, ok := params["sql"].(string); ok {
		if err := c.CheckSQL(sql); err != nil {
			return err
		}
	}

	// 检查 query 参数
	if query, ok := params["query"].(string); ok {
		if err := c.CheckSQL(query); err != nil {
			return err
		}
	}

	return nil
}

func (c *checker) checkFileParams(params map[string]any) error {
	// 检查 path 参数
	if path, ok := params["path"].(string); ok {
		if err := c.CheckPath(path); err != nil {
			return err
		}
	}

	// 检查 filename 参数
	if filename, ok := params["filename"].(string); ok {
		if err := c.CheckPath(filename); err != nil {
			return err
		}
	}

	// 检查 file 参数
	if file, ok := params["file"].(string); ok {
		if err := c.CheckPath(file); err != nil {
			return err
		}
	}

	return nil
}

func (c *checker) checkCommandParams(params map[string]any) error {
	// 检查 command 参数
	if cmd, ok := params["command"].(string); ok {
		if err := c.CheckCommand(cmd); err != nil {
			return err
		}
	}

	// 检查 cmd 参数
	if cmd, ok := params["cmd"].(string); ok {
		if err := c.CheckCommand(cmd); err != nil {
			return err
		}
	}

	// 检查 args 参数中的每个参数
	if args, ok := params["args"].([]any); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				if err := c.CheckCommand(argStr); err != nil {
					return fmt.Errorf("unsafe argument: %w", err)
				}
			}
		}
	}

	return nil
}

func (c *checker) checkHTTPParams(params map[string]any) error {
	// 检查 url 参数中的 SSRF 攻击
	if url, ok := params["url"].(string); ok {
		if err := checkSSRF(url); err != nil {
			return err
		}
	}

	return nil
}

// checkSSRF 检查 SSRF 攻击
func checkSSRF(url string) error {
	// 检查内网地址
	internalPatterns := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"10.",
		"172.16.",
		"172.17.",
		"172.18.",
		"172.19.",
		"172.20.",
		"172.21.",
		"172.22.",
		"172.23.",
		"172.24.",
		"172.25.",
		"172.26.",
		"172.27.",
		"172.28.",
		"172.29.",
		"172.30.",
		"172.31.",
		"192.168.",
		"[::1]",
		"[::ffff:127.0.0.1]",
	}

	lower := strings.ToLower(url)
	for _, pattern := range internalPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("SSRF detected: URL '%s' targets internal network", url)
		}
	}

	return nil
}
