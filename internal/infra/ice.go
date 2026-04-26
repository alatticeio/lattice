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
)

// NewFilteringMux creates a FilteringUDPMux that wraps conn as the sole UDP
// reader. When showLog is true the ICE subsystem logs at DEBUG level.
// Call SetPassThrough then Start on the returned mux before creating ICE agents.
func NewFilteringMux(conn net.PacketConn, showLog bool) *FilteringUDPMux {
	var logger logging.LeveledLogger
	if showLog {
		f := logging.NewDefaultLoggerFactory()
		f.DefaultLogLevel = logging.LogLevelDebug
		logger = f.NewLogger("ice")
	}
	return NewFilteringUDPMux(conn, logger)
}
