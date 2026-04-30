package resource

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// internal/platform/k8s/impersonator.go

type IdentityImpersonator struct {
	baseConfig *rest.Config // 基础高权限配置
	scheme     *runtime.Scheme
}

func NewIdentityImpersonator() (*IdentityImpersonator, error) {
	// 类似于 ctrl.GetConfigOrDie()，但这里我们处理错误
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	return &IdentityImpersonator{
		baseConfig: cfg,
		scheme:     scheme,
	}, nil
}

// NamespaceAccessor 为特定的工作区生成一个受限的客户端
// wsID: 工作区 ID
// role: 业务角色 (admin, member, viewer)
func (i *IdentityImpersonator) NamespaceAccessor(wsID string, role string) (client.Client, error) {
	// 1. 深度拷贝原始的 rest.Config，避免修改全局配置
	config := rest.CopyConfig(i.baseConfig)

	// 2. 构造身份面具 (Impersonation)
	// 用户名：用于审计日志，wf-user-101
	// 组：这是 RBAC 校验的关键，wf-group-101-admin
	config.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("wf-user-%s", wsID),
		Groups:   []string{fmt.Sprintf("wf-group-%s-%s", wsID, role)},
	}

	// 3. 创建客户端
	// 注意：这里我们使用 client.New 而不是 i.client
	// 因为 i.client 通常是带 Cache 的，而模拟访问必须是 Direct Client (直接请求 API Server)
	scopedClient, err := client.New(config, client.Options{
		Scheme: i.scheme, // 必须传入 Scheme，否则无法解析原生资源或 CRD
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create scoped k8s client: %w", err)
	}

	return scopedClient, nil
}
