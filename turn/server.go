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

//go:build pro

package turn

import (
	"context"
	"net"
	"strconv"
	"wireflow/internal/config"
	"wireflow/internal/log"

	"github.com/pion/turn/v4"
)

const realm = "wireflow.run"

type TurnServer struct {
	logger   *log.Logger
	port     int
	publicIP string
	users    []*config.User
}

type TurnServerConfig struct {
	Logger   *log.Logger
	PublicIP string
	Port     int
	Users    []*config.User
}

func NewTurnServer(cfg *TurnServerConfig) *TurnServer {
	return &TurnServer{
		logger:   cfg.Logger,
		port:     cfg.Port,
		publicIP: cfg.PublicIP,
		users:    cfg.Users,
	}
}

// Start runs the TURN server until ctx is cancelled.
func (ts *TurnServer) Start(ctx context.Context) error {
	ts.logger.Info("TURN server starting", "public_ip", ts.publicIP, "port", ts.port)

	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(ts.port))
	if err != nil {
		return err
	}

	authKeys := buildAuthKeyMap(ts.users)

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(username, _ string, _ net.Addr) ([]byte, bool) {
			key, ok := authKeys[username]
			return key, ok
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(ts.publicIP),
					Address:      "0.0.0.0",
				},
			},
		},
	})
	if err != nil {
		return err
	}

	<-ctx.Done()
	ts.logger.Info("TURN server shutting down")

	if err = s.Close(); err != nil {
		ts.logger.Error("failed to close TURN server", err)
	}
	return nil
}

// buildAuthKeyMap pre-hashes credentials for the TURN auth handler.
func buildAuthKeyMap(users []*config.User) map[string][]byte {
	m := make(map[string][]byte, len(users))
	for _, u := range users {
		m[u.Username] = turn.GenerateAuthKey(u.Username, realm, u.Password)
	}
	return m
}
