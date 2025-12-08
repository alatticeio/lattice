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

package drp

import (
	"fmt"
	"net"
	"net/url"
	"testing"
)

func TestParseNode(t *testing.T) {

	str := "http://10.0.0.1:8080/drp"
	u, err := url.Parse(str)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(u.Host, u.Port())

	addr, err := net.ResolveTCPAddr("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}

	node := NewNode("", addr, nil)
	fmt.Println(node)

}
