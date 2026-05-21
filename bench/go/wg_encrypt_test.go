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

package benchgo

import (
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

// BenchmarkWireGuardEncrypt measures the cost of a single ChaCha20-Poly1305
// AEAD Seal on a 1500-byte payload (MTU-sized WireGuard packet).
// Target: < 15 μs per op.
func BenchmarkWireGuardEncrypt(b *testing.B) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		b.Fatal(err)
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		b.Fatal(err)
	}

	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		b.Fatal(err)
	}

	plaintext := make([]byte, 1500)
	if _, err := rand.Read(plaintext); err != nil {
		b.Fatal(err)
	}

	dst := make([]byte, 0, len(plaintext)+aead.Overhead())

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(plaintext)))

	for i := 0; i < b.N; i++ {
		dst = aead.Seal(dst[:0], nonce, plaintext, nil)
	}
}
