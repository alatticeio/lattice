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

// dangerousSQLPatterns 包含危险的 SQL 模式
var dangerousSQLPatterns = []string{
	"DROP\\s+TABLE",
	"DELETE\\s+FROM",
	"UPDATE\\s+.*\\s+SET",
	"INSERT\\s+INTO",
	"ALTER\\s+TABLE",
	"EXEC\\s*\\(",
	"UNION\\s+SELECT",
	"--",
	";",
	"xp_cmdshell",
	"sp_executesql",
}

// CheckSQL 检查 SQL 语句是否包含危险模式
func CheckSQL(sql string) error {
	if sql == "" {
		return nil
	}

	upper := strings.ToUpper(sql)

	for _, pattern := range dangerousSQLPatterns {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			continue
		}

		if re.MatchString(upper) {
			return fmt.Errorf("potentially dangerous SQL detected: pattern '%s' found in query", pattern)
		}
	}

	return nil
}
