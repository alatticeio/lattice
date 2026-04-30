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

package relay

import (
	"testing"
)

func TestHeaderMarshalUnmarshal(t *testing.T) {
	h := Header{
		Seq:        42,
		PayloadLen: 1500,
		Cmd:        Forward,
		ToID:       99,
	}
	data := h.Marshal()
	if len(data) != HeaderSize {
		t.Fatalf("expected %d bytes, got %d", HeaderSize, len(data))
	}

	h2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if h2.Seq != h.Seq || h2.PayloadLen != h.PayloadLen || h2.Cmd != h.Cmd || h2.ToID != h.ToID {
		t.Errorf("roundtrip mismatch: %+v -> %+v", h, h2)
	}
}

func TestUnmarshalTooShort(t *testing.T) {
	_, err := Unmarshal([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short buffer")
	}
}

func TestHeaderSize(t *testing.T) {
	if HeaderSize != 12 {
		t.Errorf("expected HeaderSize=12, got %d", HeaderSize)
	}
}

func TestCommands(t *testing.T) {
	if Register != 0x01 {
		t.Errorf("Register = %d, want 0x01", Register)
	}
	if Forward != 0x02 {
		t.Errorf("Forward = %d, want 0x02", Forward)
	}
	if KeepAlive != 0x03 {
		t.Errorf("KeepAlive = %d, want 0x03", KeepAlive)
	}
	if Probe != 0x04 {
		t.Errorf("Probe = %d, want 0x04", Probe)
	}
}

func TestMarshalInto(t *testing.T) {
	h := Header{Seq: 100, PayloadLen: 500, Cmd: Forward, ToID: 42}
	buf := make([]byte, HeaderSize)
	h.MarshalInto(buf)

	h2, err := Unmarshal(buf)
	if err != nil {
		t.Fatal(err)
	}
	if h2.Seq != h.Seq || h2.PayloadLen != h.PayloadLen || h2.Cmd != h.Cmd || h2.ToID != h.ToID {
		t.Errorf("MarshalInto roundtrip mismatch")
	}
}
