# Superpowers Docs

内部开发文档，记录 Lattice 的设计决策、实现规划和故障历史。
不面向最终用户，不随版本发布。

---

## 目录结构

```
docs/superpowers/
├── specs/      设计规范
├── plans/      实现计划
├── adr/        架构决策记录
├── rca/        故障分析
├── faq/        概念澄清与讨论记录
└── archive/    已废弃的旧 spec
```

---

## specs/ — 设计规范

**是什么**：描述系统当前状态的技术文档，回答"它是什么、怎么工作"。

**特点**：
- 从代码反向推导，与代码保持同步
- 代码变了就更新，是持续维护的活文档
- 是新成员理解某个子系统的首选入口

**命名**：`YYYY-MM-DD-<topic>-design.md`

**示例**：
- `2026-05-18-ice-handshake-design.md` — ICE 握手协议与互斥量规范
- `2026-05-18-sandbox-agent-architecture.md` — gVisor 沙箱架构

---

## plans/ — 实现计划

**是什么**：任务拆解文档，记录"要做什么、怎么分步做"。通常在开始编码前写。

**特点**：
- 任务完成后自然过期，但保留作为历史参考
- 不需要随代码更新，完成后原地留存即可
- 可以在文档顶部加 `[done]` 标注已完成

**命名**：`YYYY-MM-DD-<feature>-plan.md`

---

## adr/ — 架构决策记录（ADR）

**是什么**：记录重要架构选型的决策理由，回答"为什么这样做而不是那样做"。

**特点**：
- **永远不删、永远不修改**，只能新增或标记为 Superseded
- 即使决策后来被推翻，原 ADR 仍然保留，加一条新 ADR 说明变更
- 格式极简：背景 → 决策 → 原因 → 后果

**命名**：`NNNN-<decision-slug>.md`（四位递增序号）

**格式模板**：
```markdown
# ADR-NNNN: <标题>

**状态**: Accepted | Superseded by ADR-XXXX | Deprecated
**日期**: YYYY-MM-DD

## 背景
遇到了什么问题，需要做什么选择。

## 决策
我们选择了 X。

## 原因
- 原因一
- 原因二

## 后果
- 正面影响
- 需要接受的权衡
```

**示例**：
- `0001-gvisor-library-vs-runsc.md` — 选择 gVisor library 模式而非 runsc

---

## rca/ — 故障分析（RCA / Post-mortem）

**是什么**：记录具体 bug 或故障的排查过程、根因和修复验证。

**特点**：
- **永远不删**，是系统演进的历史档案
- 帮助未来遇到类似问题的人快速定位根因
- 不需要与代码同步，是历史事件的快照

**命名**：`YYYY-MM-DD-<issue-slug>.md`

**建议包含**：
- 症状描述
- 根因分析（数据证据）
- 修复方案
- 验证过程

**示例**：
- `2026-05-17-ice-agent-nil-race.md` — ICE agent nil 竞态与 ACK 时序修复

---

## archive/ — 已废弃的旧 spec

**是什么**：曾经权威、现已被新版 spec 完全取代的设计文档。

**特点**：
- 只存放曾经是 `specs/` 中的正式文档
- 不存放调试记录、草稿或过期计划（那些分别属于 rca/ 和 plans/）
- 保留是为了追溯设计演进历史

**示例**：
- `2026-05-16-agent-sandbox-network-isolation-design.md` — 被 `2026-05-18-sandbox-agent-architecture.md` 取代

---

## faq/ — 概念澄清与讨论记录

**是什么**：记录头脑风暴、概念澄清和日常讨论中沉淀的结论。回答"这段讨论后来怎么样了"。

**特点**：
- 通常是对话驱动的，不是正式文档
- 内容可能涵盖：能力边界澄清、不同路径对比、常见误解解释
- 不需要与代码实时同步，但要反映当时的理解状态
- 后续如果有正式设计，应该在 specs/ 或 adr/ 中另起文档

**命名**：`YYYY-MM-DD-<topic>-discussion.md`

**示例**：
- `2026-05-18-agent-protection-levels-discussion.md` — Agent 防护三层模型讨论
