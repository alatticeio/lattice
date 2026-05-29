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

// MessageType classifies DrpMessage purpose.
type MessageType int32

const (
	MessageType_MessageDirectOfferType       MessageType = 0
	MessageType_MessageDirectOfferAnswerType MessageType = 1
	MessageType_MessageRelayOfferType        MessageType = 2
	MessageType_MessageRelayAnswerType       MessageType = 3
	MessageType_MessageForwardType           MessageType = 4
	MessageType_MessageHeartBeatType         MessageType = 5
	MessageType_MessageDRPType               MessageType = 6
	MessageType_MessageDrpOfferType          MessageType = 7
	MessageType_MessageDrpOfferAnswerType    MessageType = 8
	MessageType_MessageDrpDataType           MessageType = 9
	MessageType_MessageRegisterType          MessageType = 10
)

// DrpMessage is the envelope used by the LRP relay protocol.
type DrpMessage struct {
	From      string      `json:"from,omitempty"`
	To        string      `json:"to,omitempty"`
	Body      []byte      `json:"body,omitempty"`
	Encrypt   int32       `json:"encrypt,omitempty"`
	Version   int32       `json:"version,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
	MsgType   MessageType `json:"msgType,omitempty"`
}
