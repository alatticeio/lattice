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

package client

import (
	"math/rand"
	"time"
	"wireflow/pkg/log"

	"golang.zx2c4.com/wireguard/tun"
)

func CreateTUN(mtu int, logger *log.Logger) (string, tun.Device, error) {
	name := getInterfaceName()
	device, err := tun.CreateTUN(name, mtu)
	return name, device, err
}

func getInterfaceName() string {
	rand.Seed(time.Now().UnixNano())
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, 3)
	for i := 0; i < 3; i++ {
		bytes[i] = letters[rand.Intn(len(letters))]
	}

	return "linkany-" + string(bytes)
}
