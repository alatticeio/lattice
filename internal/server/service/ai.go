package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/llm"
	managementnats "github.com/alatticeio/lattice/internal/server/nats"
	"github.com/alatticeio/lattice/internal/server/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ── Public types ──────────────────────────────────────────────────────────────

type ChatRequest struct {
	Message     string        `json:"message"`
	WorkspaceID string        `json:"workspaceId"`
	History     []ChatMessage `json:"history"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent is the SSE payload sent to the client.
type StreamEvent struct {
	Type    string          `json:"type"`              // token | tool_use | preview | error | done
	Content string          `json:"content,omitempty"` // type=token
	Tool    string          `json:"tool,omitempty"`    // type=tool_use
	Input   json.RawMessage `json:"input,omitempty"`   // type=tool_use
	Error   string          `json:"error,omitempty"`   // type=error
}

// StreamWriter receives events from the AI service and forwards them to the HTTP layer.
type StreamWriter interface {
	Write(event StreamEvent) error
}

// AuditFinding is a single security issue found during an audit scan.
type AuditFinding struct {
	Severity    string `json:"severity"` // high | medium | low
	Rule        string `json:"rule"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// AuditReport is the result of a workspace security audit.
type AuditReport struct {
	Score       int            `json:"score"`
	GeneratedAt string         `json:"generatedAt"`
	Findings    []AuditFinding `json:"findings"`
}

// AIService is the main entry point for all AI features.
type AIService interface {
	Chat(ctx context.Context, req *ChatRequest, out StreamWriter) error
	Audit(ctx context.Context, workspaceID string) (*AuditReport, error)
	// Debug performs AI-powered root cause analysis using network snapshots.
	Debug(ctx context.Context, req *DebugRequest, out StreamWriter) error
	// ListTools returns all tool definitions (used by /api/v1/ai/tools and MCP).
	ListTools(namespace string) []llm.Tool
	// ExecuteTool dispatches a single tool call. Public for MCP server use.
	ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error)
}

