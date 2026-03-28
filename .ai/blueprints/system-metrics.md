1. 全局数据协议蓝图 (The Blueprint)
   在让 AI 写代码前，先给它这个协议，确保前后端联调无缝：

Project: Monitoring Dashboard
Data Protocol: > - Format: JSON

Metrics Object: { "timestamp": "ISO8601", "metrics": [ { "name": "cpu_usage", "value": 45.2, "unit": "%", "labels": {"node": "edge-01"} } ] }

API Style: RESTful (GET /api/v1/metrics/realtime)

2. 后端 Prompt：Golang 指标采集与 API
   针对你的背景（K8s, Go, 后端架构），这个 Prompt 侧重于高性能和标准输出。

Prompt:
"请作为资深 Go 开发专家，参考 .ai/system-prompt.md 规范，实现一个监控指标后端。

任务要求：

指标采集： 使用 shirou/gopsutil 库采集本地 CPU、内存、磁盘和网络流量。

数据处理： 实现一个固定窗口的内存缓存（Ring Buffer），存储最近 60 分钟的数据。

API 实现： >    - 使用 Gin 或 Fiber 框架。

提供 GET /api/v1/metrics/history 接口，支持通过查询参数 ?metric_name=cpu 过滤。

健壮性： 采集逻辑必须在独立的 Goroutine 中运行，使用 context 控制生命周期，并处理可能的采集超时。

输出： 按照预定义的 JSON 协议格式返回数据。"

3. 前端 Prompt：Vue 3 + Tailwind + DaisyUI 展示
   利用你擅长的 Vue 栈，让 AI 生成响应式且美观的看板。

Prompt:
"请作为高级前端工程师，使用 Vue 3 (Composition API)、Tailwind CSS 和 DaisyUI 构建一个监控大屏。

任务要求：

组件化： >    - 创建一个 MetricCard 组件，展示当前实时数值（带动画效果）。

创建一个 MetricChart 组件，使用 ECharts 或 Chart.js 展示历史趋势折线图。

数据交互： >    - 使用 Axios 定时（每 5 秒）从后端 API 获取最新数据。

实现数据响应式更新，确保图表平滑过渡。

UI 设计： >    - 采用暗黑模式（Dark Mode）。

布局要求响应式，在手机端显示单列，在 PC 端显示多栏栅格。

使用 DaisyUI 的 Stat 组件展示关键指标。

状态管理： 使用 Pinia 存储全局监控状态。"

4. 联调审核 Prompt (The Connector)
   当你拿到了两端的代码，用这个 Prompt 让 AI 帮你做最后的检查：

Prompt:
"我现在的 Go 后端监听在 :8080 端口，Vue 前端开发环境在 :5173。

请帮我配置后端的 CORS 跨域策略。

请帮我编写一个 docker-compose.yaml 文件，能够一键拉起这两个服务，并配置好前端反向代理（Vite Proxy）指向后端。"

给“审核员”你的特别建议：
关于指标精度： 在审核 Go 代码时，注意看 AI 是否处理了 浮点数精度问题（建议在 API 返回时保留两位小数）。

关于前端性能： 监控系统最怕“内存泄漏”。审核 Vue 代码时，重点看 onUnmounted 生命周期里是否清除了 setInterval 和 echarts.dispose()。

关于扩展性： 既然你在考虑 NATS 和边缘节点，你可以后续追加一个 Prompt：“请将后端的数据来源重构为从 NATS Subject 订阅，而不是直接读取本地 gopsutil。”