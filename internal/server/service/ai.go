package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
}

// ── Implementation ────────────────────────────────────────────────────────────

type aiService struct {
	logger       *log.Logger
	llm          llm.Client
	store        store.Store
	k8s          *resource.Client
	presence     *managementnats.NodePresenceStore
	maxToolCalls int
}

func NewAIService(
	llmClient llm.Client,
	st store.Store,
	k8s *resource.Client,
	presence *managementnats.NodePresenceStore,
	maxToolCalls int,
) AIService {
	if maxToolCalls <= 0 {
		maxToolCalls = 5
	}
	return &aiService{
		logger:       log.GetLogger("ai-service"),
		llm:          llmClient,
		store:        st,
		k8s:          k8s,
		presence:     presence,
		maxToolCalls: maxToolCalls,
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

	tools := s.buildTools(ws.Namespace)

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

			result, toolErr := s.executeTool(ctx, ws.Namespace, tc.Name, tc.Input)
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

func (s *aiService) buildTools(namespace string) []llm.Tool {
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
	}
}

func (s *aiService) executeTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
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
