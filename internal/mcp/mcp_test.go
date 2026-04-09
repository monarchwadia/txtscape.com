package mcp

import (
	"encoding/json"
	"net/http"
	"testing"
)

// fakeHandler is a minimal HTTP handler that records what it receives
// and returns canned responses for testing MCP tool dispatch.
func fakeHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"token":"fake-token-123"}`))
	})

	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"login-token-456"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello from fake page"))
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}

func TestHandleRequest_Initialize_ServerInfo_ReturnsCapabilities(t *testing.T) {
	// Business context: MCP agents need server metadata on connect.
	// Scenario: Send an initialize request.
	// Expected: Returns server name, version, and tool capabilities.
	s := NewServer(fakeHandler())
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}

	resp := s.handleRequest(req)
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

func TestHandleRequest_ToolsList_DiscoverTools_Returns5Tools(t *testing.T) {
	// Business context: Agents discover available operations via tools/list.
	// Scenario: Request tool listing.
	// Expected: Returns 5 tools (get_page, put_page, delete_page, signup, login).
	s := NewServer(fakeHandler())
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}

	resp := s.handleRequest(req)
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
	if len(tools) != 5 {
		t.Fatalf("len(tools) = %d, want 5", len(tools))
	}

	expected := map[string]bool{
		"get_page": false, "put_page": false, "delete_page": false,
		"signup": false, "login": false,
	}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if _, ok := expected[name]; !ok {
			t.Errorf("unexpected tool: %s", name)
		}
		expected[name] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestHandleRequest_UnknownMethod_InvalidCall_ReturnsError(t *testing.T) {
	// Business context: MCP protocol requires error for unknown methods.
	// Scenario: Send an unrecognized method.
	// Expected: Returns -32601 method not found.
	s := NewServer(fakeHandler())
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "unknown/method",
	}

	resp := s.handleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("code = %d, want -32601", resp.Error.Code)
	}
}

func TestHandleToolCall_UnknownTool_InvalidTool_ReturnsError(t *testing.T) {
	// Business context: Agents should get clear errors for invalid tool names.
	// Scenario: Call a tool that doesn't exist.
	// Expected: Returns JSON-RPC error.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "nonexistent",
		Arguments: json.RawMessage(`{}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestHandleToolCall_GetPage_BrowseNetwork_ReturnsContent(t *testing.T) {
	// Business context: Agents browse the txtscape network by fetching pages.
	// Scenario: Call get_page with a valid path.
	// Expected: Returns the page content from the internal handler.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "get_page",
		Arguments: json.RawMessage(`{"path":"/~alice/hello.txt"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	content := result["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if text != "hello from fake page" {
		t.Fatalf("text = %q, want %q", text, "hello from fake page")
	}
}

func TestHandleToolCall_PutPage_PublishContent_Returns204(t *testing.T) {
	// Business context: Agents publish pages via put_page with auth.
	// Scenario: Call put_page with content and token.
	// Expected: Returns success (handler returns 204).
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "put_page",
		Arguments: json.RawMessage(`{"path":"/~alice/hello.txt","content":"my page","token":"tok123"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] == true {
		content := result["content"].([]map[string]any)
		t.Fatalf("expected success, got error: %s", content[0]["text"])
	}
}

func TestHandleToolCall_DeletePage_RemoveContent_Returns204(t *testing.T) {
	// Business context: Agents delete their own pages.
	// Scenario: Call delete_page with path and token.
	// Expected: Returns success.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "delete_page",
		Arguments: json.RawMessage(`{"path":"/~alice/hello.txt","token":"tok123"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`7`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] == true {
		content := result["content"].([]map[string]any)
		t.Fatalf("expected success, got error: %s", content[0]["text"])
	}
}

func TestHandleToolCall_Signup_CreateAccount_ReturnsToken(t *testing.T) {
	// Business context: Agents create accounts to publish pages.
	// Scenario: Call signup with username and password.
	// Expected: Returns the token from the handler's JSON response.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "signup",
		Arguments: json.RawMessage(`{"username":"alice","password":"secret123"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`8`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	content := result["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if text != `{"token":"fake-token-123"}` {
		t.Fatalf("text = %q, want token JSON", text)
	}
}

func TestHandleToolCall_Login_GetToken_ReturnsToken(t *testing.T) {
	// Business context: Agents log in to get a token for writing.
	// Scenario: Call login with existing credentials.
	// Expected: Returns the token from the handler's JSON response.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "login",
		Arguments: json.RawMessage(`{"username":"alice","password":"secret123"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`9`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	content := result["content"].([]map[string]any)
	text := content[0]["text"].(string)
	if text != `{"token":"login-token-456"}` {
		t.Fatalf("text = %q, want token JSON", text)
	}
}

func TestHandleToolCall_GetPage_EmptyPath_ValidationFails_ReturnsToolError(t *testing.T) {
	// Business context: Empty paths are invalid for page operations.
	// Scenario: Call get_page with empty path.
	// Expected: Returns a tool error.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "get_page",
		Arguments: json.RawMessage(`{"path":""}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected jsonrpc error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] != true {
		t.Fatal("expected isError=true for empty path")
	}
}

func TestHandleToolCall_PutPage_MissingToken_ValidationFails_ReturnsToolError(t *testing.T) {
	// Business context: Writes require authentication tokens.
	// Scenario: Call put_page without a token.
	// Expected: Returns a tool error about missing token.
	s := NewServer(fakeHandler())
	params, _ := json.Marshal(toolCallParams{
		Name:      "put_page",
		Arguments: json.RawMessage(`{"path":"/~alice/hello.txt","content":"test"}`),
	})
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  "tools/call",
		Params:  params,
	}

	resp := s.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected jsonrpc error: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] != true {
		t.Fatal("expected isError=true for missing token")
	}
}
