// Package llm provides a provider-agnostic LLM client interface.
// Supported providers: Anthropic (native API) and any OpenAI-compatible service
// (DeepSeek, OpenAI, self-hosted models).
package llm

import (
	"context"
	"encoding/json"
)

// Role constants for messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// StopReason values returned by the LLM.
const (
	StopReasonEndTurn  = "end_turn"
	StopReasonToolUse  = "tool_use"
	StopReasonToolCall = "tool_calls" // OpenAI compat alias
)

// Message is a provider-neutral conversation message.
type Message struct {
	Role        string       `json:"role"`
	Content     string       `json:"content,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult carries the output of a completed tool call back to the LLM.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// Tool describes a callable function exposed to the LLM.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema object
}

// Request is a provider-neutral LLM completion request.
type Request struct {
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`
}

// Response is a provider-neutral LLM completion response.
type Response struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
}

// HasToolCalls reports whether the response contains tool calls to execute.
func (r *Response) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// Client is the common interface implemented by all LLM providers.
type Client interface {
	// Complete sends a request and waits for the full response.
	// Tool-use iterations are managed by the caller (AIService agentic loop).
	Complete(ctx context.Context, req *Request) (*Response, error)
}