// DebugRequest defines the time-travel debugging query parameters.
type DebugRequest struct {
	WorkspaceID string    `json:"workspaceId"`
	Question    string    `json:"question"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
}

// ── Implementation ────────────────────────────────────────────────────────────

type aiService struct {
	logger       *log.Logger
	llm          llm.Client
	store        store.Store
	k8s          *resource.Client
	presence     *managementnats.NodePresenceStore
	maxToolCalls int
	workflow     WorkflowService
	autoApprove  map[string]bool                 // tool name -> skip approval
	intentSvc    IntentService                   // optional, Pro-only
	snapStore    store.NetworkSnapshotRepository // optional, Pro-only
}

// SetSnapStore attaches the NetworkSnapshot repository to an existing AIService.
func SetSnapStore(svc AIService, snapStore store.NetworkSnapshotRepository) {
	if as, ok := svc.(*aiService); ok {
		as.snapStore = snapStore
	}
}

// SetIntentService attaches the Intent Engine to an existing AIService.
// Used by the server to inject intent tools after both services are created.
func SetIntentService(svc AIService, intentSvc IntentService) {
	if as, ok := svc.(*aiService); ok {
		as.intentSvc = intentSvc
	}
}

// NewAIService is the existing constructor (unchanged signature for compatibility).
func NewAIService(
	llmClient llm.Client,
	st store.Store,
	k8s *resource.Client,
	presence *managementnats.NodePresenceStore,
	maxToolCalls int,
) AIService {
	return NewAIServiceWithWorkflow(llmClient, st, k8s, presence, maxToolCalls, nil, nil)
}

// NewAIServiceWithWorkflow is the full constructor used by the server.
func NewAIServiceWithWorkflow(
	llmClient llm.Client,
	st store.Store,
	k8s *resource.Client,
	presence *managementnats.NodePresenceStore,
	maxToolCalls int,
	wf WorkflowService,
	autoApprove map[string]bool,
) AIService {
	if maxToolCalls <= 0 {
		maxToolCalls = 5
	}
	if autoApprove == nil {
		autoApprove = map[string]bool{}
	}
	return &aiService{
		logger:       log.GetLogger("ai-service"),
		llm:          llmClient,
		store:        st,
		k8s:          k8s,
		presence:     presence,
		maxToolCalls: maxToolCalls,
		workflow:     wf,
		autoApprove:  autoApprove,
	}
}

// ── Chat ──────────────────────────────────────────────────────────────────────

func (s *aiService) Chat(ctx context.Context, req *ChatRequest, out StreamWriter) error {
	ws, err := s.store.Workspaces().GetByID(ctx, req.WorkspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	system, err := s.buildSystemPrompt(ctx, ws.ID, ws.Namespace, ws.DisplayName)
	if err != nil {
		s.logger.Warn("failed to build system prompt, using minimal version", "err", err)
		system = baseSystemPrompt
	}

	// Build message history
	msgs := make([]llm.Message, 0, len(req.History)+1)
	for _, h := range req.History {
		msgs = append(msgs, llm.Message{Role: h.Role, Content: h.Content})
	}
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: req.Message})

	tools := s.ListTools(ws.Namespace)

	// Agentic loop
	for i := 0; i < s.maxToolCalls; i++ {
		llmReq := &llm.Request{
			System:    system,
			Messages:  msgs,
			Tools:     tools,
			MaxTokens: 4096,
		}

		resp, err := s.llm.Complete(ctx, llmReq)
		if err != nil {
			_ = out.Write(StreamEvent{Type: "error", Error: err.Error()})
			return err
		}

		if !resp.HasToolCalls() {
			// Final text response
			_ = out.Write(StreamEvent{Type: "token", Content: resp.Content})
			_ = out.Write(StreamEvent{Type: "done"})
			return nil
		}

		// Execute tool calls
		toolResultMsg := llm.Message{Role: llm.RoleTool}
		assistantMsg := llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}

		for _, tc := range resp.ToolCalls {
			_ = out.Write(StreamEvent{Type: "tool_use", Tool: tc.Name, Input: tc.Input})

			result, toolErr := s.ExecuteTool(ctx, ws.Namespace, tc.Name, tc.Input)
			if toolErr != nil {
				result = fmt.Sprintf("error: %s", toolErr.Error())
			}
			toolResultMsg.ToolResults = append(toolResultMsg.ToolResults, llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    result,
			})
		}

		msgs = append(msgs, assistantMsg, toolResultMsg)
	}

	// Exhausted tool call budget — ask LLM for final answer without tools
	llmReq := &llm.Request{
		System:    system,
		Messages:  msgs,
		MaxTokens: 4096,
	}
	resp, err := s.llm.Complete(ctx, llmReq)
	if err != nil {
		_ = out.Write(StreamEvent{Type: "error", Error: err.Error()})
		return err
	}
	_ = out.Write(StreamEvent{Type: "token", Content: resp.Content})
	_ = out.Write(StreamEvent{Type: "done"})
	return nil
}

// ── Audit ─────────────────────────────────────────────────────────────────────

func (s *aiService) Audit(ctx context.Context, workspaceID string) (*AuditReport, error) {
	ws, err := s.store.Workspaces().GetByID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	findings := s.runAuditRules(ctx, ws.Namespace)

	// Ask LLM to generate human-readable descriptions (best effort)
	if len(findings) > 0 {
		s.enrichFindingsWithLLM(ctx, findings)
	}

	score := 100
	for _, f := range findings {
		switch f.Severity {
		case "high":
			score -= 15
		case "medium":
			score -= 8
		case "low":
			score -= 3
		}
	}
	if score < 0 {
		score = 0
	}

	return &AuditReport{
		Score:    score,
		Findings: findings,
	}, nil
}

func (s *aiService) enrichFindingsWithLLM(ctx context.Context, findings []AuditFinding) {
	type findingSummary struct {
		Rule     string `json:"rule"`
		Severity string `json:"severity"`
		Resource string `json:"resource"`
	}
	summaries := make([]findingSummary, len(findings))
	for i, f := range findings {
		summaries[i] = findingSummary{Rule: f.Rule, Severity: f.Severity, Resource: f.Resource}
	}
	summaryJSON, _ := json.Marshal(summaries)

	prompt := fmt.Sprintf(`以下是 Lattice 网络安全扫描发现的问题列表（JSON 格式）：
%s

请为每个问题生成简洁的中文说明（description）和修复建议（suggestion），
以 JSON 数组返回，字段包含 rule、description、suggestion。
只返回 JSON，不要其他内容。`, string(summaryJSON))

	resp, err := s.llm.Complete(ctx, &llm.Request{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 2048,
	})
	if err != nil {
		s.logger.Warn("LLM enrichment failed, using default descriptions", "err", err)
		return
	}

	var enriched []struct {
		Rule        string `json:"rule"`
		Description string `json:"description"`
		Suggestion  string `json:"suggestion"`
	}
	// Extract JSON from response (may have markdown fences)
	content := strings.TrimSpace(resp.Content)
	if idx := strings.Index(content, "["); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "]"); idx >= 0 {
		content = content[:idx+1]
	}
	if err := json.Unmarshal([]byte(content), &enriched); err != nil {
		s.logger.Warn("failed to parse LLM enrichment", "err", err)
		return
	}
	byRule := make(map[string]struct{ desc, sug string })
	for _, e := range enriched {
		byRule[e.Rule] = struct{ desc, sug string }{e.Description, e.Suggestion}
	}
	for i := range findings {
		if e, ok := byRule[findings[i].Rule]; ok {
			findings[i].Description = e.desc
			findings[i].Suggestion = e.sug
		}
	}
}

// ── Audit rules ───────────────────────────────────────────────────────────────

func (s *aiService) runAuditRules(ctx context.Context, namespace string) []AuditFinding {
	var findings []AuditFinding

	var policyList v1alpha1.LatticePolicyList
	if err := s.k8s.GetAPIReader().List(ctx, &policyList, client.InNamespace(namespace)); err != nil {
		s.logger.Warn("audit: failed to list policies", "err", err)
	}

	var peerList v1alpha1.LatticePeerList
	if err := s.k8s.GetAPIReader().List(ctx, &peerList, client.InNamespace(namespace)); err != nil {
		s.logger.Warn("audit: failed to list peers", "err", err)
	}

	// Rule 1: allow-all policy
	for _, p := range policyList.Items {
		if p.Spec.Action == "ALLOW" &&
			len(p.Spec.Ingress) == 0 && len(p.Spec.Egress) == 0 &&
			p.Spec.PeerSelector.MatchLabels == nil && p.Spec.PeerSelector.MatchExpressions == nil {
			findings = append(findings, AuditFinding{
				Severity: "high",
				Rule:     "allow-all-detected",
				Resource: "policy/" + p.Name,
			})
		}
	}

	// Rule 2: long-offline peers (presence store only tracks recent heartbeats; nil = never seen / long offline)
	if s.presence != nil {
		for _, peer := range peerList.Items {
			status, _ := s.presence.GetStatus(peer.Spec.AppId)
			if status == "offline" || status == "" {
				findings = append(findings, AuditFinding{
					Severity: "low",
					Rule:     "unused-peer",
					Resource: "peer/" + peer.Name,
				})
			}
		}
	}

	// Rule 3: no policies at all (network is fully open or fully blocked with no intent captured)
	if len(policyList.Items) == 0 && len(peerList.Items) > 0 {
		findings = append(findings, AuditFinding{
			Severity: "medium",
			Rule:     "no-policies",
			Resource: "namespace/" + namespace,
		})
	}

	return findings
}

// ── System prompt ─────────────────────────────────────────────────────────────

const baseSystemPrompt = `你是 Lattice 的网络管理助手，帮助用户管理基于 WireGuard 的私有网络。

## Lattice 核心概念
- LatticeNetwork: 一个隔离的 WireGuard 网络，每个网络有独立 CIDR（如 10.100.1.0/24）
- LatticePeer: 网络中的节点，代表一台设备或服务
- LatticePolicy: 访问控制策略，控制哪些 Peer 之间可以通信（默认拒绝）

## 操作规范
- 查询操作：直接返回结果
- 创建/修改/删除操作：先展示变更预览，用户确认后才能执行
- 不确定的操作：先询问用户意图，再给出方案`

func (s *aiService) buildSystemPrompt(ctx context.Context, wsID, namespace, wsName string) (string, error) {
	var peerList v1alpha1.LatticePeerList
	_ = s.k8s.GetAPIReader().List(ctx, &peerList, client.InNamespace(namespace))

	var policyList v1alpha1.LatticePolicyList
	_ = s.k8s.GetAPIReader().List(ctx, &policyList, client.InNamespace(namespace))

	var netList v1alpha1.LatticeNetworkList
	_ = s.k8s.GetAPIReader().List(ctx, &netList, client.InNamespace(namespace))

	activePeers := 0
	if s.presence != nil {
		for _, p := range peerList.Items {
			status, _ := s.presence.GetStatus(p.Spec.AppId)
			if status == "online" {
				activePeers++
			}
		}
	}

	return fmt.Sprintf(`%s

## 当前工作区状态
- 工作区: %s（ID: %s，命名空间: %s）
- 网络数量: %d
- Peer 总数: %d（在线: %d）
- 策略条数: %d`,
		baseSystemPrompt,
		wsName, wsID, namespace,
		len(netList.Items),
		len(peerList.Items), activePeers,
		len(policyList.Items),
	), nil
}

// ── Tool registry ─────────────────────────────────────────────────────────────

func (s *aiService) ListTools(namespace string) []llm.Tool {
	return []llm.Tool{
		{
			Name:        "list_peers",
			Description: "列出工作区内所有 WireGuard Peer 节点，包含在线状态、IP 地址、标签",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "list_policies",
			Description: "列出工作区内所有访问控制策略（LatticePolicy）",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "list_networks",
			Description: "列出工作区内所有 WireGuard 网络及其 CIDR",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "check_connectivity",
			Description: "检查两个 Peer 之间是否有策略允许通信，返回匹配到的策略或 'blocked'",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{
					"from":{"type":"string","description":"源 Peer 名称"},
					"to":{"type":"string","description":"目标 Peer 名称"}
				},
				"required":["from","to"]
			}`),
		},
		{
			Name:        "create_policy",
			Description: "创建访问控制策略（LatticePolicy CRD）。写操作，需要管理员审批（auto_approve=false 时）。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name":    {"type": "string", "description": "策略名称（K8s 资源名，小写字母+连字符）"},
					"network": {"type": "string", "description": "所属网络名称"},
					"action":  {"type": "string", "enum": ["ALLOW", "DENY"], "description": "允许或拒绝"},
					"peerSelector": {
						"type": "object",
						"description": "目标 Peer 的标签选择器，格式同 K8s LabelSelector",
						"properties": {
							"matchLabels": {"type": "object", "additionalProperties": {"type": "string"}}
						}
					}
				},
				"required": ["name", "network", "action"]
			}`),
		},
		{
			Name:        "delete_peer",
			Description: "删除 WireGuard Peer 节点（LatticePeer CRD）。写操作，需要审批。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Peer 名称"}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "delete_policy",
			Description: "删除访问控制策略（LatticePolicy CRD）。写操作，需要审批。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "策略名称"}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "create_peer",
			Description: "创建 WireGuard Peer 节点（LatticePeer CRD）。写操作，需要审批。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name":      {"type": "string", "description": "Peer 名称（K8s 资源名）"},
					"network":   {"type": "string", "description": "所属网络名称"},
					"appId":     {"type": "string", "description": "应用标识符（唯一 ID）"},
					"platform":  {"type": "string", "description": "平台类型，如 linux/windows/darwin"},
					"labels":    {"type": "object", "additionalProperties": {"type": "string"}, "description": "K8s 标签"}
				},
				"required": ["name", "network", "appId"]
			}`),
		},
		{
			Name:        "plan_network_change",
			Description: "（Pro）将自然语言意图转换为网络变更计划，返回 diff 预览。变更不会立即执行，需要调用 apply_network_change 确认。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"intent": {"type": "string", "description": "用自然语言描述你想要的网络状态，例如「允许 frontend 访问 api-gateway」"}
				},
				"required": ["intent"]
			}`),
		},
		{
			Name:        "apply_network_change",
			Description: "（Pro）执行之前 plan_network_change 返回的变更计划（需要管理员审批）。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"plan_id": {"type": "string", "description": "plan_network_change 返回的 planId"}
				},
				"required": ["plan_id"]
			}`),
		},
		// ── Time-Travel Debug tools (Pro) ─────────────────────────────
		{
			Name:        "list_snapshots",
			Description: "列出指定时间范围内的网络状态快照索引（不含完整数据）",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"from": {"type": "string", "format": "date-time"},
					"to":   {"type": "string", "format": "date-time"},
					"triggerType": {"type": "string", "description": "过滤: policy_change|peer_online|peer_offline|workflow_executed"}
				},
				"required": ["from", "to"]
			}`),
		},
		{
			Name:        "get_snapshot",
			Description: "获取指定快照 ID 的完整网络状态（peers、policies、networks）",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		},
		{
			Name:        "diff_snapshots",
			Description: "对比两个快照之间的变更（哪些策略被创建/删除/修改）",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"from_id": {"type": "string"},
					"to_id":   {"type": "string"}
				},
				"required": ["from_id", "to_id"]
			}`),
		},
		{
			Name:        "check_connectivity_at",
			Description: "在指定快照的策略状态下，检查两个 Peer 之间是否有连通性",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"snapshot_id": {"type": "string"},
					"from": {"type": "string", "description": "源 Peer 名称"},
					"to":   {"type": "string", "description": "目标 Peer 名称"}
				},
				"required": ["snapshot_id", "from", "to"]
			}`),
		},
	}
}
func (s *aiService) ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
	switch name {
	case "list_peers":
		return s.toolListPeers(ctx, namespace)
	case "list_policies":
		return s.toolListPolicies(ctx, namespace)
	case "list_networks":
		return s.toolListNetworks(ctx, namespace)
	case "check_connectivity":
		var args struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid input: %w", err)
		}
		return s.toolCheckConnectivity(ctx, namespace, args.From, args.To)
	case "create_policy":
		return s.toolCreatePolicy(ctx, namespace, input)
	case "delete_peer":
		return s.toolDeletePeer(ctx, namespace, input)
	case "delete_policy":
		return s.toolDeletePolicy(ctx, namespace, input)
	case "create_peer":
		return s.toolCreatePeer(ctx, namespace, input)
	case "plan_network_change":
		return s.toolPlanNetworkChange(ctx, namespace, input)
	case "apply_network_change":
		return s.toolApplyNetworkChange(ctx, input)
	case "list_snapshots":
		return s.toolListSnapshots(ctx, namespace, input)
	case "get_snapshot":
		return s.toolGetSnapshot(ctx, input)
	case "diff_snapshots":
		return s.toolDiffSnapshots(ctx, input)
	case "check_connectivity_at":
		return s.toolCheckConnectivityAt(ctx, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *aiService) toolListPeers(ctx context.Context, namespace string) (string, error) {
	var list v1alpha1.LatticePeerList
	if err := s.k8s.GetAPIReader().List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个 Peer：\n", len(list.Items)))
	for _, p := range list.Items {
		status := "未知"
		lastSeen := ""
		if s.presence != nil {
			st, ls := s.presence.GetStatus(p.Spec.AppId)
			status = st
			if ls != nil {
				lastSeen = " 最后在线: " + ls.Format("2006-01-02 15:04:05")
			}
		}
		addr := ""
		if p.Status.AllocatedAddress != nil {
			addr = " IP: " + *p.Status.AllocatedAddress
		}
		lbls := ""
		if len(p.Labels) > 0 {
			lbls = fmt.Sprintf(" 标签: %v", p.Labels)
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]%s%s%s\n", p.Name, status, addr, lastSeen, lbls))
	}
	return sb.String(), nil
}

func (s *aiService) toolListPolicies(ctx context.Context, namespace string) (string, error) {
	var list v1alpha1.LatticePolicyList
	if err := s.k8s.GetAPIReader().List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 条策略：\n", len(list.Items)))
	for _, p := range list.Items {
		sb.WriteString(fmt.Sprintf("- %s [%s] 网络: %s\n", p.Name, p.Spec.Action, p.Spec.Network))
		if len(p.Spec.Ingress) > 0 {
			sb.WriteString(fmt.Sprintf("  Ingress 规则: %d 条\n", len(p.Spec.Ingress)))
		}
		if len(p.Spec.Egress) > 0 {
			sb.WriteString(fmt.Sprintf("  Egress 规则: %d 条\n", len(p.Spec.Egress)))
		}
		if p.Spec.PeerSelector.MatchLabels != nil {
			sb.WriteString(fmt.Sprintf("  目标 Peer 标签: %v\n", p.Spec.PeerSelector.MatchLabels))
		}
	}
	return sb.String(), nil
}

func (s *aiService) toolListNetworks(ctx context.Context, namespace string) (string, error) {
	var list v1alpha1.LatticeNetworkList
	if err := s.k8s.GetAPIReader().List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个网络：\n", len(list.Items)))
	for _, n := range list.Items {
		cidr := ""
		if n.Status.ActiveCIDR != "" {
			cidr = " CIDR: " + n.Status.ActiveCIDR
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]%s\n", n.Name, n.Status.Phase, cidr))
	}
	return sb.String(), nil
}

func (s *aiService) toolCheckConnectivity(ctx context.Context, namespace, from, to string) (string, error) {
	// Get source peer labels
	var fromPeer v1alpha1.LatticePeer
	if err := s.k8s.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: namespace, Name: from}, &fromPeer); err != nil {
		return fmt.Sprintf("找不到 Peer %q", from), nil
	}
	var toPeer v1alpha1.LatticePeer
	if err := s.k8s.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: namespace, Name: to}, &toPeer); err != nil {
		return fmt.Sprintf("找不到 Peer %q", to), nil
	}

	// List policies and check if any ALLOW policy matches this pair
	var policyList v1alpha1.LatticePolicyList
	if err := s.k8s.GetAPIReader().List(ctx, &policyList, client.InNamespace(namespace)); err != nil {
		return "", err
	}

	var matched []string
	toLabels := labels.Set(toPeer.Labels)
	fromLabels := labels.Set(fromPeer.Labels)

	for _, p := range policyList.Items {
		if p.Spec.Action != "ALLOW" {
			continue
		}
		// Check if 'to' peer matches the policy's PeerSelector
		sel, err := metav1.LabelSelectorAsSelector(&p.Spec.PeerSelector)
		if err != nil {
			continue
		}
		if !sel.Matches(toLabels) {
			continue
		}
		// Check if any ingress rule allows 'from'
		for _, rule := range p.Spec.Ingress {
			for _, ps := range rule.From {
				if ps.PeerSelector == nil {
					matched = append(matched, p.Name)
					goto next
				}
				fromSel, err := metav1.LabelSelectorAsSelector(ps.PeerSelector)
				if err != nil {
					continue
				}
				if fromSel.Matches(fromLabels) {
					matched = append(matched, p.Name)
					goto next
				}
			}
		}
		// Policy has no ingress rules: allow all inbound to matched peers
		if len(p.Spec.Ingress) == 0 {
			matched = append(matched, p.Name)
		}
	next:
	}

	if len(matched) == 0 {
		return fmt.Sprintf("blocked: %s → %s 没有匹配的 ALLOW 策略", from, to), nil
	}
	return fmt.Sprintf("allowed: %s → %s 匹配策略: %s", from, to, strings.Join(matched, ", ")), nil
}

// ── Write tools ───────────────────────────────────────────────────────────────

// submitOrApply routes a write operation through WorkflowService unless auto-approved.
func (s *aiService) submitOrApply(
	ctx context.Context,
	namespace, toolName, resourceType, resourceName string,
	payload map[string]interface{},
	applyFn func(ctx context.Context, ns, payload string) (string, error),
) (string, error) {
	payload["namespace"] = namespace
	payloadBytes, _ := json.Marshal(payload)

	if s.autoApprove[toolName] {
		return applyFn(ctx, namespace, string(payloadBytes))
	}

	if s.workflow == nil {
		return "", fmt.Errorf("write tools require workflow service to be configured")
	}
	wr, err := s.workflow.Submit(ctx, SubmitWorkflowReq{
		WorkspaceID:  namespace,
		RequestedBy:  "ai-agent",
		ResourceType: resourceType,
		ResourceName: resourceName,
		Action:       toolName,
		Payload:      string(payloadBytes),
	})
	if err != nil {
		return "", fmt.Errorf("submit workflow: %w", err)
	}
	return fmt.Sprintf("已创建审批单 #%s，等待管理员审批后操作将自动执行。", wr.ID), nil
}

func (s *aiService) toolCreatePolicy(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	var args struct {
		Name         string                `json:"name"`
		Network      string                `json:"network"`
		Action       string                `json:"action"`
		PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Name == "" || args.Network == "" || args.Action == "" {
		return "", fmt.Errorf("name, network, action are required")
	}
	payload := map[string]interface{}{
		"name": args.Name, "network": args.Network,
		"action": args.Action, "peerSelector": args.PeerSelector,
	}
	return s.submitOrApply(ctx, namespace, "create_policy", "policy", args.Name, payload, s.applyCreatePolicy)
}

func (s *aiService) applyCreatePolicy(ctx context.Context, namespace, payload string) (string, error) {
	var args struct {
		Name         string                `json:"name"`
		Network      string                `json:"network"`
		Action       string                `json:"action"`
		PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
		Namespace    string                `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		return "", err
	}
	if args.Namespace != "" {
		namespace = args.Namespace
	}
	policy := &v1alpha1.LatticePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: args.Name, Namespace: namespace},
		Spec: v1alpha1.LatticePolicySpec{
			Network: args.Network,
			Action:  args.Action,
		},
	}
	if args.PeerSelector != nil {
		policy.Spec.PeerSelector = *args.PeerSelector
	}
	if err := s.k8s.GetClient().Create(ctx, policy); err != nil {
		return "", fmt.Errorf("create policy: %w", err)
	}
	return fmt.Sprintf("策略 %s 已创建 [%s] 网络: %s", args.Name, args.Action, args.Network), nil
}

