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

//go:build !pro

package relay

import (
	"context"
	"errors"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
)

var errProRequired = errors.New("TURN server is a Lattice Pro feature — upgrade at https://alattice.io/pro")

// TurnServerConfig mirrors the Pro struct so cmd/manager/cmd/turn.go compiles in community builds.
type TurnServerConfig struct {
	Logger   *log.Logger
	PublicIP string
	Port     int
	Users    []*config.User
}

type TurnServer struct{}

func NewTurnServer(_ *TurnServerConfig) *TurnServer { return &TurnServer{} }

func (ts *TurnServer) Start(_ context.Context) error { return errProRequired }
