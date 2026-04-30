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
	"context"
	"sync/atomic"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/grpc"

	"google.golang.org/protobuf/proto"
)

const (
	sendChanDepth  = 256
	writerBufSize  = 128 * 1024
	probeChanSize  = 1024
	MaxProbePayload = 2048
)

type Task struct {
	SessionID uint64
	Data      []byte
}

type writer interface {
	Write(p []byte) (int, error)
}

// lrpClient holds logic shared between TCP and QUIC clients.
type lrpClient struct {
	ctx       context.Context
	cancel    context.CancelFunc
	log       *log.Logger
	localId   infra.PeerID
	serverURL string
	onMessage func(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error
	probeCh   chan *Task
	seq       atomic.Uint32
}

func (c *lrpClient) nextSeq() uint16 {
	return uint16(c.seq.Add(1) & 0xFFFF)
}

func (c *lrpClient) probeWorker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case task := <-c.probeCh:
			var packet grpc.SignalPacket
			if err := proto.Unmarshal(task.Data, &packet); err != nil {
				c.log.Error("failed to unmarshal probe packet", err)
				continue
			}
			if err := c.onMessage(c.ctx, infra.FromUint64(packet.SenderId), &packet); err != nil {
				c.log.Error("probe handler returned error", err)
			}
		}
	}
}

// register sends a Register frame on the given writer.
func (c *lrpClient) register(w writer) error {
	h := &Header{
		Seq:        c.nextSeq(),
		PayloadLen: 0,
		Cmd:        Register,
		ToID:       uint32(c.localId.ToUint64()),
	}
	_, err := w.Write(h.Marshal())
	return err
}

// makeFrame builds a complete LRP frame (header + payload).
func (c *lrpClient) makeFrame(toID uint64, cmd uint8, data []byte) []byte {
	h := Header{
		Seq:        c.nextSeq(),
		PayloadLen: uint32(len(data)),
		Cmd:        cmd,
		ToID:       uint32(toID),
	}
	hb := h.Marshal()
	frame := make([]byte, HeaderSize+len(data))
	copy(frame, hb)
	copy(frame[HeaderSize:], data)
	return frame
}

