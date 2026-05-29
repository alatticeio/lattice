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

// PacketType identifies the kind of SignalPacket.
type PacketType int32

const (
	PacketType_UNKNOWN        PacketType = 0
	PacketType_HANDSHAKE_SYN  PacketType = 1
	PacketType_HANDSHAKE_ACK  PacketType = 2
	PacketType_OFFER          PacketType = 3
	PacketType_ANSWER         PacketType = 4
	PacketType_MESSAGE        PacketType = 5
	PacketType_RESTART_NOTIFY PacketType = 6
)

// DialerType distinguishes ICE vs LRP relay dialers.
type DialerType int32

const (
	DialerType_ICE DialerType = 0
	DialerType_LRP DialerType = 1
)

// SignalPacket is the envelope exchanged between peers over NATS and relay.
// The proto oneof payload is represented as optional pointer fields; only one will be non-nil.
type SignalPacket struct {
	Type      PacketType `json:"type,omitempty"`
	Dialer    DialerType `json:"dialer,omitempty"`
	SenderID  uint64     `json:"sender_id,omitempty"`
	Handshake *Handshake `json:"handshake,omitempty"`
	Offer     *Offer     `json:"offer,omitempty"`
	Answer    *Offer     `json:"answer,omitempty"`
	Message   *Message   `json:"message,omitempty"`
}

// GetHandshake returns the Handshake payload or nil.
func (p *SignalPacket) GetHandshake() *Handshake {
	if p == nil {
		return nil
	}
	return p.Handshake
}

// GetOffer returns the Offer payload or nil.
func (p *SignalPacket) GetOffer() *Offer {
	if p == nil {
		return nil
	}
	return p.Offer
}

// GetAnswer returns the Answer payload or nil.
func (p *SignalPacket) GetAnswer() *Offer {
	if p == nil {
		return nil
	}
	return p.Answer
}

// GetMessage returns the Message payload or nil.
func (p *SignalPacket) GetMessage() *Message {
	if p == nil {
		return nil
	}
	return p.Message
}

// Offer carries ICE candidate/credential information.
type Offer struct {
	Ufrag      string `json:"ufrag,omitempty"`
	Pwd        string `json:"pwd,omitempty"`
	TieBreaker uint64 `json:"tieBreaker,omitempty"`
	Candidate  string `json:"candidate,omitempty"`
	Vip        string `json:"vip,omitempty"`
	Current    []byte `json:"current,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
}

// Message carries opaque content bytes.
type Message struct {
	Content []byte `json:"content,omitempty"`
}

// Handshake carries initial peer exchange data.
type Handshake struct {
	Timestamp int64  `json:"timestamp,omitempty"`
	PeerInfo  []byte `json:"peer_info,omitempty"`
}
