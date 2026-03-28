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
	"net/netip"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestPrefix(t *testing.T) {
	s := "10.0.0.2/32"
	prefix, err := netip.ParsePrefix(s)
	fmt.Println(prefix, err)
}

func TestFromKey(t *testing.T) {
	str := "08v7fO4FCBQutPFgUEZvUNj8KYE3IvOynDJD7OYAemc="
	key, err := wgtypes.ParseKey(str)
	if err != nil {
		t.Fatal(err)
	}
	localId := FromKey(key)
	t.Log(localId.ToUint64())
	t.Log(int64(localId.ToUint64()))
}
