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

package utils

// ResourceType 资源类型
type ResourceType int

const (
	Group ResourceType = iota
	Node
	Policy
	Label
	Rule
)

func (r ResourceType) String() string {
	switch r {
	case Group:
		return "group"
	case Node:
		return "node"
	case Label:
		return "label"
	case Policy:
		return "policy"
	case Rule:
		return "rule"
	default:
		return "Unknown"
	}
}
