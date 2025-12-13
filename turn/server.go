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

package turn

import (
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"wireflow/management/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"

	"github.com/pion/turn/v4"
)

type TurnServer struct {
	logger   *log.Logger
	port     int
	publicIP string
	client   *client.Client
}

type TurnServerConfig struct {
	Logger   *log.Logger
	PublicIP string
	Port     int
	Client   *client.Client
}

func NewTurnServer(cfg *TurnServerConfig) *TurnServer {
	return &TurnServer{client: cfg.Client, port: cfg.Port, publicIP: cfg.PublicIP, logger: cfg.Logger}
}

func (ts *TurnServer) Start() error {
	return ts.start(ts.publicIP, ts.port)
}

func (ts *TurnServer) start(publicIP string, port int) error {
	ts.logger.Verbosef("Starting turn server on %s:%d", publicIP, port)

	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	//usersMap := map[string][]byte{}
	//for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(users, -1) {
	//	usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], "wireflowio.com", kv[2])
	//}

	usersMap := generateAuthKeyMap(ts.client.GetUsers())

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: "wireflowio.com",
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the remoteKey for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) { // nolint: revive
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(publicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",             // But actually be listening on every interface
				},
			},
		},
	})
	if err != nil {
		return err
	}

	// Block until user sends SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err = s.Close(); err != nil {
		ts.logger.Errorf("Failed to close turn server: %v", err)
	}

	return nil
}

func generateAuthKeyMap(users []*config.User) map[string][]byte {
	usersMap := map[string][]byte{}
	for _, user := range users {
		usersMap[user.Username] = generateAuthKey(user.Username, "wireflowio.com", user.Password)
	}
	return usersMap
}

func generateAuthKey(username, realm, password string) []byte {
	return turn.GenerateAuthKey(username, realm, password)
}
