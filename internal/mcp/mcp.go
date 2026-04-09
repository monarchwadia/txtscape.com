package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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

type surfArgs struct {
	URL string `json:"url"`
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Serve runs the MCP server on stdin/stdout using JSON-RPC.
func Serve() {
	scanner := bufio.NewScanner(os.Stdin)
	// Allow up to 1MB per line (for large requests)
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
		resp := handleRequest(req)
		writeResponse(os.Stdout, resp)
	}
}

func handleRequest(req jsonrpcRequest) jsonrpcResponse {
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
		// No response needed for notifications
		return jsonrpcResponse{}

	case "tools/list":
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": []map[string]any{
					{
						"name":        "surf",
						"description": "Fetch a .txt URL and return its contents. Use this to browse the txtscape network — follow markdown links from page to page.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"url": map[string]any{
									"type":        "string",
									"description": "The HTTPS URL of a .txt file to fetch",
								},
							},
							"required": []string{"url"},
						},
					},
				},
			},
		}

	case "tools/call":
		return handleToolCall(req)

	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32601, Message: "method not found"},
		}
	}
}

func handleToolCall(req jsonrpcRequest) jsonrpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "invalid params"},
		}
	}

	if params.Name != "surf" {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}

	var args surfArgs
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "invalid arguments"},
		}
	}

	if args.URL == "" {
		return toolError(req.ID, "url is required")
	}

	if !strings.HasPrefix(args.URL, "https://") {
		return toolError(req.ID, "only https:// URLs are supported")
	}

	content, err := fetchURL(args.URL)
	if err != nil {
		return toolError(req.ID, err.Error())
	}

	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": content,
				},
			},
		},
	}
}

func fetchURL(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "txtscape/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	// Limit to 1MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", url, err)
	}

	return string(body), nil
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
	// Skip empty responses (notifications)
	if resp.JSONRPC == "" {
		return
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}
