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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	managementnats "github.com/alatticeio/lattice/internal/server/nats"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// RegisterSandboxViaNATS performs the two-phase sandbox bootstrap over NATS:
//  1. NATS register with enrollment token + public key → receive agent JWT.
//  2. NATS GetNetMap poll until the controller assigns a VPN IP.
//
// Returns a fully-populated *infra.Peer with PrivateKey injected (never sent
// to the server). The returned peer is passed as NodeConfig.CurrentPeer to
// skip the NATS register inside NewNode.
//
// serverURL is the Lattice control-plane HTTP base URL (used only for
// NATS URL discovery). enrollmentToken is the one-time enrollment token.
func RegisterSandboxViaNATS(
	ctx context.Context,
	serverURL, enrollmentToken, agentName string,
	privKey wgtypes.Key,
) (*infra.Peer, error) {
	pubKey := privKey.PublicKey().String()

	natsURL, err := discoverNATSURLOnly(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("discover NATS: %w", err)
	}

	natsClient, err := managementnats.NewNatsService(ctx, agentName, "sandbox", natsURL)
	if err != nil {
		return nil, fmt.Errorf("NATS connect: %w", err)
	}
	defer func() { _ = natsClient.Close() }()

	// Step 1: NATS register — sends enrollment token + public key.
	// Server creates LatticePeer + AgentIdentity and returns JWT in Token field.
	regPayload, _ := json.Marshal(&dto.PeerDto{
		AppID:     agentName,
		Token:     enrollmentToken,
		PublicKey: pubKey,
		Port:      51820,
	})
	regData, err := natsClient.Request(ctx, "lattice.signals.peer", "register", regPayload)
	if err != nil {
		return nil, fmt.Errorf("NATS register: %w", err)
	}
	var registered infra.Peer
	if err = json.Unmarshal(regData, &registered); err != nil {
		return nil, fmt.Errorf("parse register response: %w", err)
	}
	agentJWT := registered.Token
	if agentJWT == "" {
		return nil, fmt.Errorf("server returned empty JWT")
	}

	return fetchNetMap(ctx, natsClient, agentName, pubKey, agentJWT, privKey)
}

// ResumeSandboxViaNATS resumes a previously-registered sandbox using persisted
// credentials (private key + JWT). It skips re-registration and fetches the
// current network map directly. Use this on container restarts to avoid consuming
// the one-time enrollment token again.
func ResumeSandboxViaNATS(
	ctx context.Context,
	serverURL, agentJWT, agentName string,
	privKey wgtypes.Key,
) (*infra.Peer, error) {
	natsURL, err := discoverNATSURLOnly(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("discover NATS: %w", err)
	}

	natsClient, err := managementnats.NewNatsService(ctx, agentName, "sandbox", natsURL)
	if err != nil {
		return nil, fmt.Errorf("NATS connect: %w", err)
	}
	defer func() { _ = natsClient.Close() }()

	return fetchNetMap(ctx, natsClient, agentName, privKey.PublicKey().String(), agentJWT, privKey)
}

// fetchNetMap polls GetNetMap until the controller has assigned a VPN IP.
func fetchNetMap(
	ctx context.Context,
	natsClient *managementnats.NatsSignalService,
	agentName, pubKey, agentJWT string,
	privKey wgtypes.Key,
) (*infra.Peer, error) {
	getMapPayload, _ := json.Marshal(&dto.PeerDto{
		AppID:     agentName,
		PublicKey: pubKey,
		Token:     agentJWT,
	})
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		data, reqErr := natsClient.Request(ctx, "lattice.signals.peer", "GetNetMap", getMapPayload)
		if reqErr == nil {
			var msg infra.Message
			if json.Unmarshal(data, &msg) == nil && msg.Current != nil &&
				msg.Current.Address != nil && *msg.Current.Address != "" {
				peer := msg.Current
				peer.PrivateKey = privKey.String() // inject locally-generated key
				if peer.AppID == "" {
					peer.AppID = agentName
				}
				// Store JWT so node.Start() → GetNetworkMap can authenticate.
				peer.Token = agentJWT
				return peer, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("timed out waiting for VPN IP allocation")
}
