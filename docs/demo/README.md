# Lattice E2E Demo (vhs)

## 前提

```bash
# macOS
brew install vhs

# 其他系统: https://github.com/charmbracelet/vhs
```

## 生成 GIF

```bash
cd docs
vhs < demo/e2e-sandbox-demo.tape
```

输出: `public/demo/sandbox-e2e.gif`

## 在 Markdown 中使用

```markdown
![Lattice Sandbox E2E Demo](/demo/sandbox-e2e.gif)
```

## 脚本对应关系

`e2e-sandbox-demo.tape` 的每一步对应 `test/e2e/agent_sandbox_test.go` 中的场景：

| Tape 步骤 | e2e Scenario |
|-----------|-------------|
| Step 1-2 | BeforeAll: login + create workspace + enrollment token |
| Step 3 | BeforeAll: deploy companion agent (`lattice up`) |
| Step 4 | BeforeAll: deploy sandbox pod (`lattice sandbox start`) |
| Scenario 1 | AgentIdentity CRD Active phase 验证 |
| Scenario 2 | Sandbox → Companion via WireGuard overlay |
| Scenario 3 | Companion → Sandbox via ForwardListener |
| Scenario 4 | Non-lattice IP blocked |
| Scenario 5 | Policy DENY blocks connection |
| Scenario 6 | Audit log contains allow + drop events |
| Scenario 7 | Agent revocation → phase=Revoked |

> **注意:** 本脚本的输出为预录文本（mock output），不依赖真实 K8s 集群。
> 如需要真实流量录屏，请在 GitHub Actions e2e job 中嵌入录制步骤。
