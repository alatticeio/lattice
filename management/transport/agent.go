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

package transport

import (
	"context"
	"sync/atomic"

	"github.com/wireflowio/ice"
)

type AgentWrapper struct {
	sender func(ctx context.Context, peerId string, data []byte) error // nolint
	*ice.Agent
	IsCredentialsInited atomic.Bool
	RUfrag              string
	RPwd                string
	RTieBreaker         uint64
}

type AgentConfig struct {
	Send    func(ctx context.Context, peerId string, data []byte) error
	LocalId string
	PeerID  string
	StunURI string
}
