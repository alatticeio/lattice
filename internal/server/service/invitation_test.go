// Copyright 2026 The Lattice Authors, Inc.
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

package service_test

import (
	"testing"

	"github.com/alatticeio/lattice/internal/server/service"
)

func TestInviteToken_SignAndVerify(t *testing.T) {
	secret := "test-secret-key"

	token, err := service.GenerateInviteToken(secret)
	if err != nil {
		t.Fatal(err)
	}

	// Valid token should verify
	if !service.VerifyInviteToken(token, secret) {
		t.Error("expected valid token to verify")
	}

	// Wrong secret should fail
	if service.VerifyInviteToken(token, "wrong-secret") {
		t.Error("expected token to fail with wrong secret")
	}

	// Tampered token should fail
	tampered := token[:len(token)-1] + "x"
	if service.VerifyInviteToken(tampered, secret) {
		t.Error("expected tampered token to fail")
	}

	// Malformed token should fail
	if service.VerifyInviteToken("not-a-token", secret) {
		t.Error("expected malformed token to fail")
	}
}
