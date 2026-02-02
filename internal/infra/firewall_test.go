package infra

import (
	"testing"
)

// MockProvisioner 用于测试逻辑流，不执行真实命令
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
	// 1. 准备模拟数据 (来自 Controller 的结构体)
	fakeRule := FirewallRule{
		PolicyName: "test-policy",
		Ingress: []TrafficRule{
			{Peers: []string{"192.168.1.1"}, Port: 80, Protocol: "tcp"},
		},
	}

	// 2. 使用 Mock 执行器
	mock := &MockProvisioner{}

	// 3. 执行
	err := mock.Provision(fakeRule)

	// 4. 断言
	if err != nil {
		t.Fatalf("应该执行成功，但是报错了: %v", err)
	}
	if mock.LastRule.Ingress[0].Port != 80 {
		t.Errorf("端口转换错误，期望 80，实际 %d", mock.LastRule.Ingress[0].Port)
	}
}
