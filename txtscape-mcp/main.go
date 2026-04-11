package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	maxDepth   = 10
	maxSize    = 1048576 // 1MB
	pagesDir   = ".txtscape/pages"
	configFile = ".txtscape/config.json"
)

// --- Concerns config ---

type concern struct {
	FolderName  string `json:"folderName"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Template    string `json:"template,omitempty"`
}

type txtscapeConfig struct {
	Concerns []concern `json:"concerns"`
	Nickname string    `json:"nickname,omitempty"`
}

func (s *server) nickname() string {
	cfg := s.loadConfig()
	if cfg != nil && cfg.Nickname != "" {
		return cfg.Nickname
	}
	return "journal"
}

func (s *server) loadConfig() *txtscapeConfig {
	data, err := os.ReadFile(filepath.Join(s.root, configFile))
	if err != nil {
		return nil
	}
	var cfg txtscapeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func (s *server) baseInstructions() string {
	name := s.nickname()
	return fmt.Sprintf(
		"%[1]s is this project's local knowledge base, managed by txtscape. "+
			"Use put_page to store decisions, patterns, and knowledge as .txt files. "+
			"Use search_pages to find relevant memories. "+
			"Use list_pages to browse the directory tree with previews. "+
			"Files are plain text with markdown formatting. "+
			"All %[1]s pages are stored in .txtscape/pages/ and should be committed to git. "+
			"When the user says \"%[1]s\" or \"txtscape\" or asks you to store or look up project knowledge, they mean this tool. "+
			"Path rules: files must end in .txt, folder names are lowercase alphanumeric/hyphens/underscores (max 50 chars each), max 10 folder levels deep. "+
			"File size limit: 1MB per page. Search returns up to 100 matches. "+
			"Use str_replace_page for surgical edits (old_str must match exactly once). "+
			"Use snapshot to load all pages in one call. "+
			"Use related_pages to discover cross-references between pages. "+
			"Use page_history to see git commit history for a page. "+
			"get_page returns a hash for optimistic concurrency — pass expected_hash to put_page to prevent stale overwrites.",
		name,
	)
}

func (s *server) buildInstructions() string {
	cfg := s.loadConfig()
	base := s.baseInstructions()
	if cfg == nil || len(cfg.Concerns) == 0 {
		return base
	}

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\nThis project's memory is organized into the following concerns:\n")
	for _, c := range cfg.Concerns {
		b.WriteString(fmt.Sprintf("- %s/ (%s): %s\n", c.FolderName, c.Label, c.Description))
		if c.Template != "" {
			b.WriteString(fmt.Sprintf("  Template for new pages:\n  %s\n", c.Template))
		}
	}
	b.WriteString("Pages can also exist outside of concern folders for ad-hoc notes.")
	return b.String()
}

// folderWarning returns a warning string if the path's top-level folder is not
// a configured concern. Returns "" if no config exists or the folder matches.
func (s *server) folderWarning(cleanPath string) string {
	cfg := s.loadConfig()
	if cfg == nil || len(cfg.Concerns) == 0 {
		return ""
	}
	parts := strings.SplitN(cleanPath, "/", 2)
	if len(parts) < 2 {
		// file at root level, no folder
		return ""
	}
	folder := parts[0]
	for _, c := range cfg.Concerns {
		if c.FolderName == folder {
			return ""
		}
	}
	var names []string
	for _, c := range cfg.Concerns {
		names = append(names, c.FolderName+"/")
	}
	return fmt.Sprintf("\nwarning: folder %q is not a configured concern. Known concerns: %s", folder, strings.Join(names, ", "))
}

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

	// Normalize: trim slashes, collapse double slashes
	raw = strings.Trim(raw, "/")
	for strings.Contains(raw, "//") {
		raw = strings.ReplaceAll(raw, "//", "/")
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
			return "", fmt.Errorf("invalid folder name %q: must be 1-50 lowercase alphanumeric/hyphens/underscores", p)
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
					"version": "0.0.4",
				},
				"instructions": s.buildInstructions(),
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
			"description": "Read a .txt page from project memory. Returns content and a _meta.hash for optimistic concurrency (pass to put_page/str_replace_page/append_page as expected_hash).",
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
			"description": "Create or update a .txt page in project memory. Folders are created automatically. Max 1MB per page. Path must end in .txt. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars each, max 10 levels deep. Supports optimistic concurrency: pass expected_hash (from get_page) to prevent stale overwrites.",
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
					"expected_hash": map[string]any{
						"type":        "string",
						"description": "Optional. SHA-256 hash from get_page. If provided and doesn't match current file hash, the update is rejected to prevent stale overwrites.",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			"name":        "append_page",
			"description": "Append content to an existing .txt page in project memory. Creates the page if it doesn't exist. Max 1MB total size.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path relative to .txtscape/pages/. Must end in .txt.",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Text to append to the end of the page.",
					},
					"expected_hash": map[string]any{
						"type":        "string",
						"description": "Optional. SHA-256 hash from get_page for optimistic concurrency.",
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
			"name":        "move_page",
			"description": "Move/rename a .txt page in project memory. Destination folders are created automatically. Fails if destination already exists.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Current page path. Must end in .txt.",
					},
					"new_path": map[string]any{
						"type":        "string",
						"description": "New page path. Must end in .txt. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars. Max 10 levels deep.",
					},
				},
				"required": []string{"path", "new_path"},
			},
		},
		{
			"name":        "list_pages",
			"description": "List files and folders in project memory. Returns the first line of each file as a preview. Pass empty path or '/' to list the root. Set recursive=true to return the full tree.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Folder path to list, or empty/'/' for root. Folder names: lowercase alphanumeric/hyphens/underscores, max 50 chars. Example: decisions",
					},
					"recursive": map[string]any{
						"type":        "boolean",
						"description": "If true, list all files in all subfolders recursively. Default: false.",
					},
				},
			},
		},
		{
			"name":        "search_pages",
			"description": "Search across all pages in project memory. Returns matching lines with surrounding context. Returns up to 100 matches, case-insensitive. Supports regex with isRegex=true.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Text or regex pattern to search for (case-insensitive). Returns up to 100 matches with 1 line of surrounding context each.",
					},
					"isRegex": map[string]any{
						"type":        "boolean",
						"description": "If true, treat query as a Go regex pattern. Default: false (substring match).",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "str_replace_page",
			"description": "Replace an exact string in a page with a new string. The old_str must appear exactly once in the page (no ambiguity). Use this for surgical edits without rewriting the whole page.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path relative to .txtscape/pages/. Must end in .txt.",
					},
					"old_str": map[string]any{
						"type":        "string",
						"description": "The exact string to find. Must appear exactly once in the page.",
					},
					"new_str": map[string]any{
						"type":        "string",
						"description": "The replacement string.",
					},
					"expected_hash": map[string]any{
						"type":        "string",
						"description": "Optional. SHA-256 hash from get_page for optimistic concurrency.",
					},
				},
				"required": []string{"path", "old_str", "new_str"},
			},
		},
		{
			"name":        "snapshot",
			"description": "Read all pages in a subtree and return them concatenated with path headers. Useful for loading all project memory in one call. Pass empty path for the full tree, or a folder path for a subtree. Returns up to 100 pages per call — use offset to paginate.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Folder path to snapshot, or empty for root. Example: decisions",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of pages to skip (for pagination). Default 0.",
					},
				},
			},
		},
		{
			"name":        "related_pages",
			"description": "Find pages related to a given page. Returns outgoing references (pages mentioned in this page's content) and incoming references (pages that mention this page).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path to find relations for. Must end in .txt.",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "page_history",
			"description": "Show the git commit history for a page. Returns commit hashes, dates, and messages. Set include_diff=true to see what changed in each commit. Requires the project to be a git repository.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Page path to show history for. Must end in .txt.",
					},
					"include_diff": map[string]any{
						"type":        "boolean",
						"description": "If true, include the patch/diff for each commit. Default: false.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of commits to return. Default: 20.",
					},
				},
				"required": []string{"path"},
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
	case "move_page":
		return s.handleMovePage(req.ID, params.Arguments)
	case "get_page":
		return s.handleGetPage(req.ID, params.Arguments)
	case "put_page":
		return s.handlePutPage(req.ID, params.Arguments)
	case "append_page":
		return s.handleAppendPage(req.ID, params.Arguments)
	case "delete_page":
		return s.handleDeletePage(req.ID, params.Arguments)
	case "list_pages":
		return s.handleListPages(req.ID, params.Arguments)
	case "search_pages":
		return s.handleSearchPages(req.ID, params.Arguments)
	case "str_replace_page":
		return s.handleStrReplacePage(req.ID, params.Arguments)
	case "snapshot":
		return s.handleSnapshot(req.ID, params.Arguments)
	case "related_pages":
		return s.handleRelatedPages(req.ID, params.Arguments)
	case "page_history":
		return s.handlePageHistory(req.ID, params.Arguments)
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
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(data)},
			},
			"_meta": map[string]any{
				"hash": hashHex,
			},
		},
	}
}

func (s *server) handlePutPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expected_hash"`
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
	_, existErr := os.Stat(fullPath)
	isUpdate := existErr == nil

	// Optimistic concurrency check
	if a.ExpectedHash != "" && isUpdate {
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return toolError(id, "reading page for hash check: "+err.Error())
		}
		hash := sha256.Sum256(existing)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash != a.ExpectedHash {
			return toolError(id, "hash mismatch: page was modified since last read")
		}
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return toolError(id, "creating directory: "+err.Error())
	}
	if err := os.WriteFile(fullPath, []byte(a.Content), 0o644); err != nil {
		return toolError(id, "writing page: "+err.Error())
	}
	warn := s.folderWarning(clean)
	if isUpdate {
		return toolSuccessWithHash(id, "page updated: "+a.Path+warn, []byte(a.Content))
	}
	return toolSuccessWithHash(id, "page created: "+a.Path+warn, []byte(a.Content))
}