func (s *aiService) toolDeletePeer(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	payload := map[string]interface{}{"name": args.Name}
	return s.submitOrApply(ctx, namespace, "delete_peer", "peer", args.Name, payload, s.applyDeletePeer)
}

func (s *aiService) applyDeletePeer(ctx context.Context, namespace, payload string) (string, error) {
	var args struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		return "", err
	}
	if args.Namespace != "" {
		namespace = args.Namespace
	}
	peer := &v1alpha1.LatticePeer{}
	peer.Name = args.Name
	peer.Namespace = namespace
	if err := s.k8s.GetClient().Delete(ctx, peer); err != nil {
		return "", fmt.Errorf("delete peer: %w", err)
	}
	return fmt.Sprintf("Peer %s 已删除", args.Name), nil
}

func (s *aiService) toolDeletePolicy(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	payload := map[string]interface{}{"name": args.Name}
	return s.submitOrApply(ctx, namespace, "delete_policy", "policy", args.Name, payload, s.applyDeletePolicy)
}

func (s *aiService) applyDeletePolicy(ctx context.Context, namespace, payload string) (string, error) {
	var args struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		return "", err
	}
	if args.Namespace != "" {
		namespace = args.Namespace
	}
	pol := &v1alpha1.LatticePolicy{}
	pol.Name = args.Name
	pol.Namespace = namespace
	if err := s.k8s.GetClient().Delete(ctx, pol); err != nil {
		return "", fmt.Errorf("delete policy: %w", err)
	}
	return fmt.Sprintf("策略 %s 已删除", args.Name), nil
}

