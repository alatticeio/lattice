1. 测试环境标准 (Environment)
   基础设施： 必须基于 k3d 创建轻量级集群。

工具链： 使用 chainsaw 或 kuttl 进行声明式测试，或者使用 Go 编写基于 sigs.k8s.io/e2e-framework 的测试。

网络环境： 必须模拟跨节点通信，验证 WireGuard 隧道在不同 Node 间的连通性。

2. 测试用例设计规范 (Test Case Design)
   每一个 E2E 任务必须包含以下三个维度的验证：

A. 资源生命周期 (CRUD)
创建： 提交 CRD 后，检查对应的物理 WireGuard 接口是否已在 Node 上创建。

更新： 修改 Peer 的 PublicKey 或 AllowedIPs，验证配置是否实时同步。

删除： 删除 CR 后，验证 Finalizer 是否执行，物理接口与路由表是否清理干净。

B. 连通性验证 (Connectivity)
Data Plane Check： 必须在 Pod 内部执行 ping 或 nc 测试，验证 Overlay 网络是否真正打通。

MTU 检查： 验证大包传输是否正常，确保 MSS Clamping 逻辑生效。

C. 鲁棒性验证 (Resilience)
重启测试： 模拟 Controller Pod 重启，验证状态恢复（Idempotency）。

配置漂移： 手动修改 Node 上的 WireGuard 配置，验证 Controller 是否能将其“自愈”回预期状态。

3. 代码与配置要求 (Implementation)
   GitHub Actions 集成： 必须提供对应的 .github/workflows/e2e.yaml 更新，确保新功能触发 CI。

日志收集： 测试失败时，必须自动 Dump 所有相关 Pod 的日志和 wg show 的输出。

超时控制： 每一个步骤必须设置合理的 timeout（默认 60s），严禁无限期等待。

4. Definition of Done for AI (验收清单)
   在提交 E2E 测试代码前，AI 必须确认：

[ ] 测试脚本可以在本地 make e2e-test 一键运行。

[ ] 测试用例不依赖特定的宿主机内核模块（除非已在 CI 环境预装）。

[ ] 所有的测试资源（Namespace, Pods）在测试结束后会自动清理。