func (s *server) handleAppendPage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expected_hash"`
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

	fullPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(clean))

	// Read existing content if file exists
	existing, readErr := os.ReadFile(fullPath)
	isNew := os.IsNotExist(readErr)
	if readErr != nil && !isNew {
		return toolError(id, "reading page: "+readErr.Error())
	}

	// Optimistic concurrency check
	if a.ExpectedHash != "" && !isNew {
		hash := sha256.Sum256(existing)
		if hex.EncodeToString(hash[:]) != a.ExpectedHash {
			return toolError(id, "hash mismatch: page was modified since last read")
		}
	}

	newContent := append(existing, []byte(a.Content)...)
	if len(newContent) > maxSize {
		return toolError(id, fmt.Sprintf("appended content would exceed maximum size of %d bytes", maxSize))
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return toolError(id, "creating directory: "+err.Error())
	}
	if err := os.WriteFile(fullPath, newContent, 0o644); err != nil {
		return toolError(id, "writing page: "+err.Error())
	}
	warn := s.folderWarning(clean)
	if isNew {
		return toolSuccessWithHash(id, "page created: "+a.Path+warn, newContent)
	}
	return toolSuccessWithHash(id, "page appended: "+a.Path+warn, newContent)
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

	// Clean up empty parent directories up to pages root
	dir := filepath.Dir(fullPath)
	for dir != s.pagesRoot() {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}

	return toolSuccess(id, "page deleted: "+a.Path)
}

