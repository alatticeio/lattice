// Copyright 2026 The Lattice Authors, Inc.
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

package signal

// Type classifies ManagementMessage purpose.
type Type int32

const (
	Type_MessageTypeCreateNetwork     Type = 0
	Type_MessageTypeJoinNetwork       Type = 1
	Type_MessageTypeLeaveNetwork      Type = 2
	Type_MessageTypeNetworkAddNode    Type = 3
	Type_MessageTypeNetworkRemoveNode Type = 4
)

// ManagementMessage is the envelope for management channel communication.
type ManagementMessage struct {
	PubKey    string `json:"pub_key,omitempty"`
	Body      []byte `json:"body,omitempty"`
	Encrypt   int32  `json:"encrypt,omitempty"`
	Version   int32  `json:"version,omitempty"`
	Type      Type   `json:"type,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// LoginRequest carries credentials for management authentication.
type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// LoginResponse carries the token issued after successful login.
type LoginResponse struct {
	Token string `json:"token,omitempty"`
}

// Request carries a peer's public key and enrollment token.
type Request struct {
	PubKey string `json:"pub_key,omitempty"`
	AppId  string `json:"appId,omitempty"`
	Token  string `json:"token,omitempty"`
}
