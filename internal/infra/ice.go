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
	"net"

	"github.com/pion/logging"
	"github.com/wireflowio/ice"
)

func NewUdpMux(conn net.PacketConn, showLog bool) *ice.UniversalUDPMuxDefault {

	var loggerFactory *logging.DefaultLoggerFactory
	var logger logging.LeveledLogger
	if showLog {
		loggerFactory = logging.NewDefaultLoggerFactory()
		loggerFactory.DefaultLogLevel = logging.LogLevelDebug
		logger = loggerFactory.NewLogger("ice")
	}

	universalUdpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  logger,
		UDPConn: conn,
		Net:     nil,
	})

	return universalUdpMux
}
