package mcp

import (
	"encoding/json"
	"testing"
)

func TestHandleRequest_Initialize_ServerInfo_ReturnsCapabilities(t *testing.T) {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}

	resp := handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if info["name"] != "txtscape" {
		t.Fatalf("name = %v, want txtscape", info["name"])
	}
}

func TestHandleRequest_ToolsList_DiscoverTools_ReturnsSurf(t *testing.T) {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}

	resp := handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatal("tools is not a slice")
	}
	if len(tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(tools))
	}
	if tools[0]["name"] != "surf" {
		t.Fatalf("tool name = %v, want surf", tools[0]["name"])
	}
}

func TestHandleRequest_UnknownMethod_InvalidCall_ReturnsError(t *testing.T) {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "unknown/method",
	}

	resp := handleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("code = %d, want -32601", resp.Error.Code)
	}
}

func TestHandleToolCall_NoURL_ValidationFails_ReturnsToolError(t *testing.T) {
	params, _ := json.Marshal(toolCallParams{
		Name:      "surf",
		Arguments: json.RawMessage(`{"url": ""}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected jsonrpc error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] != true {
		t.Fatal("expected isError=true")
	}
}

func TestHandleToolCall_HTTPUrl_SecurityReject_ReturnsToolError(t *testing.T) {
	params, _ := json.Marshal(toolCallParams{
		Name:      "surf",
		Arguments: json.RawMessage(`{"url": "http://example.com/page.txt"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := handleRequest(req)
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] != true {
		t.Fatal("expected isError=true for http:// URL")
	}
}

func TestHandleToolCall_UnknownTool_InvalidTool_ReturnsError(t *testing.T) {
	params, _ := json.Marshal(toolCallParams{
		Name:      "notsurf",
		Arguments: json.RawMessage(`{}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := handleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}
