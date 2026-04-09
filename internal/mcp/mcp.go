package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Server is an MCP server that dispatches tool calls to an internal HTTP handler.
type Server struct {
	handler http.Handler
}

// NewServer creates an MCP server backed by the given HTTP handler (typically the app's mux).
func NewServer(handler http.Handler) *Server {
	return &Server{handler: handler}
}

// Serve runs the MCP server on stdin/stdout using JSON-RPC.
func (s *Server) Serve() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(os.Stdout, jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}
		resp := s.handleRequest(req)
		writeResponse(os.Stdout, resp)
	}
}

func (s *Server) handleRequest(req jsonrpcRequest) jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "txtscape",
					"version": "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		return jsonrpcResponse{}

	case "tools/list":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": toolDefinitions(),
			},
		}

	case "tools/call":
		return s.handleToolCall(req)

	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32601, Message: "method not found"},
		}
	}
}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "get_page",
			"description": "Fetch a page or directory listing from the txtscape network. Pages are plain text linked together with markdown-style links. Follow links to browse from page to page. Start at a user's root (/~username) to see their directory listing.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path like /~alice/blog/post.txt or /~alice for directory listing",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "put_page",
			"description": "Create or update a plain text page. Path must end in .txt. Maximum 100KB. Folders are created implicitly. Requires a token from signup or login.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path like /~alice/blog/post.txt",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Plain text content (may contain markdown links to other pages)",
					},
					"token": map[string]any{
						"type":        "string",
						"description": "Bearer token from signup or login",
					},
				},
				"required": []string{"path", "content", "token"},
			},
		},
		{
			"name":        "delete_page",
			"description": "Delete a page. Requires a token from signup or login.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path like /~alice/blog/post.txt",
					},
					"token": map[string]any{
						"type":        "string",
						"description": "Bearer token from signup or login",
					},
				},
				"required": []string{"path", "token"},
			},
		},
		{
			"name":        "signup",
			"description": "Create a new account. Returns a bearer token for writing pages.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"username": map[string]any{
						"type":        "string",
						"description": "1-30 lowercase alphanumeric, hyphens, or underscores",
					},
					"password": map[string]any{
						"type":        "string",
						"description": "8-72 characters",
					},
				},
				"required": []string{"username", "password"},
			},
		},
		{
			"name":        "login",
			"description": "Get a new bearer token for an existing account.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"username": map[string]any{
						"type":        "string",
						"description": "Your username",
					},
					"password": map[string]any{
						"type":        "string",
						"description": "Your password",
					},
				},
				"required": []string{"username", "password"},
			},
		},
	}
}

func (s *Server) handleToolCall(req jsonrpcRequest) jsonrpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "invalid params"},
		}
	}

	switch params.Name {
	case "get_page":
		return s.handleGetPage(req.ID, params.Arguments)
	case "put_page":
		return s.handlePutPage(req.ID, params.Arguments)
	case "delete_page":
		return s.handleDeletePage(req.ID, params.Arguments)
	case "signup":
		return s.handleSignup(req.ID, params.Arguments)
	case "login":
		return s.handleLogin(req.ID, params.Arguments)
	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

// callInternal dispatches a request to the internal HTTP handler.
func (s *Server) callInternal(method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func (s *Server) handleGetPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}

	rec := s.callInternal("GET", a.Path, nil, nil)
	body := rec.Body.String()

	if rec.Code < 200 || rec.Code >= 300 {
		return toolError(id, fmt.Sprintf("HTTP %d: %s", rec.Code, strings.TrimSpace(body)))
	}

	return toolSuccess(id, body)
}

func (s *Server) handlePutPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	if a.Content == "" {
		return toolError(id, "content is required")
	}
	if a.Token == "" {
		return toolError(id, "token is required")
	}

	rec := s.callInternal("PUT", a.Path, strings.NewReader(a.Content), map[string]string{
		"Authorization": "Bearer " + a.Token,
	})
	body := rec.Body.String()

	if rec.Code < 200 || rec.Code >= 300 {
		return toolError(id, fmt.Sprintf("HTTP %d: %s", rec.Code, strings.TrimSpace(body)))
	}

	return toolSuccess(id, "page saved")
}

func (s *Server) handleDeletePage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path  string `json:"path"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	if a.Token == "" {
		return toolError(id, "token is required")
	}

	rec := s.callInternal("DELETE", a.Path, nil, map[string]string{
		"Authorization": "Bearer " + a.Token,
	})
	body := rec.Body.String()

	if rec.Code < 200 || rec.Code >= 300 {
		return toolError(id, fmt.Sprintf("HTTP %d: %s", rec.Code, strings.TrimSpace(body)))
	}

	return toolSuccess(id, "page deleted")
}

func (s *Server) handleSignup(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Username == "" {
		return toolError(id, "username is required")
	}
	if a.Password == "" {
		return toolError(id, "password is required")
	}

	form := url.Values{"username": {a.Username}, "password": {a.Password}}
	rec := s.callInternal("POST", "/signup", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	body := rec.Body.String()

	if rec.Code < 200 || rec.Code >= 300 {
		return toolError(id, fmt.Sprintf("HTTP %d: %s", rec.Code, strings.TrimSpace(body)))
	}

	return toolSuccess(id, strings.TrimSpace(body))
}

func (s *Server) handleLogin(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Username == "" {
		return toolError(id, "username is required")
	}
	if a.Password == "" {
		return toolError(id, "password is required")
	}

	form := url.Values{"username": {a.Username}, "password": {a.Password}}
	rec := s.callInternal("POST", "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	body := rec.Body.String()

	if rec.Code < 200 || rec.Code >= 300 {
		return toolError(id, fmt.Sprintf("HTTP %d: %s", rec.Code, strings.TrimSpace(body)))
	}

	return toolSuccess(id, strings.TrimSpace(body))
}

func toolSuccess(id json.RawMessage, text string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	}
}

func toolError(id json.RawMessage, msg string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": msg,
				},
			},
			"isError": true,
		},
	}
}

func writeResponse(w io.Writer, resp jsonrpcResponse) {
	if resp.JSONRPC == "" {
		return
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}

// HTTPHandler returns an http.Handler that implements the MCP Streamable HTTP transport.
// POST /mcp receives JSON-RPC requests, returns application/json responses.
// GET /mcp returns 405 (no server-initiated SSE streams).
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s.handleHTTPPost(w, r)
		case http.MethodGet:
			http.Error(w, "SSE stream not supported", http.StatusMethodNotAllowed)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (s *Server) handleHTTPPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}

	var req jsonrpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(jsonrpcResponse{
			JSONRPC: "2.0",
			Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	resp := s.handleRequest(req)

	// Notifications return empty JSONRPC — respond with 202 Accepted
	if resp.JSONRPC == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
