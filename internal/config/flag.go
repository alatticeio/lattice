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

// Flags is a struct that contains the flags that are passed to the mgtClient
type Flags struct {
	LogLevel      string
	RedisAddr     string
	RedisPassword string
	InterfaceName string
	ForceRelay    bool
	AppKey        string

	// DaemonGround is a flag to indicate whether the node should run in foreground mode
	DaemonGround  bool
	MetricsEnable bool
	DnsEnable     bool

	//Url
	ManagementUrl string
	SignalingUrl  string
	TurnServerUrl string
}

type NetworkOptions struct {
	AppId      string
	Identifier string
	Name       string
	CIDR       string
	ServerUrl  string
}
