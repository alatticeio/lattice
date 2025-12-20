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

package config

type Protocol string

type LoginInfo struct {
	Token string `json:"token,omitempty"`
}

// HttpRequest will take any real request to linkany server
type HttpRequest[T any] struct {
	T T
}

// PeerRegisterInfo current peer info, will register to linkany server,then on web
// you can configure the peer online.
type PeerRegisterInfo struct {
	ID                  string `json:"id,omitempty"`
	AppId               string `json:"appId,omitempty"`
	Hostname            string `json:"hostname,omitempty"`
	PrivateKey          string `json:"privateKey,omitempty"`
	PublicKey           string `json:"publicKey,omitempty"`
	PersistentKeepalive int    `json:"persistentKeepalive,omitempty"`

	Ufrag      string `json:"ufrag,omitempty"`
	Pwd        string `json:"pwd,omitempty"`
	TieBreaker uint32 `json:"tieBreaker,omitempty"`
	HostIP     string `json:"hostIP,omitempty"`  // inner ip port
	SrflxIP    string `json:"srflxIP,omitempty"` // nat ip port
	RelayIP    string `json:"relayIP,omitempty"` // relay ip port
	Status     int    `json:"status,omitempty"`
}

type PeerInfo struct {
	PrivateKey          string   `json:"privateKey,omitempty"`
	PublicKey           string   `json:"publicKey,omitempty"`
	PersistentKeepalive int      `json:"persistentKeepalive,omitempty"`
	AllowedIPS          []string `json:"allowedIPS,omitempty"`
}
