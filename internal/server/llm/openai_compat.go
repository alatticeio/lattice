package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
const defaultDeepSeekModel = "deepseek-chat"
const defaultOpenAIModel = "gpt-4o"

// OpenAICompatClient implements Client for any OpenAI-compatible API
// (DeepSeek, OpenAI, self-hosted models via ollama/vllm, etc.).
type OpenAICompatClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewOpenAICompatClient(baseURL, apiKey, model string) *OpenAICompatClient {
	return &OpenAICompatClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// ── OpenAI wire types ─────────────────────────────────────────────────────────

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"` // string or nil
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
}

type oaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string          `json:"type"`
	Function oaiToolFunction `json:"function"`
}

type oaiToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type oaiChoice struct {
	Message      oaiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (c *OpenAICompatClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	or := oaiRequest{Model: c.model}

	// system prompt as first system message
	if req.System != "" {
		or.Messages = append(or.Messages, oaiMessage{Role: "system", Content: req.System})
	}

	// Convert tools
	for _, t := range req.Tools {
		or.Tools = append(or.Tools, oaiTool{
			Type: "function",
			Function: oaiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	// Convert messages
	for _, m := range req.Messages {
		or.Messages = append(or.Messages, toOAIMessages(m)...)
	}

	body, err := json.Marshal(or)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var or2 oaiResponse
	if err := json.Unmarshal(data, &or2); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if or2.Error != nil {
		return nil, fmt.Errorf("llm error %s: %s", or2.Error.Type, or2.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, string(data))
	}
	if len(or2.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices")
	}

	return fromOAIResponse(or2.Choices[0]), nil
}

// toOAIMessages converts a neutral Message to one or more OpenAI messages.
// Tool results require one message per result in OpenAI format.
func toOAIMessages(m Message) []oaiMessage {
	if m.Role == RoleTool {
		var msgs []oaiMessage
		for _, tr := range m.ToolResults {
			msgs = append(msgs, oaiMessage{
				Role:       "tool",
				Content:    tr.Content,
				ToolCallID: tr.ToolCallID,
			})
		}
		return msgs
	}

	msg := oaiMessage{Role: m.Role}
	if len(m.ToolCalls) > 0 {
		msg.Content = nil
		for _, tc := range m.ToolCalls {
			argStr, _ := json.Marshal(tc.Input)
			msg.ToolCalls = append(msg.ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: oaiFunction{
					Name:      tc.Name,
					Arguments: string(argStr),
				},
			})
		}
	} else {
		msg.Content = m.Content
	}
	return []oaiMessage{msg}
}

func fromOAIResponse(choice oaiChoice) *Response {
	r := &Response{StopReason: StopReasonEndTurn}
	if choice.FinishReason == "tool_calls" {
		r.StopReason = StopReasonToolUse
	}

	msg := choice.Message
	if s, ok := msg.Content.(string); ok {
		r.Content = s
	}
	for _, tc := range msg.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return r
}
