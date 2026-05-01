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

package auth_test

import (
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/server/auth"
)

func TestRevocationList_RevokeAndCheck(t *testing.T) {
	rl := auth.NewRevocationList()

	expiresAt := time.Now().Add(12 * time.Hour)
	rl.Revoke("test-jti-123", expiresAt)

	if !rl.IsRevoked("test-jti-123") {
		t.Error("expected jti to be revoked")
	}

	if rl.IsRevoked("non-existent-jti") {
		t.Error("expected non-existent jti to not be revoked")
	}
}

func TestRevocationList_CleanupExpired(t *testing.T) {
	rl := auth.NewRevocationList()

	// Revoke with past expiry
	rl.Revoke("old-jti", time.Now().Add(-1*time.Hour))

	if !rl.IsRevoked("old-jti") {
		t.Error("expected old jti to be revoked")
	}

	rl.Cleanup()

	if rl.IsRevoked("old-jti") {
		t.Error("expected old jti to be cleaned up")
	}
}