func (s *aiService) toolCreatePeer(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	var args struct {
		Name     string            `json:"name"`
		Network  string            `json:"network"`
		AppId    string            `json:"appId"`
		Platform string            `json:"platform"`
		Labels   map[string]string `json:"labels,omitempty"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Name == "" || args.AppId == "" {
		return "", fmt.Errorf("name and appId are required")
	}
	payload := map[string]interface{}{
		"name": args.Name, "network": args.Network,
		"appId": args.AppId, "platform": args.Platform, "labels": args.Labels,
	}
	return s.submitOrApply(ctx, namespace, "create_peer", "peer", args.Name, payload, s.applyCreatePeer)
}

func (s *aiService) applyCreatePeer(ctx context.Context, namespace, payload string) (string, error) {
	var args struct {
		Name      string            `json:"name"`
		Network   string            `json:"network"`
		AppId     string            `json:"appId"`
		Platform  string            `json:"platform"`
		Labels    map[string]string `json:"labels,omitempty"`
		Namespace string            `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(payload), &args); err != nil {
		return "", err
	}
	if args.Namespace != "" {
		namespace = args.Namespace
	}
	peer := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: namespace,
			Labels:    args.Labels,
		},
		Spec: v1alpha1.LatticePeerSpec{
			AppId:    args.AppId,
			Platform: args.Platform,
		},
	}
	if err := s.k8s.GetClient().Create(ctx, peer); err != nil {
		return "", fmt.Errorf("create peer: %w", err)
	}
	return fmt.Sprintf("Peer %s 已创建 (appId: %s)", args.Name, args.AppId), nil
}

