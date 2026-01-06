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

package client

import (
	"wireflow/internal/core/infra"
)

// TODO: macOS, FreeBSD and other BSDs likely do support this feature set, but
// use alternatively named flags and need ports and require testing.

// getSrcFromControl parses the control for PKTINFO and if found updates ep with
// the source information found.
func getSrcFromControl(control []byte, wep *infra.MagicEndpoint) {
}

// setSrcControl parses the control for PKTINFO and if found updates ep with
// the source information found.
func setSrcControl(control *[]byte, ep *infra.MagicEndpoint) {
}

// srcControlSize returns the recommended buffer size for pooling sticky control
// data.
const srcControlSize = 0

const StdNetSupportsStickySockets = false
