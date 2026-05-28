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

package sandbox

import "context"

// DriverConfig holds all parameters needed to start a sandbox, regardless of
// isolation mode. Fields unused by a given driver are silently ignored.
type DriverConfig struct {
	// Common fields.
	SandboxName string
	ServerURL   string
	Token       string
	EgressAllow string
	EgressDeny  bool

	// pod mode fields.
	ProxyAddr    string
	ForwardRules []string

	// gvisor (runsc) mode fields.
	RootFS      string   // path to container root filesystem
	AgentBinary string   // AI agent binary inside the container
	AgentArgs   []string // AI agent arguments
	BundleDir   string   // writable OCI bundle dir; defaults to /tmp/lattice-runsc/<name>

	// WgPort is the UDP port for WireGuard. 0 means let the OS pick a random
	// available port, which is required when another lattice agent already
	// occupies port 51820 on the same host.
	WgPort int

	// ReadyCh, if non-nil, receives a signal when the sandbox is fully ready
	// (SOCKS5 listening, WireGuard node started). Used by sandbox run to know
	// when to inject ALL_PROXY and exec the child process.
	ReadyCh chan<- struct{}
}

// IsolationDriver abstracts the lifecycle of a sandbox isolation backend.
// Implementations: PodDriver (--mode pod), RunscDriver (--mode gvisor).
type IsolationDriver interface {
	// Name returns a short identifier for logging (e.g. "pod", "gvisor").
	Name() string
	// Start runs the sandbox, blocking until ctx is cancelled or the sandbox
	// exits. It must perform cleanup (Stop) before returning.
	Start(ctx context.Context) error
}