// ── Intent Engine tools (Pro) ──────────────────────────────────────────────────

func (s *aiService) toolPlanNetworkChange(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	if s.intentSvc == nil {
		return "", fmt.Errorf("network intent engine is a Pro feature — upgrade at https://alattice.io/pro")
	}
	var args struct {
		Intent string `json:"intent"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	ws, err := s.store.Workspaces().GetByNamespace(ctx, namespace)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}

	plan, err := s.intentSvc.Plan(ctx, IntentRequest{
		WorkspaceID: ws.ID,
		Namespace:   namespace,
		Intent:      args.Intent,
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 变更计划 (ID: %s)\n\n", plan.ID))
	sb.WriteString(fmt.Sprintf("**风险等级**: %s\n\n", plan.RiskLevel))
	sb.WriteString(plan.Summary)
	sb.WriteString("\n\n### 变更明细\n")
	for _, c := range plan.Changes {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", c.Action, c.Resource))
	}
	sb.WriteString(fmt.Sprintf("\n> 有效期至: %s\n", plan.ExpiresAt.Format(time.RFC3339)))
	sb.WriteString("> 确认执行请调用 apply_network_change，传入 plan_id")
	return sb.String(), nil
}

func (s *aiService) toolApplyNetworkChange(ctx context.Context, input json.RawMessage) (string, error) {
	if s.intentSvc == nil {
		return "", fmt.Errorf("network intent engine is a Pro feature — upgrade at https://alattice.io/pro")
	}
	var args struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	workflowIDs, err := s.intentSvc.Apply(ctx, args.PlanID, "mcp-user")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("变更计划 %s 已提交审批，工作流 ID: %v。审批通过后自动执行。", args.PlanID, workflowIDs), nil
}

// ── Time-Travel Debug tools (Pro) ──────────────────────────────────────────────

func (s *aiService) Debug(ctx context.Context, req *DebugRequest, out StreamWriter) error {
	ws, err := s.store.Workspaces().GetByID(ctx, req.WorkspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	debugTools := s.buildDebugTools()
	system := fmt.Sprintf(`你是 Lattice 网络调试助手。通过查询历史快照来定位网络故障根因。
当前工作区: %s（命名空间: %s）
调试时间范围: %s ~ %s

分析步骤：先用 list_snapshots 找到相关时间点的快照，再用 diff_snapshots 对比变更，最后用 check_connectivity_at 验证结论。`,
		ws.DisplayName, ws.Namespace,
		req.From.Format("2006-01-02 15:04"), req.To.Format("2006-01-02 15:04"))

	msgs := []llm.Message{{Role: llm.RoleUser, Content: req.Question}}

	for i := 0; i < s.maxToolCalls; i++ {
		resp, err := s.llm.Complete(ctx, &llm.Request{
			System:    system,
			Messages:  msgs,
			Tools:     debugTools,
			MaxTokens: 4096,
		})
		if err != nil {
			_ = out.Write(StreamEvent{Type: "error", Error: err.Error()})
			return err
		}
		if !resp.HasToolCalls() {
			_ = out.Write(StreamEvent{Type: "token", Content: resp.Content})
			_ = out.Write(StreamEvent{Type: "done"})
			return nil
		}
		toolResultMsg := llm.Message{Role: llm.RoleTool}
		assistantMsg := llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls}
		for _, tc := range resp.ToolCalls {
			_ = out.Write(StreamEvent{Type: "tool_use", Tool: tc.Name, Input: tc.Input})
			result, toolErr := s.ExecuteTool(ctx, ws.Namespace, tc.Name, tc.Input)
			if toolErr != nil {
				result = "error: " + toolErr.Error()
			}
			toolResultMsg.ToolResults = append(toolResultMsg.ToolResults, llm.ToolResult{
				ToolCallID: tc.ID, Content: result,
			})
		}
		msgs = append(msgs, assistantMsg, toolResultMsg)
	}
	return nil
}

func (s *aiService) buildDebugTools() []llm.Tool {
	all := s.ListTools("")
	debug := make([]llm.Tool, 0, 4)
	debugNames := map[string]bool{
		"list_snapshots": true, "get_snapshot": true,
		"diff_snapshots": true, "check_connectivity_at": true,
	}
	for _, t := range all {
		if debugNames[t.Name] {
			debug = append(debug, t)
		}
	}
	return debug
}

func (s *aiService) toolListSnapshots(ctx context.Context, namespace string, input json.RawMessage) (string, error) {
	if s.snapStore == nil {
		return "", fmt.Errorf("snapshot store not configured (Time-Travel requires Pro)")
	}
	var args struct {
		From        string `json:"from"`
		To          string `json:"to"`
		TriggerType string `json:"triggerType"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	from, err := time.Parse(time.RFC3339, args.From)
	if err != nil {
		return "", fmt.Errorf("invalid from time: %w", err)
	}
	to, err := time.Parse(time.RFC3339, args.To)
	if err != nil {
		return "", fmt.Errorf("invalid to time: %w", err)
	}

	ws, err := s.store.Workspaces().GetByNamespace(ctx, namespace)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	snaps, err := s.snapStore.List(ctx, ws.ID, from, to, args.TriggerType)
	if err != nil {
		return "", err
	}
	if len(snaps) == 0 {
		return "该时间范围内没有快照记录。", nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个快照：\n", len(snaps)))
	for _, sn := range snaps {
		sb.WriteString(fmt.Sprintf("- [%s] %s  触发: %s  操作者: %s\n",
			sn.ID, sn.CapturedAt.Format("2006-01-02 15:04:05"), sn.TriggerType, sn.TriggerBy))
	}
	return sb.String(), nil
}

func (s *aiService) toolGetSnapshot(ctx context.Context, input json.RawMessage) (string, error) {
	if s.snapStore == nil {
		return "", fmt.Errorf("snapshot store not configured")
	}
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	snap, err := s.snapStore.GetByID(ctx, args.ID)
	if err != nil {
		return "", fmt.Errorf("snapshot not found: %w", err)
	}
	result := fmt.Sprintf("快照 %s（%s, 触发: %s）\n\nPeers:\n%s\n\nPolicies:\n%s\n\nNetworks:\n%s",
		snap.ID, snap.CapturedAt.Format("2006-01-02 15:04:05"), snap.TriggerType,
		snap.Peers, snap.Policies, snap.Networks)
	return result, nil
}

func (s *aiService) toolDiffSnapshots(ctx context.Context, input json.RawMessage) (string, error) {
	if s.snapStore == nil {
		return "", fmt.Errorf("snapshot store not configured")
	}
	var args struct {
		FromID string `json:"from_id"`
		ToID   string `json:"to_id"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	from, err := s.snapStore.GetByID(ctx, args.FromID)
	if err != nil {
		return "", fmt.Errorf("from snapshot not found: %w", err)
	}
	to, err := s.snapStore.GetByID(ctx, args.ToID)
	if err != nil {
		return "", fmt.Errorf("to snapshot not found: %w", err)
	}

	diff := diffPolicies(from.Policies, to.Policies)
	if diff == "" {
		return fmt.Sprintf("快照 %s → %s 之间策略无变化。", args.FromID, args.ToID), nil
	}
	return fmt.Sprintf("快照 %s (%s) → %s (%s) 策略变更:\n%s",
		args.FromID, from.CapturedAt.Format("15:04:05"),
		args.ToID, to.CapturedAt.Format("15:04:05"),
		diff), nil
}

// diffPolicies returns a human-readable diff between two JSON policy arrays.
func diffPolicies(fromJSON, toJSON string) string {
	var from, to []struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(fromJSON), &from)
	_ = json.Unmarshal([]byte(toJSON), &to)

	fromMap := make(map[string]string)
	for _, p := range from {
		fromMap[p.Name] = p.Action
	}
	toMap := make(map[string]string)
	for _, p := range to {
		toMap[p.Name] = p.Action
	}

	var sb strings.Builder
	for name := range fromMap {
		if _, exists := toMap[name]; !exists {
			sb.WriteString(fmt.Sprintf("- 删除: policy/%s\n", name))
		}
	}
	for name := range toMap {
		if _, exists := fromMap[name]; !exists {
			sb.WriteString(fmt.Sprintf("+ 新增: policy/%s [%s]\n", name, toMap[name]))
		}
	}
	return sb.String()
}

func (s *aiService) toolCheckConnectivityAt(ctx context.Context, input json.RawMessage) (string, error) {
	if s.snapStore == nil {
		return "", fmt.Errorf("snapshot store not configured")
	}
	var args struct {
		SnapshotID string `json:"snapshot_id"`
		From       string `json:"from"`
		To         string `json:"to"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	snap, err := s.snapStore.GetByID(ctx, args.SnapshotID)
	if err != nil {
		return "", fmt.Errorf("snapshot not found: %w", err)
	}

	var policies []struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(snap.Policies), &policies); err != nil {
		return "", fmt.Errorf("parse snapshot policies: %w", err)
	}

	for _, p := range policies {
		if p.Action == "ALLOW" {
			return fmt.Sprintf("快照 %s 中存在 ALLOW 策略 %q，%s → %s 可能连通（需结合 Peer 标签确认）",
				args.SnapshotID, p.Name, args.From, args.To), nil
		}
	}
	return fmt.Sprintf("快照 %s 中未找到允许 %s → %s 的 ALLOW 策略，连接被阻断。",
		args.SnapshotID, args.From, args.To), nil
}
