# Wireflow Code Review Checklist
3. K8s 控制循环与幂等性 (Idempotency)
[ ] 逻辑闭环： Reconcile 函数是否覆盖了资源的所有状态（Created, Updated, Deleted）？

[ ] 状态重入： 如果程序在任意一行崩溃并重启，再次执行 Reconcile 是否会产生副作用（如重复创建接口）？

[ ] 限流与退避： 在发生错误时，是否返回了 requeueAfter 或带有 err 的返回，以利用系统的指数退避机制？

2. 资源清理与 Finalizers
   [ ] 残留检查： 当 DeletionTimestamp 非空时，是否先执行了物理设备（WireGuard Interface）的清理，才移除 Finalizer？

[ ] 超时处理： 清理逻辑是否设置了合理的超时，防止因为节点不可达导致 CR 永远卡在 Terminating 状态？

3. 状态与观测 (Status & Observability)
   [ ] Conditions 规范： 是否使用了 metav1.Condition 结构？Reason 字段是否使用了 UpperCamelCase 风格？

[ ] 事件上报： 关键行为（如 Key Rotation, Peer Connected）是否通过 EventRecorder 发送了 K8s Event？

[ ] 日志分级： 正常逻辑使用 log.V(4)，错误逻辑使用 log.Error，严禁在生产路径中使用 fmt.Println。

4. Go 性能与安全
   [ ] 内存/泄露： 所有的 Ticker、Channel 或 Goroutine 是否都有明确的退出机制（通过 ctx.Done()）？

[ ] 敏感信息： WireGuard 的私钥（PrivateKey）是否严禁出现在日志、Status 或非加密的 ConfigMap 中？

[ ] 空指针防护： 对 CR.Spec 中的可选字段（Optional Fields）是否做了 nil 检查？



- [ ] **Cleanup**: 进程退出时是否通过 `defer` 或信号处理清理了 `utun` 和防火墙规则？
- [ ] **Concurrency**: 在 `all-in-one` 模式下，多个 Goroutine 访问 DB 是否存在竞争？
- [ ] **Platform**: 涉及 `pfctl` 的代码在 Linux 下是否有对应的空实现或 `netlink` 实现？
- [ ] **Metrics**: 关键逻辑（如 Peer 状态切换）是否增加了 `prometheus.Counter`？
- [ ] **Logging**: 错误日志是否包含足够的 Context（如具体的系统调用参数）？
- [ ] **UX**: 新增的功能是否需要同步更新 Vue 前端界面？