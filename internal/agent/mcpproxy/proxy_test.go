package mcpproxy

import (
	"strings"
	"testing"
)

func TestExtractMCPTool_ToolsCall(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read_file","arguments":{"path":"/data/report.pdf"}}}`)
	tool, params := extractMCPTool(body)
	if tool != "read_file" {
		t.Fatalf("expected read_file, got %q", tool)
	}
	if params["path"] != "/data/report.pdf" {
		t.Fatalf("expected path param, got %v", params)
	}
}

func TestExtractMCPTool_NonToolCall(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list","params":{}}`)
	tool, _ := extractMCPTool(body)
	if tool != "" {
		t.Fatalf("expected empty tool for non-tools/call, got %q", tool)
	}
}

func TestExtractMCPTool_Invalid(t *testing.T) {
	tool, _ := extractMCPTool([]byte(`not json`))
	if tool != "" {
		t.Fatalf("expected empty tool for invalid JSON, got %q", tool)
	}
}

func TestSummarizeParams_Redaction(t *testing.T) {
	params := map[string]interface{}{
		"path":     "/data/file.txt",
		"password": "supersecret",
		"token":    "abc123",
	}
	summary := summarizeParams(params)
	if strings.Contains(summary, "supersecret") || strings.Contains(summary, "abc123") {
		t.Errorf("sensitive values should be redacted, got: %s", summary)
	}
	if !strings.Contains(summary, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in summary, got: %s", summary)
	}
}

func TestSummarizeParams_Truncation(t *testing.T) {
	longStr := make([]byte, 300)
	for i := range longStr {
		longStr[i] = 'a'
	}
	params := map[string]interface{}{"content": string(longStr)}
	summary := summarizeParams(params)
	if !strings.Contains(summary, "[truncated") {
		t.Errorf("expected truncation note, got: %s", summary)
	}
}
