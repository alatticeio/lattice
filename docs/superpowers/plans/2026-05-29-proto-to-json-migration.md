# Proto → JSON Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `google.golang.org/protobuf` (binary proto serialization) with standard `encoding/json` for signal/relay messages, removing ~4MB from the agent binary.

**Architecture:** The 3 `.pb.go` files in `internal/grpc/` define message types used purely for serialization over NATS and QUIC/TCP transport. They have no generated gRPC service code (those were already deleted). We replace them with plain Go structs with JSON tags. The `oneof payload` in `SignalPacket` becomes optional pointer fields. We add `GetHandshake()`/`GetOffer()`/`GetMessage()` helper methods to minimize caller changes. All 6 `proto.Marshal`/`proto.Unmarshal` calls become `json.Marshal`/`json.Unmarshal`. Wire format changes from binary protobuf to JSON — all lattice components update simultaneously (monorepo).

**Tech Stack:** Go 1.25, `encoding/json` (stdlib)

**Branch:** `feat/reduce-binary-size`

---

## File Map

| Action | File |
|--------|------|
| Create | `internal/grpc/signal.go` |
| Create | `internal/grpc/drp.go` |
| Create | `internal/grpc/management.go` |
| Delete | `internal/grpc/signal.pb.go` |
| Delete | `internal/grpc/drp.pb.go` |
| Delete | `internal/grpc/management.pb.go` |
| Modify | `internal/server/transport/ice_dialer.go` |
| Modify | `internal/server/transport/lrp_dialer.go` |
| Modify | `internal/server/transport/probe_factory.go` |
| Modify | `internal/server/transport/probe.go` |
| Modify | `internal/server/nats/nats.go` |
| Modify | `internal/server/resource/client.go` |
| Modify | `internal/relay/lrp_client.go` |
| Modify | `internal/relay/lrp_client_quic.go` |
| Modify | `internal/relay/lrp_client_tcp.go` |
| Modify | `internal/agent/infra/dialer.go` |
| Modify | `internal/agent/infra/transport.go` |

---

## Task 1: Create replacement Go type files

Replace the 3 pb.go files with plain Go files. Same package `grpc`, same type names, no proto dependencies.

**Files:**
- Create: `internal/grpc/signal.go`
- Create: `internal/grpc/drp.go`
- Create: `internal/grpc/management.go`
- Delete: `internal/grpc/signal.pb.go`, `internal/grpc/drp.pb.go`, `internal/grpc/management.pb.go`

- [ ] **Step 1: Create internal/grpc/signal.go**

```go
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

package grpc

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
// The oneof payload is represented as optional pointer fields; only one will be non-nil.
type SignalPacket struct {
	Type     PacketType `json:"type,omitempty"`
	Dialer   DialerType `json:"dialer,omitempty"`
	SenderID uint64     `json:"sender_id,omitempty"`
	// Payload — at most one of these will be set.
	Handshake *Handshake `json:"handshake,omitempty"`
	Offer     *Offer     `json:"offer,omitempty"`
	Answer    *Answer    `json:"answer,omitempty"`
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

// Answer is an alias of Offer (same fields, used for the answer phase).
type Answer = Offer

// Message carries opaque content bytes.
type Message struct {
	Content []byte `json:"content,omitempty"`
}

// Handshake carries initial peer exchange data.
type Handshake struct {
	Timestamp int64  `json:"timestamp,omitempty"`
	PeerInfo  []byte `json:"peer_info,omitempty"`
}
```

- [ ] **Step 2: Create internal/grpc/drp.go**

```go
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

package grpc

// MessageType classifies DrpMessage purpose.
type MessageType int32

const (
	MessageType_UNKNOWN  MessageType = 0
	MessageType_SIGNAL   MessageType = 1
	MessageType_FORWARD  MessageType = 2
	MessageType_REGISTER MessageType = 3
	MessageType_PING     MessageType = 4
	MessageType_PONG     MessageType = 5
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
```

- [ ] **Step 3: Create internal/grpc/management.go**

```go
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

package grpc

// Type classifies ManagementMessage purpose.
type Type int32

const (
	Type_UNKNOWN  Type = 0
	Type_LOGIN    Type = 1
	Type_GET      Type = 2
	Type_LIST     Type = 3
	Type_WATCH    Type = 4
	Type_KEEPALIVE Type = 5
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
	AppID  string `json:"appId,omitempty"`
	Token  string `json:"token,omitempty"`
}
```

- [ ] **Step 4: Verify the exact enum constants used in the codebase**

Before deleting pb.go, confirm the enum constant names/values match what callers use:

```bash
grep -rn "grpc\.\(PacketType_\|DialerType_\|MessageType_\|Type_\)" --include="*.go" \
  internal/ cmd/ 2>/dev/null | grep -v "pb.go\|_test.go" | grep -oE "grpc\.[A-Za-z_]+" | sort -u
```

Compare with what we defined in the new files. If any constant is missing, add it.

- [ ] **Step 5: Delete the pb.go files**

```bash
rm internal/grpc/signal.pb.go internal/grpc/drp.pb.go internal/grpc/management.pb.go
```

- [ ] **Step 6: Build to see what breaks**

```bash
go build ./... 2>&1 | head -40
```

Expected: errors in callers referencing `SignalPacket_Handshake`, `SignalPacket_Offer`, `SignalPacket_Message`, `proto.Marshal`, `proto.Unmarshal`. These are fixed in Task 2.

