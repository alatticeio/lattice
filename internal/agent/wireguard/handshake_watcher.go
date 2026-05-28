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

package wireguard

import (
	"context"
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
)

// PeerHandshake returns the LastHandshakeTime for the peer identified by
// publicKey (base64) on the given WireGuard interface. Returns an error if the
// interface is not found or the peer is not configured.
func PeerHandshake(ifaceName, publicKey string) (time.Time, error) {
	ctr, err := wgctrl.New()
	if err != nil {
		return time.Time{}, fmt.Errorf("wgctrl: %w", err)
	}
	defer ctr.Close() //nolint:errcheck

	dev, err := ctr.Device(ifaceName)
	if err != nil {
		return time.Time{}, fmt.Errorf("device %s: %w", ifaceName, err)
	}
	for _, p := range dev.Peers {
		if p.PublicKey.String() == publicKey {
			return p.LastHandshakeTime, nil
		}
	}
	return time.Time{}, fmt.Errorf("peer %s not found on %s", publicKey, ifaceName)
}

// WatchFirstHandshake polls the WireGuard interface every 500 ms until any
// peer completes a handshake, then calls onHandshake with the elapsed time
// since startTime and returns. It is a one-shot watcher: it exits after the
// first handshake regardless of how many peers are configured.
//
// The function is safe to run as a goroutine and exits cleanly when ctx is
// cancelled. Errors opening wgctrl or reading the device are silently retried
// so that the watcher survives transient interface-not-ready races at startup.
//
// Community + Pro: this is a health indicator, not a Pro feature.
func WatchFirstHandshake(ctx context.Context, ifaceName string, startTime time.Time, onHandshake func(time.Duration)) {
	ctr, err := wgctrl.New()
	if err != nil {
		return
	}
	defer ctr.Close() //nolint:errcheck

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dev, err := ctr.Device(ifaceName)
			if err != nil {
				// Interface may not be ready yet; retry next tick.
				continue
			}
			for _, p := range dev.Peers {
				if !p.LastHandshakeTime.IsZero() {
					onHandshake(time.Since(startTime))
					return
				}
			}
		}
	}
}