func (s *server) handleMovePage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	if a.NewPath == "" {
		return toolError(id, "new_path is required")
	}
	cleanFrom, err := validatePath(a.Path)
	if err != nil {
		return toolError(id, "path: "+err.Error())
	}
	cleanTo, err := validatePath(a.NewPath)
	if err != nil {
		return toolError(id, "new_path: "+err.Error())
	}

	fromPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(cleanFrom))
	toPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(cleanTo))

	// Source must exist
	if _, err := os.Stat(fromPath); os.IsNotExist(err) {
		return toolError(id, "page not found: "+a.Path)
	}
	// Destination must NOT exist
	if _, err := os.Stat(toPath); err == nil {
		return toolError(id, "destination already exists: "+a.NewPath)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return toolError(id, "creating directory: "+err.Error())
	}
	// Move the file
	if err := os.Rename(fromPath, toPath); err != nil {
		return toolError(id, "moving page: "+err.Error())
	}

	// Clean up empty source parent directories
	dir := filepath.Dir(fromPath)
	for dir != s.pagesRoot() {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}

	// Read destination to return hash
	warn := s.folderWarning(cleanTo)
	data, err := os.ReadFile(toPath)
	if err != nil {
		return toolSuccess(id, "page moved: "+a.Path+" → "+a.NewPath+warn)
	}
	return toolSuccessWithHash(id, "page moved: "+a.Path+" → "+a.NewPath+warn, data)
}

