package transport

import (
	"testing"
)

// mockProvisioner records method calls for verification.
type mockProvisioner struct {
	registerCalls []struct{ publicKey, allowedIPs string }
	setEpCalls    []struct{ publicKey, endpoint string; keepalive int }
	removeCalls   []string
	routeCalls    []struct{ address, iface string }
	setupNATCalls []string
}

func (m *mockProvisioner) AddPeer(pk, allowedIPs string) error {
	m.registerCalls = append(m.registerCalls, struct{ publicKey, allowedIPs string }{pk, allowedIPs})
	return nil
}
func (m *mockProvisioner) SetEndpoint(pk, endpoint string, ka int) error {
	m.setEpCalls = append(m.setEpCalls, struct{ publicKey, endpoint string; keepalive int }{pk, endpoint, ka})
	return nil
}
func (m *mockProvisioner) RemovePeer(pk string) error {
	m.removeCalls = append(m.removeCalls, pk)
	return nil
}
func (m *mockProvisioner) ApplyRoute(address, iface string) error {
	m.routeCalls = append(m.routeCalls, struct{ address, iface string }{address, iface})
	return nil
}
func (m *mockProvisioner) SetupNAT(iface string) error {
	m.setupNATCalls = append(m.setupNATCalls, iface)
	return nil
}

func TestWgConfigurator_RegisterPeer_Idempotent(t *testing.T) {
	mock := &mockProvisioner{}
	cfg := NewWGConfigurator(mock, mock)

	cfg.RegisterPeer("pk1", "10.0.0.1/32")
	cfg.RegisterPeer("pk1", "10.0.0.1/32") // duplicate, should be idempotent

	if len(mock.registerCalls) != 1 {
		t.Errorf("expected 1 register call, got %d", len(mock.registerCalls))
	}
}

func TestWgConfigurator_SetEndpoint(t *testing.T) {
	mock := &mockProvisioner{}
	cfg := NewWGConfigurator(mock, mock)

	cfg.SetEndpoint("pk1", "1.2.3.4:51820", 25)
	cfg.SetEndpoint("pk1", "5.6.7.8:51820", 0) // update endpoint

	if len(mock.setEpCalls) != 2 {
		t.Errorf("expected 2 SetEndpoint calls, got %d", len(mock.setEpCalls))
	}
	if mock.setEpCalls[1].endpoint != "5.6.7.8:51820" {
		t.Errorf("expected updated endpoint, got %s", mock.setEpCalls[1].endpoint)
	}
	if mock.setEpCalls[1].keepalive != 0 {
		t.Errorf("expected keepalive=0, got %d", mock.setEpCalls[1].keepalive)
	}
}

func TestWgConfigurator_RemovePeer(t *testing.T) {
	mock := &mockProvisioner{}
	cfg := NewWGConfigurator(mock, mock)

	cfg.RemovePeer("pk1")
	if len(mock.removeCalls) != 1 {
		t.Errorf("expected 1 remove call, got %d", len(mock.removeCalls))
	}
}

func TestWgConfigurator_NilOps(t *testing.T) {
	cfg := NewWGConfigurator(nil, nil)

	// All operations should succeed as no-ops
	if err := cfg.RegisterPeer("pk1", "10.0.0.1/32"); err != nil {
		t.Errorf("RegisterPeer with nil ops should not error: %v", err)
	}
	if err := cfg.SetEndpoint("pk1", "1.2.3.4:51820", 25); err != nil {
		t.Errorf("SetEndpoint with nil ops should not error: %v", err)
	}
	if err := cfg.RemovePeer("pk1"); err != nil {
		t.Errorf("RemovePeer with nil ops should not error: %v", err)
	}
	if err := cfg.ApplyRoute("10.0.0.1", "wg0"); err != nil {
		t.Errorf("ApplyRoute with nil ops should not error: %v", err)
	}
	if err := cfg.SetupNAT("wg0"); err != nil {
		t.Errorf("SetupNAT with nil ops should not error: %v", err)
	}
}
