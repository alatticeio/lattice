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
	"path/filepath"
	"strings"
)

// sensitiveDirectories 包含敏感目录前缀
var sensitiveDirectories = []string{
	"/etc",
	"/var",
	"/root",
	"/home",
	"/proc",
	"/sys",
	"/dev",
	"/boot",
	"/usr/bin",
	"/usr/sbin",
	"/sbin",
	"/bin",
}

// CheckPath 检查文件路径是否包含路径遍历或访问敏感目录
func CheckPath(path string) error {
	if path == "" {
		return nil
	}

	// 清理路径
	cleaned := filepath.Clean(path)

	// 检测路径遍历
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal detected: path '%s' contains '..'", path)
	}

	// 检测绝对路径访问敏感目录
	if filepath.IsAbs(cleaned) {
		for _, dir := range sensitiveDirectories {
			if strings.HasPrefix(cleaned, dir) {
				return fmt.Errorf("access to sensitive directory denied: path '%s' targets '%s'", path, dir)
			}
		}
	}

	return nil
}
