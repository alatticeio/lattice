# AI Chat UI 优化设计

**日期：** 2026-05-11
**范围：** `fronted/src/components/ai/` + `fronted/src/pages/ai/index.vue` + `fronted/src/stores/useAiStore.ts`

---

## 背景

当前 AI chat 页面存在以下问题：
- 消息布局不一致（用户消息和 AI 消息的宽度参照系不同）
- Markdown 渲染为手写正则，有边界 bug，代码块无高亮无复制按钮
- 对话侧边栏标题永远是"新对话"
- 缺少基础交互细节：时间戳、复制按钮、顶部标题栏
- 空状态欢迎页推荐问题布局过于简单

---

## 设计决策

### 1. 消息布局：用户气泡 + AI 正文

采用 ChatGPT 风格：
- 用户消息：右侧气泡，`bg-primary` 颜色，最宽 75%，右下角圆角收窄
- AI 消息：左侧无背景，直接展示为正文，在 `max-w-3xl` 容器内，顶部显示"Lattice AI · HH:mm"
- 所有消息统一在 `max-w-3xl mx-auto` 的限宽容器中，消除两种消息宽度参照系不一致的问题
- 顶部新增标题栏，显示当前对话标题 + 模型名

### 2. Markdown 渲染：marked + highlight.js

- 移除 `MessageBubble.vue` 中手写的 `renderMarkdown` 正则函数
- 引入 `marked`（Markdown 解析）+ `highlight.js`（语法高亮）
- 代码块渲染：顶部工具栏显示语言标识 + 复制按钮，暗色主题
- `marked` 配置使用 `highlight.js` 的 `highlightAuto` 作为高亮函数

### 3. 对话标题自动生成

- `useAiStore.ts` 的 `addUserMessage` **已经实现**：第一条用户消息自动截取前 40 字作为标题
- 无需改动 store，仅需侧边栏正确展示即可

### 4. 交互细节

- **代码块复制按钮**：点击后图标变为 ✓，2 秒后复原
- **消息复制按钮**：hover AI 消息时右上角出现复制图标，复制纯文本内容
- **时间戳**：每条消息 hover 时显示发送时间（HH:mm 格式）
- **顶部标题栏**：`ChatWindow` 顶部固定栏，显示当前对话标题（无对话时隐藏）

### 5. 空状态欢迎页重构

- 推荐问题改为列表布局（原为 2×2 grid）
- 按分类分组展示："常见问题"、"网络管理"各 2 条
- 每条带 icon + 完整问题文本，点击直接发送

---

## 受影响的文件

| 文件 | 改动类型 |
|------|--------|
| `fronted/src/components/ai/MessageBubble.vue` | 重写布局 + 替换 renderMarkdown + 加复制/时间戳 |
| `fronted/src/components/ai/ChatWindow.vue` | 加顶部标题栏 |
| `fronted/src/components/ai/ChatInput.vue` | 修复 nextTick 导入 bug |
| `fronted/src/components/ai/SuggestedPrompts.vue` | 重构为分组列表布局 |
| `fronted/src/stores/useAiStore.ts` | 无需改动（标题逻辑已存在） |
| `fronted/package.json` | 新增 `marked`、`highlight.js` 依赖 |

---

## 不在本次范围内

- 侧边栏折叠功能
- 对话重命名
- 消息编辑/重新生成
- 后端 AI 生成标题
