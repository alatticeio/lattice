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

package manager

import (
	"wireflow/internal/core/domain"

	wg "golang.zx2c4.com/wireguard/device"
)

var (
	_ domain.Configurer = (*defaultConfiger)(nil)
)

type defaultConfiger struct {
	device    *wg.Device
	address   string
	ifaceName string
}

func (c *defaultConfiger) GetAddress() string {
	return c.address
}

func (c *defaultConfiger) GetIfaceName() string {
	return c.ifaceName
}

type Params struct {
	Device    *wg.Device
	IfaceName string
	Address   string
}

func (c *defaultConfiger) Configure(conf *domain.DeviceConfig) error {
	return c.device.IpcSet(conf.String())
}

func (c *defaultConfiger) AddPeer(peer *domain.SetPeer) error {
	return c.device.IpcSet(peer.String())
}

func (c *defaultConfiger) RemovePeer(peer *domain.SetPeer) error {
	return c.device.IpcSet(peer.String())
}

func (c *defaultConfiger) RemoveAllPeers() {
	c.device.RemoveAllPeers()
}

func NewConfigurer(config *Params) domain.Configurer {
	return &defaultConfiger{
		device:    config.Device,
		address:   config.Address,
		ifaceName: config.IfaceName,
	}
}