- [ ] **Step 7: Commit the new type files (don't wait for callers)**

Actually — do NOT commit yet. Wait until Task 2 makes everything compile. Then commit both tasks together in one commit.

---

## Task 2: Update all callers

Fix the 6 proto.Marshal/Unmarshal calls and 5 oneof setter calls across 11 files. Also remove `"google.golang.org/protobuf/proto"` imports.

**Files:**
- Modify: `internal/server/transport/ice_dialer.go`
- Modify: `internal/server/transport/lrp_dialer.go`
- Modify: `internal/server/nats/nats.go`
- Modify: `internal/server/resource/client.go`
- Modify: `internal/relay/lrp_client.go`
- Plus any others that fail to compile

### Pattern A: Replace proto.Marshal → json.Marshal

In every file that calls `proto.Marshal(x)`:
- Replace: `proto.Marshal(x)` → `json.Marshal(x)`
- Replace import: `"google.golang.org/protobuf/proto"` → `"encoding/json"` (if not already imported)
- `proto.Marshal` returns `([]byte, error)` — `json.Marshal` has the same signature ✓

### Pattern B: Replace proto.Unmarshal → json.Unmarshal

In every file that calls `proto.Unmarshal(data, &x)`:
- Replace: `proto.Unmarshal(data, &x)` → `json.Unmarshal(data, &x)`
- Same signature: `([]byte, interface{}) error` ✓

### Pattern C: Replace oneof setters

Find all occurrences of:
```go
p.Payload = &grpc.SignalPacket_Handshake{Handshake: hs}
```
Replace with:
```go
p.Handshake = hs
```

Find:
```go
p.Payload = &grpc.SignalPacket_Offer{
    Offer: &grpc.Offer{...},
}
```
Replace with:
```go
p.Offer = &grpc.Offer{...}
```

Find:
```go
Payload: &grpc.SignalPacket_Message{
    Message: &grpc.Message{Content: ...},
}
```
Replace with:
```go
Message: &grpc.Message{Content: ...},
```

Note: `packet.GetHandshake()`, `packet.GetOffer()`, `packet.GetMessage()` are now defined as methods on the new struct — those callers need NO change.

### Specific file changes:

- [ ] **Step 1: Fix internal/server/nats/nats.go**

Read the file, then:
- Replace `proto.Unmarshal(m.Data, &packet)` → `json.Unmarshal(m.Data, &packet)`
- Remove `"google.golang.org/protobuf/proto"` import, add `"encoding/json"` if missing

- [ ] **Step 2: Fix internal/relay/lrp_client.go**

- Replace `proto.Unmarshal(task.Data, &packet)` → `json.Unmarshal(task.Data, &packet)`
- Remove proto import, add json import

- [ ] **Step 3: Fix internal/server/resource/client.go**

- Replace `proto.Marshal(packet)` → `json.Marshal(packet)`
- Replace `Payload: &grpc.SignalPacket_Message{Message: &grpc.Message{...}}` → `Message: &grpc.Message{...}`
- Update imports

- [ ] **Step 4: Fix internal/server/transport/ice_dialer.go**

- Replace `proto.Marshal(p)` → `json.Marshal(p)`
- Replace `p.Payload = &grpc.SignalPacket_Handshake{Handshake: hs}` → `p.Handshake = hs`
- Replace `p.Payload = &grpc.SignalPacket_Offer{Offer: &grpc.Offer{...}}` → `p.Offer = &grpc.Offer{...}`
- Update imports

- [ ] **Step 5: Fix internal/server/transport/lrp_dialer.go**

- Replace `proto.Marshal(p)` (2 calls) → `json.Marshal(p)`
- Replace `p.Payload = &grpc.SignalPacket_Handshake{Handshake: hs}` → `p.Handshake = hs`
- Replace `Payload: &grpc.SignalPacket_Offer{Offer: ...}` → `Offer: ...`
- Update imports

- [ ] **Step 6: Fix remaining files that fail to compile**

Run `go build ./... 2>&1` and fix any remaining errors. Common causes:
- Any remaining `SignalPacket_Xxx` wrapper types → remove the wrapper, use direct field
- Any `proto.` calls missed → replace with `json.`
- Any missing constants → check and add to the new type files

- [ ] **Step 7: Build everything**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 8: Verify protobuf is gone from agent deps**

```bash
go list -deps ./cmd/lattice/ | grep "google.golang.org/protobuf"
```
Expected: no output.

- [ ] **Step 9: Binary size check**

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/lattice-noproto ./cmd/lattice/ && ls -lh /tmp/lattice-noproto
```

- [ ] **Step 10: Run tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 11: Lint**

```bash
make lint
```
Expected: 0 issues.

- [ ] **Step 12: Commit all changes**

```bash
git add internal/grpc/signal.go internal/grpc/drp.go internal/grpc/management.go
git add -u internal/grpc/signal.pb.go internal/grpc/drp.pb.go internal/grpc/management.pb.go
git add internal/server/transport/ice_dialer.go internal/server/transport/lrp_dialer.go
git add internal/server/nats/nats.go internal/server/resource/client.go
git add internal/relay/lrp_client.go
git add $(git diff --name-only)
git commit -s -m "refactor(grpc): replace protobuf serialization with JSON, remove protobuf dependency"
```

---

## Verification

```bash
# Protobuf gone from agent
go list -deps ./cmd/lattice/ | grep "google.golang.org/protobuf"
# Expected: no output

# Binary size
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/lattice-final ./cmd/lattice/
ls -lh /tmp/lattice-final
# Expected: ~14MB (down from 18MB)

make test
make lint
```
