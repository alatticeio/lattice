// Copyright 2025 wireflowio.com, Inc.
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

const (
	// PageNo default page number
	PageNo = 1
	// PageSize default page size
	PageSize = 10
)

type AcceptType string

const (
	ACCEPT AcceptType = "accepted"
	REJECT AcceptType = "rejected"
)

type KeyValue struct {
	Key   string
	Value interface{}
}

func NewKeyValue(k string, v interface{}) *KeyValue {
	return &KeyValue{
		Key:   k,
		Value: v,
	}
}

type ParamBuilder interface {
	Generate() []*KeyValue
}

type GroupType int

const (
	OwnGroupType = iota
	SharedType
)

func (g GroupType) String() string {
	switch g {
	case OwnGroupType:
		return "own"
	case SharedType:
		return "invited"
	default:
		return "Unknown"
	}
}
