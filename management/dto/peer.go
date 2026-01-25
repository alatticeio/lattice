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

package dto

import (
	"time"
)

type PeerDto struct {
	Name                string    `json:"name,omitempty"`
	Description         string    `json:"description,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	InterfaceName       string    `json:"interface_name,omitempty"`
	NetworkID           string    `json:"networkID,omitempty"` // belong to which group
	CreatedBy           string    `json:"createdBy,omitempty"` // ownerID
	UserId              uint64    `json:"userId,omitempty"`
	Hostname            string    `json:"hostname,omitempty"`
	AppID               string    `json:"app_id,omitempty"`
	Address             *string   `json:"address,omitempty"`
	Endpoint            string    `json:"endpoint,omitempty"`
	PersistentKeepalive int       `json:"persistentKeepalive,omitempty"`
	PublicKey           string    `json:"publicKey,omitempty"`
	PeerID              uint64    `json:"peerId,omitempty"`
	AllowedIPs          string    `json:"allowedIps,omitempty"`
	RelayIP             string    `json:"relayIp,omitempty"`
	TieBreaker          uint32    `json:"tieBreaker"`
	Ufrag               string    `json:"ufrag"`
	Pwd                 string    `json:"pwd"`
	Port                int       `json:"port"`
	GroupName           string    `json:"groupName"`
	Version             uint64    `json:"version"`
	LastUpdatedAt       time.Time `json:"lastUpdatedAt"`
	Token               string    `json:"token,omitempty"`
}

type TokenDto struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Expiry    string `json:"expiry"`
	Limit     int    `json:"limit"`
}
