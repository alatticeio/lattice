package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultAnthropicBaseURL = "https://api.anthropic.com"
const defaultAnthropicModel = "claude-sonnet-4-6"
const anthropicVersion = "2023-06-01"

// AnthropicClient implements Client using the Anthropic Messages API.
type AnthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewAnthropicClient(apiKey, model, baseURL string) *AnthropicClient {
	if model == "" {
		model = defaultAnthropicModel
	}
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return &AnthropicClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// ── Anthropic wire types ──────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content anthropicContent `json:"content"`
}

// anthropicContent can be a plain string or a list of content blocks.
type anthropicContent struct {
	text   string
	blocks []anthropicBlock
}

func (c anthropicContent) MarshalJSON() ([]byte, error) {
	if len(c.blocks) > 0 {
		return json.Marshal(c.blocks)
	}
	return json.Marshal(c.text)
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (c *AnthropicClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	ar := anthropicRequest{
		Model:     c.model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
	}
	if ar.MaxTokens == 0 {
		ar.MaxTokens = 4096
	}

	// Convert tools
	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	// Convert messages
	for _, m := range req.Messages {
		am, err := toAnthropicMessage(m)
		if err != nil {
			return nil, fmt.Errorf("convert message: %w", err)
		}
		ar.Messages = append(ar.Messages, am)
	}

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ar2 anthropicResponse
	if err := json.Unmarshal(data, &ar2); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if ar2.Error != nil {
		return nil, fmt.Errorf("anthropic error %s: %s", ar2.Error.Type, ar2.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(data))
	}

	return fromAnthropicResponse(ar2), nil
}

func toAnthropicMessage(m Message) (anthropicMessage, error) {
	switch {
	case m.Role == RoleTool:
		// Tool results: one block per result
		var blocks []anthropicBlock
		for _, tr := range m.ToolResults {
			blocks = append(blocks, anthropicBlock{
				Type:      "tool_result",
				ToolUseID: tr.ToolCallID,
				Content:   tr.Content,
			})
		}
		return anthropicMessage{
			Role:    "user",
			Content: anthropicContent{blocks: blocks},
		}, nil

	case len(m.ToolCalls) > 0:
		// Assistant message with tool calls
		var blocks []anthropicBlock
		if m.Content != "" {
			blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
		}
		for _, tc := range m.ToolCalls {
			blocks = append(blocks, anthropicBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			})
		}
		return anthropicMessage{
			Role:    "assistant",
			Content: anthropicContent{blocks: blocks},
		}, nil

	default:
		return anthropicMessage{
			Role:    m.Role,
			Content: anthropicContent{text: m.Content},
		}, nil
	}
}

func fromAnthropicResponse(ar anthropicResponse) *Response {
	r := &Response{StopReason: StopReasonEndTurn}
	if ar.StopReason == "tool_use" {
		r.StopReason = StopReasonToolUse
	}
	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			r.Content += block.Text
		case "tool_use":
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	return r
}
