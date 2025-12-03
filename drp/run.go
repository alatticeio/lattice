// Copyright 2025 wireflowio.com, Inc.
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

package drp

import (
	"wireflow/pkg/log"
)

func Start(listen string) error {
	// Create a new server
	s, err := NewServer(&ServerConfig{
		Listen: listen,
		Logger: log.NewLogger(log.Loglevel, "drp-signaling"),
	})

	if err != nil {
		return err

	}
	// Start the server
	return s.Start()
}
