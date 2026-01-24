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

package infra

import (
	"context"
	"wireflow/internal/grpc"
)

// SignalService only used for sending signal byte packet
type SignalService interface {
	// pub/sub
	Send(ctx context.Context, peerId PeerID, data []byte) error

	//req/resp
	Request(ctx context.Context, subject, method string, data []byte) ([]byte, error)

	// server service
	Service(subject, queue string, service func(data []byte) ([]byte, error))
}

type Probe interface {
	Start(ctx context.Context, remoteId PeerID) error

	Handle(ctx context.Context, remoteId PeerID, packet *grpc.SignalPacket) error

	// 2. 健康检查：在链路 Connected 后，定时发送探测包
	Ping(ctx context.Context) error
}