func (s *server) handleListPages(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
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

	if a.Recursive {
		return s.listPagesRecursive(id, dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			if a.Path == "" {
				// Fresh project: no pages directory yet. Treat as empty, not an error.
				return toolSuccess(id, "(empty directory)")
			}
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

func (s *server) listPagesRecursive(id json.RawMessage, root string) jsonrpcResponse {
	var buf strings.Builder
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		relPath, _ := filepath.Rel(root, path)
		relPath = filepath.ToSlash(relPath)
		preview := firstLine(path)
		buf.WriteString(fmt.Sprintf("📄 %s  —  %s\n", relPath, preview))
		return nil
	})
	if buf.Len() == 0 {
		return toolSuccess(id, "(empty directory)")
	}
	return toolSuccess(id, buf.String())
}

func (s *server) handleSearchPages(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Query   string `json:"query"`
		IsRegex bool   `json:"isRegex"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Query == "" {
		return toolError(id, "query is required")
	}

	// Build the match function
	var matches func(string) bool
	if a.IsRegex {
		re, err := regexp.Compile("(?i)" + a.Query)
		if err != nil {
			return toolError(id, "invalid regex: "+err.Error())
		}
		matches = func(s string) bool { return re.MatchString(s) }
	} else {
		query := strings.ToLower(a.Query)
		matches = func(s string) bool { return strings.Contains(strings.ToLower(s), query) }
	}

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

		// Match against the file path itself
		pathMatched := false
		if matches(relPath) {
			pathMatched = true
			if matchCount > 0 {
				results.WriteString("\n")
			}
			preview := firstLine(path)
			results.WriteString(fmt.Sprintf("--- %s (path match) ---\n%s\n", relPath, preview))
			matchCount++
			if matchCount >= 100 {
				return fmt.Errorf("limit reached")
			}
		}

		lines := strings.Split(string(data), "\n")

		for i, line := range lines {
			if matches(line) {
				// Skip first line if path already matched (preview already shows it)
				if pathMatched && i == 0 {
					continue
				}
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

// --- New tool handlers ---

func (s *server) handleStrReplacePage(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path         string `json:"path"`
		OldStr       string `json:"old_str"`
		NewStr       string `json:"new_str"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(id, "invalid arguments")
	}
	if a.Path == "" {
		return toolError(id, "path is required")
	}
	if a.OldStr == "" {
		return toolError(id, "old_str is required")
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

	// Optimistic concurrency check
	if a.ExpectedHash != "" {
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != a.ExpectedHash {
			return toolError(id, "hash mismatch: page was modified since last read")
		}
	}

	content := string(data)
	count := strings.Count(content, a.OldStr)
	if count == 0 {
		return toolError(id, "old_str not found in page")
	}
	if count > 1 {
		return toolError(id, fmt.Sprintf("old_str found %d times (multiple matches, must be unique)", count))
	}

	newContent := strings.Replace(content, a.OldStr, a.NewStr, 1)
	if len(newContent) > maxSize {
		return toolError(id, fmt.Sprintf("replacement would exceed maximum size of %d bytes", maxSize))
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0o644); err != nil {
		return toolError(id, "writing page: "+err.Error())
	}

	// Build a diff snippet showing ±3 lines around the replacement
	snippet := diffSnippet(content, newContent, a.OldStr, a.NewStr)
	verb := "replaced in: "
	if a.NewStr == "" {
		verb = "deleted from: "
	}
	warn := s.folderWarning(clean)
	return toolSuccessWithHash(id, verb+a.Path+warn+"\n\n"+snippet, []byte(newContent))
}

func (s *server) handleSnapshot(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
	}
	if args != nil {
		json.Unmarshal(args, &a)
	}

	root := s.pagesRoot()
	if a.Path != "" && a.Path != "/" {
		parts := strings.Split(strings.Trim(a.Path, "/"), "/")
		for _, p := range parts {
			if !folderPartRe.MatchString(p) {
				return toolError(id, fmt.Sprintf("invalid folder name %q", p))
			}
		}
		root = filepath.Join(root, filepath.FromSlash(a.Path))
		if _, err := os.Stat(root); os.IsNotExist(err) {
			return toolError(id, "directory not found: "+a.Path)
		}
	}

	const maxPages = 100

	// First pass: collect pages
	type pageEntry struct {
		relPath string
		data    []byte
	}
	var pages []pageEntry
	totalBytes := 0

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(root, path)
		relPath = filepath.ToSlash(relPath)
		pages = append(pages, pageEntry{relPath: relPath, data: data})
		totalBytes += len(data)
		return nil
	})

	if len(pages) == 0 {
		if a.Path != "" && a.Path != "/" {
			return toolSuccess(id, fmt.Sprintf("(no pages in %s/)", strings.Trim(a.Path, "/")))
		}
		return toolSuccess(id, "(no pages yet)")
	}

	// Apply offset
	start := a.Offset
	if start < 0 {
		start = 0
	}
	if start >= len(pages) {
		return toolSuccess(id, fmt.Sprintf("(empty — offset %d past %d pages)", start, len(pages)))
	}

	end := start + maxPages
	truncated := end < len(pages)
	if end > len(pages) {
		end = len(pages)
	}

	var buf strings.Builder
	// Summary header
	if start == 0 && !truncated {
		buf.WriteString(fmt.Sprintf("(%d pages, %d bytes)\n\n", len(pages), totalBytes))
	} else {
		buf.WriteString(fmt.Sprintf("(showing %d–%d of %d pages, %d bytes total)\n\n", start+1, end, len(pages), totalBytes))
	}

	for i := start; i < end; i++ {
		p := pages[i]
		if i > start {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("=== %s ===\n", p.relPath))
		buf.WriteString(string(p.data))
		if len(p.data) > 0 && p.data[len(p.data)-1] != '\n' {
			buf.WriteString("\n")
		}
	}

	if truncated {
		buf.WriteString(fmt.Sprintf("\n(truncated: showing %d–%d of %d pages)\n", start+1, end, len(pages)))
	}

	return toolSuccess(id, buf.String())
}

