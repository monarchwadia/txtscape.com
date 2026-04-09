package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// --- JSON-RPC types ---

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

// --- Path validation ---

var (
	fileNameRe   = regexp.MustCompile(`^[a-z0-9_-]{1,245}\.txt$`)
	folderPartRe = regexp.MustCompile(`^[a-z0-9_-]{1,50}$`)
)

const (
	maxDepth = 10
	maxSize  = 1048576 // 1MB
	pagesDir = ".txtscape/pages"
)

func validatePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.Contains(raw, `\`) {
		return "", fmt.Errorf("path must not contain backslashes")
	}
	if strings.Contains(raw, "..") {
		return "", fmt.Errorf("path must not contain '..'")
	}

	parts := strings.Split(raw, "/")
	fileName := parts[len(parts)-1]
	folderParts := parts[:len(parts)-1]

	if !fileNameRe.MatchString(fileName) {
		return "", fmt.Errorf("invalid filename %q: must be lowercase alphanumeric/hyphens/underscores ending in .txt", fileName)
	}

	if len(folderParts) >= maxDepth {
		return "", fmt.Errorf("path exceeds maximum depth of %d levels", maxDepth)
	}

	for _, p := range folderParts {
		if !folderPartRe.MatchString(p) {
			return "", fmt.Errorf("invalid folder name %q: must be 1-10 lowercase alphanumeric/hyphens/underscores", p)
		}
	}

	return raw, nil
}

// --- Server ---

type server struct {
	root string // absolute path to the directory containing .txtscape/pages
}

func newServer(root string) *server {
	return &server{root: root}
}

func (s *server) pagesRoot() string {
	return filepath.Join(s.root, pagesDir)
}

func (s *server) serve() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeJSON(os.Stdout, jsonrpcResponse{
				JSONRPC: "2.0",
				Error:   &jsonrpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}
		resp := s.handleRequest(req)
		if resp.JSONRPC != "" {
			writeJSON(os.Stdout, resp)
		}
	}
}

func (s *server) handleRequest(req jsonrpcRequest) jsonrpcResponse {
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
					"version": "0.1.0",
				},
				"instructions": "txtscape is a committable project memory. " +
					"Use put_page to store decisions, patterns, and knowledge as .txt files. " +
					"Use search_pages to find relevant memories. " +
					"Use list_pages to browse the directory tree with previews. " +
					"Files are plain text with markdown formatting. " +
					"All pages are stored in .txtscape/pages/ and should be committed to git. " +
					"Path rules: files must end in .txt, folder names are lowercase alphanumeric/hyphens/underscores (max 50 chars each), max 10 folder levels deep. " +
					"File size limit: 1MB per page. Search returns up to 100 matches.",
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
			"description": "Read a .txt page from project memory.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path relative to .txtscape/pages/. Must end in .txt. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars. Max 10 levels deep. Example: decisions/use-flat-files.txt",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "put_page",
			"description": "Create or update a .txt page in project memory. Folders are created automatically. Max 1MB per page. Path must end in .txt. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars each, max 10 levels deep.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path relative to .txtscape/pages/. Must end in .txt. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars. Max 10 levels deep. Example: decisions/use-flat-files.txt",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Plain text content (markdown formatting supported). Maximum 1MB.",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			"name":        "delete_page",
			"description": "Delete a .txt page from project memory.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path relative to .txtscape/pages/. Must end in .txt. Example: decisions/use-flat-files.txt",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "list_pages",
			"description": "List files and folders in project memory. Returns the first line of each file as a preview. Pass empty path or '/' to list the root.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Folder path to list, or empty/'/' for root. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars. Example: decisions",
					},
				},
			},
		},
		{
			"name":        "search_pages",
			"description": "Search across all pages in project memory. Returns matching lines with surrounding context. Returns up to 100 matches, case-insensitive.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Text to search for (case-insensitive). Returns up to 100 matches with 1 line of surrounding context each.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (s *server) handleToolCall(req jsonrpcRequest) jsonrpcResponse {
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
	case "list_pages":
		return s.handleListPages(req.ID, params.Arguments)
	case "search_pages":
		return s.handleSearchPages(req.ID, params.Arguments)
	default:
		return jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonrpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

// --- Tool handlers ---

func (s *server) handleGetPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	clean, err := validatePath(a.Path)
	if err != nil {
		return toolError(id, err.Error())
	}

	fullPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(clean))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return toolError(id, "page not found: "+a.Path)
		}
		return toolError(id, "reading page: "+err.Error())
	}
	return toolSuccess(id, string(data))
}

func (s *server) handlePutPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
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
	clean, err := validatePath(a.Path)
	if err != nil {
		return toolError(id, err.Error())
	}
	if len(a.Content) > maxSize {
		return toolError(id, fmt.Sprintf("content exceeds maximum size of %d bytes", maxSize))
	}

	fullPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(clean))
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return toolError(id, "creating directory: "+err.Error())
	}
	if err := os.WriteFile(fullPath, []byte(a.Content), 0o644); err != nil {
		return toolError(id, "writing page: "+err.Error())
	}
	return toolSuccess(id, "page saved: "+a.Path)
}

func (s *server) handleDeletePage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	clean, err := validatePath(a.Path)
	if err != nil {
		return toolError(id, err.Error())
	}

	fullPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(clean))
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return toolError(id, "page not found: "+a.Path)
		}
		return toolError(id, "deleting page: "+err.Error())
	}
	return toolSuccess(id, "page deleted: "+a.Path)
}

func (s *server) handleListPages(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path string `json:"path"`
	}
	if args != nil {
		json.Unmarshal(args, &a)
	}

	dirPath := s.pagesRoot()
	if a.Path != "" && a.Path != "/" {
		// Validate folder parts only (no filename)
		parts := strings.Split(strings.Trim(a.Path, "/"), "/")
		for _, p := range parts {
			if !folderPartRe.MatchString(p) {
				return toolError(id, fmt.Sprintf("invalid folder name %q", p))
			}
		}
		if len(parts) >= maxDepth {
			return toolError(id, fmt.Sprintf("path exceeds maximum depth of %d levels", maxDepth))
		}
		dirPath = filepath.Join(dirPath, filepath.FromSlash(a.Path))
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return toolError(id, "directory not found: "+a.Path)
		}
		return toolError(id, "listing directory: "+err.Error())
	}

	var buf strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			buf.WriteString(fmt.Sprintf("📁 %s/\n", e.Name()))
		} else if strings.HasSuffix(e.Name(), ".txt") {
			preview := firstLine(filepath.Join(dirPath, e.Name()))
			buf.WriteString(fmt.Sprintf("📄 %s  —  %s\n", e.Name(), preview))
		}
	}

	if buf.Len() == 0 {
		return toolSuccess(id, "(empty directory)")
	}
	return toolSuccess(id, buf.String())
}

func (s *server) handleSearchPages(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Query == "" {
		return toolError(id, "query is required")
	}

	query := strings.ToLower(a.Query)
	var results strings.Builder
	matchCount := 0

	filepath.Walk(s.pagesRoot(), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(s.pagesRoot(), path)
		relPath = filepath.ToSlash(relPath)
		lines := strings.Split(string(data), "\n")

		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), query) {
				if matchCount > 0 {
					results.WriteString("\n")
				}
				results.WriteString(fmt.Sprintf("--- %s (line %d) ---\n", relPath, i+1))

				// Show 1 line before, match, 1 line after
				start := i - 1
				if start < 0 {
					start = 0
				}
				end := i + 2
				if end > len(lines) {
					end = len(lines)
				}
				for j := start; j < end; j++ {
					results.WriteString(lines[j] + "\n")
				}
				matchCount++
				if matchCount >= 100 {
					return fmt.Errorf("limit reached")
				}
			}
		}
		return nil
	})

	if matchCount == 0 {
		return toolSuccess(id, "no matches found for: "+a.Query)
	}
	return toolSuccess(id, results.String())
}

// --- Helpers ---

func firstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		if len(line) > 80 {
			return line[:80] + "..."
		}
		return line
	}
	return ""
}

func toolSuccess(id json.RawMessage, text string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
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
				{"type": "text", "text": msg},
			},
			"isError": true,
		},
	}
}

func writeJSON(w *os.File, v any) {
	data, _ := json.Marshal(v)
	w.Write(data)
	w.Write([]byte("\n"))
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine working directory: %v\n", err)
		os.Exit(1)
	}
	s := newServer(root)
	s.serve()
}
