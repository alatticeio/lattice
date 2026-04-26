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

import "wireflow/internal/infra"

// isInitiator returns true when the local node should drive the ICE/WRRP
// handshake (send SYN, drive OFFER/ANSWER, set PersistentKeepalive).
//
// Numeric uint64 comparison is used throughout to avoid decimal string ordering
// bugs: "9" > "14" lexicographically but 9 < 14 numerically.  The previous
// code had three different comparisons (two string, one numeric) which gave
// inconsistent results for IDs that differ in decimal digit count.
func isInitiator(local, remote infra.PeerIdentity) bool {
	return local.ID().ToUint64() > remote.ID().ToUint64()
}