func (s *server) handleRelatedPages(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
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

	// Collect all page paths
	var allPages []string
	filepath.Walk(s.pagesRoot(), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		relPath, _ := filepath.Rel(s.pagesRoot(), path)
		relPath = filepath.ToSlash(relPath)
		allPages = append(allPages, relPath)
		return nil
	})

	content := string(data)
	var outgoing []string
	for _, p := range allPages {
		if p == clean {
			continue
		}
		// Check full path match, suffix match, and filename-only match
		if strings.Contains(content, p) {
			outgoing = append(outgoing, p)
		} else if pageRefersTo(content, p) {
			outgoing = append(outgoing, p)
		}
	}

	var incoming []string
	for _, p := range allPages {
		if p == clean {
			continue
		}
		otherPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(p))
		otherData, readErr := os.ReadFile(otherPath)
		if readErr != nil {
			continue
		}
		otherContent := string(otherData)
		if strings.Contains(otherContent, clean) {
			incoming = append(incoming, p)
		} else if pageRefersTo(otherContent, clean) {
			incoming = append(incoming, p)
		}
	}

	if len(outgoing) == 0 && len(incoming) == 0 {
		return toolSuccess(id, "no related pages found for: "+a.Path)
	}

	var buf strings.Builder
	if len(outgoing) > 0 {
		buf.WriteString("References from this page:\n")
		for _, p := range outgoing {
			buf.WriteString("  → " + p + "\n")
		}
	}
	if len(incoming) > 0 {
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Pages referencing this page:\n")
		for _, p := range incoming {
			buf.WriteString("  ← " + p + "\n")
		}
	}
	return toolSuccess(id, buf.String())
}

