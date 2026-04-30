// internal/server/transport/configurator.go
// Copyright 2026 The Lattice Authors, Inc.
// Licensed under the Apache License, Version 2.0.

package transport

import (
	"sync"
)

// ConnectionConfigurator defines the WireGuard configuration actions
// triggered by connection state transitions.
type ConnectionConfigurator interface {
	// RegisterPeer adds a peer entry to WireGuard (called on first SYN/ACK).
	RegisterPeer(publicKey, allowedIPs string) error

	// SetEndpoint updates the WireGuard peer endpoint (called on every connect).
	SetEndpoint(publicKey, endpoint string, persistentKeepalive int) error

	// RemovePeer removes a peer entry from WireGuard (called on Failed/Closed).
	RemovePeer(publicKey string) error

	// ApplyRoute adds or removes a kernel route for the peer's VPN address.
	ApplyRoute(address, iface string) error

	// SetupNAT configures NAT rules for the WireGuard interface.
	SetupNAT(iface string) error
}

// PeerOps is the minimal interface for WireGuard peer management.
type PeerOps interface {
	AddPeer(publicKey, allowedIPs string) error
	SetEndpoint(publicKey, endpoint string, persistentKeepalive int) error
	RemovePeer(publicKey string) error
}

// RouteOps is the minimal interface for routing and NAT.
type RouteOps interface {
	ApplyRoute(address, iface string) error
	SetupNAT(iface string) error
}

// wgConfigurator implements ConnectionConfigurator with idempotent semantics.
type wgConfigurator struct {
	mu       sync.Mutex
	peers    map[string]bool // track which peers have been registered
	peerOps  PeerOps
	routeOps RouteOps
}

// NewWGConfigurator creates a configurator. peerOps and routeOps may be nil
// (operations become no-ops, useful for testing).
func NewWGConfigurator(peerOps PeerOps, routeOps RouteOps) *wgConfigurator {
	return &wgConfigurator{
		peers:    make(map[string]bool),
		peerOps:  peerOps,
		routeOps: routeOps,
	}
}

func (c *wgConfigurator) RegisterPeer(publicKey, allowedIPs string) error {
	c.mu.Lock()
	if c.peers[publicKey] {
		c.mu.Unlock()
		return nil // idempotent
	}
	c.peers[publicKey] = true
	c.mu.Unlock()

	if c.peerOps == nil {
		return nil
	}
	return c.peerOps.AddPeer(publicKey, allowedIPs)
}

func (c *wgConfigurator) SetEndpoint(publicKey, endpoint string, persistentKeepalive int) error {
	if c.peerOps == nil {
		return nil
	}
	return c.peerOps.SetEndpoint(publicKey, endpoint, persistentKeepalive)
}

func (c *wgConfigurator) RemovePeer(publicKey string) error {
	c.mu.Lock()
	delete(c.peers, publicKey)
	c.mu.Unlock()

	if c.peerOps == nil {
		return nil
	}
	return c.peerOps.RemovePeer(publicKey)
}

func (c *wgConfigurator) ApplyRoute(address, iface string) error {
	if c.routeOps == nil {
		return nil
	}
	return c.routeOps.ApplyRoute(address, iface)
}

func (c *wgConfigurator) SetupNAT(iface string) error {
	if c.routeOps == nil {
		return nil
	}
	return c.routeOps.SetupNAT(iface)
}
