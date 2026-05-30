package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DiscoverTools connects to an MCP server endpoint and lists available tools.
func DiscoverTools(endpoint string, timeout time.Duration) ([]ToolDef, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp struct {
		Result ToolsListResult `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result.Tools, nil
}
