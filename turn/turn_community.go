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

//go:build !pro

package turn

import (
	"context"
	"errors"
	"wireflow/internal/config"
	"wireflow/internal/log"
)

var errProRequired = errors.New("TURN server is a Wireflow Pro feature — upgrade at https://wireflow.run/pro")

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
