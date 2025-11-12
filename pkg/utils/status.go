// Copyright 2025 Wireflow.io, Inc.
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

import (
	"encoding/json"
	"fmt"
)

type NodeStatus int

const (
	Unregisterd NodeStatus = iota
	Registered
	Online
	Offline
	Disabled
)

func (n NodeStatus) String() string {
	switch n {
	case Unregisterd:
		return "unregistered"
	case Registered:
		return "registered"
	case Online:
		return "online"
	case Offline:
		return "offline"
	case Disabled:
		return "disabled"
	default:
		return "unknown"
	}
}

type ActiveStatus int

const (
	DISABLED ActiveStatus = iota
	ENABLED
)

func (s ActiveStatus) String() string {
	switch s {
	case DISABLED:
		return "disabled"
	case ENABLED:
		return "enabled"
	default:
		return "unknown"
	}
}

func (s ActiveStatus) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(s.String())
}

var statusMap = map[string]ActiveStatus{
	"disabled": DISABLED,
	"enabled":  ENABLED,
}

func (s *ActiveStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// 根据字符串设置Status值
	if status, ok := statusMap[str]; ok {
		*s = status
		return nil
	}

	return fmt.Errorf("invalid status: %s", str)
}

type RuleType int

const (
	NodeToNode RuleType = iota
	NodeToTag
	TagToNode
	TagToTag
)

func (r RuleType) String() string {
	switch r {
	case NodeToNode:
		return "节点到节点"
	case NodeToTag:
		return "节点到标签"
	case TagToNode:
		return "标签到节点"
	case TagToTag:
		return "标签到标签"
	default:
		return "未知"
	}
}

func (r RuleType) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(r.String())
}

//type ActiveStatus int
//
//func (a ActiveStatus) String() string {
//	switch a {
//	case 0:
//		return "forbidden"
//	case 1:
//		return "active"
//	default:
//		return "unknown"
//	}
//}