func (s *server) handlePageHistory(id json.RawMessage, args json.RawMessage) jsonrpcResponse {
	var a struct {
		Path        string `json:"path"`
		IncludeDiff bool   `json:"include_diff"`
		Limit       int    `json:"limit"`
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

	if a.Limit <= 0 {
		a.Limit = 20
	}

	// Check if the file exists on disk
	fullPath := filepath.Join(s.pagesRoot(), filepath.FromSlash(clean))
	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		return toolError(id, "page not found: "+a.Path)
	}

	relPath := filepath.Join(pagesDir, filepath.FromSlash(clean))

	gitArgs := []string{"-C", s.root, "log", "--follow", fmt.Sprintf("-n%d", a.Limit)}
	if a.IncludeDiff {
		gitArgs = append(gitArgs, "-p", "--pretty=format:commit %H %ai %s", "--")
	} else {
		gitArgs = append(gitArgs, "--pretty=format:%H %ai %s", "--")
	}
	gitArgs = append(gitArgs, relPath)

	out, err := execGit(gitArgs...)
	if err != nil {
		return toolError(id, "git log failed: "+strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) == "" {
		return toolSuccess(id, "no history found for: "+a.Path+" (file exists but is not yet committed to git)")
	}
	return toolSuccess(id, out)
}

func execGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// pageRefersTo checks if content mentions targetPath by suffix or filename.
// For example, content mentioning "patterns/tokens.txt" matches target
// "project/patterns/tokens.txt", and "tokens.txt" matches "deep/folder/tokens.txt".
func pageRefersTo(content, targetPath string) bool {
	// Try all suffixes: "a/b/c.txt", "b/c.txt", "c.txt"
	parts := strings.Split(targetPath, "/")
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], "/")
		if strings.Contains(content, suffix) {
			return true
		}
	}
	return false
}

// diffSnippet builds a unified-diff style snippet around a replacement
// with - (removed), + (added), and space (context) line prefixes.
func diffSnippet(oldContent, newContent, oldStr, newStr string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Find where the old string starts in the old content
	oldIdx := strings.Index(oldContent, oldStr)
	if oldIdx < 0 {
		oldIdx = 0
	}
	oldStartLine := strings.Count(oldContent[:oldIdx], "\n")
	oldStrLines := strings.Split(oldStr, "\n")
	oldEndLine := oldStartLine + len(oldStrLines) // exclusive

	// Find where the new string starts in the new content
	var newStartLine, newEndLine int
	if newStr == "" {
		newStartLine = oldStartLine
		newEndLine = oldStartLine
	} else {
		newIdx := strings.Index(newContent, newStr)
		if newIdx < 0 {
			newStartLine = oldStartLine
			newEndLine = oldStartLine
		} else {
			newStartLine = strings.Count(newContent[:newIdx], "\n")
			newStrLines := strings.Split(newStr, "\n")
			newEndLine = newStartLine + len(newStrLines)
		}
	}

	// Context window: 3 lines before and after
	contextBefore := 3
	contextAfter := 3
	start := oldStartLine - contextBefore
	if start < 0 {
		start = 0
	}
	endOld := oldEndLine + contextAfter
	if endOld > len(oldLines) {
		endOld = len(oldLines)
	}
	endNew := newEndLine + contextAfter
	if endNew > len(newLines) {
		endNew = len(newLines)
	}

	var buf strings.Builder

	// Before-context (same in both)
	for i := start; i < oldStartLine; i++ {
		buf.WriteString(" " + oldLines[i] + "\n")
	}
	// Removed lines
	for i := oldStartLine; i < oldEndLine; i++ {
		buf.WriteString("-" + oldLines[i] + "\n")
	}
	// Added lines
	for i := newStartLine; i < newEndLine; i++ {
		buf.WriteString("+" + newLines[i] + "\n")
	}
	// After-context (from new content)
	for i := newEndLine; i < endNew; i++ {
		buf.WriteString(" " + newLines[i] + "\n")
	}

	return strings.TrimRight(buf.String(), "\n")
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

func toolSuccessWithHash(id json.RawMessage, text string, content []byte) jsonrpcResponse {
	hash := sha256.Sum256(content)
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
			"_meta": map[string]any{
				"hash": hex.EncodeToString(hash[:]),
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
