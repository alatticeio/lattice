package infra

import (
	"testing"
)

// MockProvisioner is used for testing logic flows without executing real commands
type MockProvisioner struct {
	LastRule FirewallRule
	Applied  bool
}

func (m *MockProvisioner) Name() string { return "mock" }
func (m *MockProvisioner) Provision(rule FirewallRule) error {
	m.LastRule = rule
	m.Applied = true
	return nil
}
func (m *MockProvisioner) Cleanup() error { return nil }

func TestAgent_ProvisioningLogic(t *testing.T) {
	// 1. Prepare mock data (from Controller structs)
	fakeRule := FirewallRule{
		PolicyName: "test-policy",
		Ingress: []TrafficRule{
			{Peers: []string{"192.168.1.1"}, Port: 80, Protocol: "tcp"},
		},
	}

	// 2. Use mock executor
	mock := &MockProvisioner{}

	// 3. Execute
	err := mock.Provision(fakeRule)

	// 4. Assert
	if err != nil {
		t.Fatalf("expected success, but got error: %v", err)
	}
	if mock.LastRule.Ingress[0].Port != 80 {
		t.Errorf("port conversion error, expected 80, got %d", mock.LastRule.Ingress[0].Port)
	}
}
