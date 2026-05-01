// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2026 The Lattice Authors
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

package auth

import (
	"sync"
	"time"
)

// RevocationList tracks revoked JWT IDs (jti) with their expiration times.
type RevocationList struct {
	mu      sync.RWMutex
	revoked map[string]time.Time // jti → expiresAt
}

// NewRevocationList creates an empty revocation list.
func NewRevocationList() *RevocationList {
	return &RevocationList{
		revoked: make(map[string]time.Time),
	}
}

// Revoke adds a jti to the revocation list with its expiration time.
func (rl *RevocationList) Revoke(jti string, expiresAt time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.revoked[jti] = expiresAt
}

// IsRevoked checks if a jti has been revoked.
func (rl *RevocationList) IsRevoked(jti string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	_, ok := rl.revoked[jti]
	return ok
}

// Cleanup removes all entries whose expiration time has passed.
func (rl *RevocationList) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for jti, expiresAt := range rl.revoked {
		if now.After(expiresAt) {
			delete(rl.revoked, jti)
		}
	}
}

// StartCleanup starts a background goroutine that periodically cleans expired entries.
func (rl *RevocationList) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			rl.Cleanup()
		}
	}()
}
