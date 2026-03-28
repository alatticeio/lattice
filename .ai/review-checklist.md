1. K8s 控制循环与幂等性 (Idempotency)
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