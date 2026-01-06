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
	"net"
	"wireflow/internal/grpc"
)

// SignalService only used for sending signal byte packet
type SignalService interface {
	// pub/sub
	Send(ctx context.Context, peerId string, data []byte) error

	//req/resp
	Request(ctx context.Context, subject, method string, data []byte) ([]byte, error)

	// server service
	Service(subject, queue string, service func(subject string, data []byte) ([]byte, error))
}

// Transport for transfer wireguard packets
type Transport interface {
	// Init and gather candidates send to peerId
	Prepare() error

	// 2. 注入远端发来的候选地址或 Offer
	HandleOffer(ctx context.Context, peerId string, packet *grpc.SignalPacket) error

	OnConnectionStateChange(state TransportState) error

	// 3. 启动/重启打洞尝试
	// 这是一个异步过程，成功后会建立物理连接
	Start(ctx context.Context, peerId string) error

	// 4. 获取连接对象
	// 打洞成功后，提供给 Wireguard 使用的 ReadWriter
	RawConn() (net.Conn, error)

	// 5. 暴露底层状态（New/Gathering/Checking/Connected/Failed）
	State() TransportState

	// 6. 销毁资源
	Close() error
}

type TransportState int

const (
	New TransportState = iota
	Preparing
	Gathering
	Checking
	Connected
	Failed
)

type Probe interface {
	// 1. 核心控制循环：驱动 Transport 进行打洞
	Probe(ctx context.Context, remoteId string) error

	HandleOffer(ctx context.Context, remoteId string, packet *grpc.SignalPacket) error

	// 2. 健康检查：在链路 Connected 后，定时发送探测包
	// 记录 RTT、抖动、丢包率等
	Ping(ctx context.Context) error

	// 3. 策略回调：当 Transport 报告 Failed 时被调用
	// 内部实现：是立即重试，还是退避 5 秒后再重试
	OnTransportFail(err error)
}
