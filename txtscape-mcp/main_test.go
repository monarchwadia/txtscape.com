package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test helpers ---

// setupTestServer creates a server rooted in a temp directory with .txtscape/pages/ ready.
func setupTestServer(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()
	pagesPath := filepath.Join(root, pagesDir)
	if err := os.MkdirAll(pagesPath, 0o755); err != nil {
		t.Fatalf("creating pages dir: %v", err)
	}
	return newServer(root)
}

// callMethod sends a JSON-RPC request and returns the response.
func callMethod(s *server, method string, params any) jsonrpcResponse {
	var rawParams json.RawMessage
	if params != nil {
		rawParams, _ = json.Marshal(params)
	}
	return s.handleRequest(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  method,
		Params:  rawParams,
	})
}

// callTool sends a tools/call request and returns the response.
func callTool(s *server, name string, args any) jsonrpcResponse {
	rawArgs, _ := json.Marshal(args)
	return callMethod(s, "tools/call", toolCallParams{
		Name:      name,
		Arguments: rawArgs,
	})
}

// getTextContent extracts the text string from a tool response.
func getTextContent(t *testing.T, resp jsonrpcResponse) string {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	// content can be []map[string]any (direct) or []any (after JSON round-trip)
	var text string
	switch content := result["content"].(type) {
	case []map[string]any:
		if len(content) == 0 {
			t.Fatal("no content in result")
		}
		text, _ = content[0]["text"].(string)
	case []any:
		if len(content) == 0 {
			t.Fatal("no content in result")
		}
		item, _ := content[0].(map[string]any)
		text, _ = item["text"].(string)
	default:
		t.Fatalf("content has unexpected type %T", result["content"])
	}
	return text
}

// isToolError returns true if the response has isError: true.
func isToolError(resp jsonrpcResponse) bool {
	if resp.Error != nil {
		return true
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		return false
	}
	isErr, _ := result["isError"].(bool)
	return isErr
}

// --- Initialize tests ---

func TestInitialize_ServerInfo_ReturnsCapabilities(t *testing.T) {
	// Business context: MCP clients need server metadata on connect to discover
	// capabilities and present server info to users.
	// Scenario: Send an initialize request.
	// Expected: Returns server name "txtscape", version, and tool capabilities.
	s := setupTestServer(t)
	resp := callMethod(s, "initialize", nil)

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
		t.Errorf("name = %v, want txtscape", info["name"])
	}
	if info["version"] != "0.1.0" {
		t.Errorf("version = %v, want 0.1.0", info["version"])
	}
}

func TestInitialize_Instructions_ContainsUsageGuidance(t *testing.T) {
	// Business context: The instructions field guides agents on how to use the tools
	// effectively, preventing misuse and improving the agent experience.
	// Scenario: Check initialize response instructions.
	// Expected: Contains key phrases about project memory usage.
	s := setupTestServer(t)
	resp := callMethod(s, "initialize", nil)

	result := resp.Result.(map[string]any)
	instructions, ok := result["instructions"].(string)
	if !ok {
		t.Fatal("instructions is not a string")
	}
	if !strings.Contains(instructions, "project memory") {
		t.Error("instructions should mention 'project memory'")
	}
}

// --- tools/list tests ---

func TestToolsList_DiscoverTools_Returns5Tools(t *testing.T) {
	// Business context: Agents discover available tools via tools/list.
	// The complete set is: get_page, put_page, delete_page, list_pages, search_pages.
	// Scenario: Request tool listing.
	// Expected: Returns exactly 5 tools.
	s := setupTestServer(t)
	resp := callMethod(s, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatal("tools is not a list of maps")
	}
	if len(tools) != 5 {
		t.Errorf("got %d tools, want 5", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"get_page", "put_page", "delete_page", "list_pages", "search_pages"} {
		if !names[want] {
			t.Errorf("missing tool: %s", want)
		}
	}
}

func TestUnknownMethod_ReturnsMethodNotFound(t *testing.T) {
	// Business context: MCP servers must respond with -32601 for unknown methods
	// per the JSON-RPC spec, so agents can handle unsupported methods gracefully.
	// Scenario: Send an unrecognized method name.
	// Expected: Error with code -32601.
	s := setupTestServer(t)
	resp := callMethod(s, "bogus/method", nil)

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

// --- put_page + get_page tests ---

func TestPutPage_NewFile_CreatesFile(t *testing.T) {
	// Business context: The core write operation. Agents store decisions, patterns,
	// and knowledge as .txt files in project memory.
	// Scenario: Put a new page at a simple path.
	// Expected: File is created, success message returned.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "hello.txt",
		"content": "# Hello\n\nThis is a test page.",
	})

	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "created") {
		t.Errorf("expected success message containing 'created', got: %s", text)
	}

	// Verify file exists on disk
	data, err := os.ReadFile(filepath.Join(s.pagesRoot(), "hello.txt"))
	if err != nil {
		t.Fatalf("file not found on disk: %v", err)
	}
	if string(data) != "# Hello\n\nThis is a test page." {
		t.Errorf("file content = %q, want %q", string(data), "# Hello\n\nThis is a test page.")
	}
}

func TestPutPage_NestedPath_CreatesFolders(t *testing.T) {
	// Business context: Memory is organized in folders (decisions/, patterns/, etc.).
	// Putting a file in a nested path should create intermediate directories.
	// Scenario: Put a page at decisions/use-flat-files.txt.
	// Expected: decisions/ folder created, file written.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "decisions/use-flat-files.txt",
		"content": "# Use flat files for storage",
	})

	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}

	data, err := os.ReadFile(filepath.Join(s.pagesRoot(), "decisions", "use-flat-files.txt"))
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if !strings.Contains(string(data), "flat files") {
		t.Error("file content doesn't match")
	}
}

func TestPutPage_UpdateExisting_OverwritesContent(t *testing.T) {
	// Business context: Agents update existing pages as knowledge evolves.
	// Scenario: Write a page, then write to the same path with new content.
	// Expected: File content is replaced.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "evolving.txt",
		"content": "version 1",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "evolving.txt",
		"content": "version 2",
	})

	data, err := os.ReadFile(filepath.Join(s.pagesRoot(), "evolving.txt"))
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != "version 2" {
		t.Errorf("content = %q, want %q", string(data), "version 2")
	}
}

func TestGetPage_ExistingFile_ReturnsContent(t *testing.T) {
	// Business context: The core read operation. Agents retrieve stored
	// decisions and knowledge to inform their work.
	// Scenario: Write a page, then read it back.
	// Expected: Content matches what was written.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "notes.txt",
		"content": "# Important Notes\n\nDon't forget.",
	})

	resp := callTool(s, "get_page", map[string]string{"path": "notes.txt"})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if text != "# Important Notes\n\nDon't forget." {
		t.Errorf("content = %q, want %q", text, "# Important Notes\n\nDon't forget.")
	}
}

func TestGetPage_NotFound_ReturnsError(t *testing.T) {
	// Business context: Agents should get clear feedback when a page doesn't exist,
	// so they can decide whether to create it.
	// Scenario: Read a non-existent page.
	// Expected: Error with "not found" message.
	s := setupTestServer(t)
	resp := callTool(s, "get_page", map[string]string{"path": "missing.txt"})

	if !isToolError(resp) {
		t.Fatal("expected tool error, got success")
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", text)
	}
}

// --- delete_page tests ---

func TestDeletePage_ExistingFile_RemovesFile(t *testing.T) {
	// Business context: Agents need to clean up outdated or incorrect memories.
	// Scenario: Create a page, then delete it.
	// Expected: File is removed from disk.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "temporary.txt",
		"content": "will be deleted",
	})

	resp := callTool(s, "delete_page", map[string]string{"path": "temporary.txt"})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}

	_, err := os.ReadFile(filepath.Join(s.pagesRoot(), "temporary.txt"))
	if !os.IsNotExist(err) {
		t.Error("file should not exist after deletion")
	}
}

func TestDeletePage_NotFound_ReturnsError(t *testing.T) {
	// Business context: Deleting a non-existent page should return clear feedback.
	// Scenario: Delete a page that doesn't exist.
	// Expected: Error with "not found" message.
	s := setupTestServer(t)
	resp := callTool(s, "delete_page", map[string]string{"path": "ghost.txt"})

	if !isToolError(resp) {
		t.Fatal("expected tool error, got success")
	}
}

// --- list_pages tests ---

func TestListPages_RootWithFiles_ShowsPreviews(t *testing.T) {
	// Business context: Agents need to orient themselves in project memory.
	// list_pages shows first-line previews so the agent can decide what to read
	// without opening every file — reducing round trips.
	// Scenario: Create two files, list root.
	// Expected: Both files shown with their first line as preview.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "decisions.txt",
		"content": "# Architectural Decisions\n\nList of decisions.",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "patterns.txt",
		"content": "# Coding Patterns\n\nList of patterns.",
	})

	resp := callTool(s, "list_pages", map[string]string{})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)

	if !strings.Contains(text, "decisions.txt") {
		t.Error("listing should contain decisions.txt")
	}
	if !strings.Contains(text, "patterns.txt") {
		t.Error("listing should contain patterns.txt")
	}
	if !strings.Contains(text, "# Architectural Decisions") {
		t.Error("listing should contain first line preview of decisions.txt")
	}
}

func TestListPages_WithSubfolders_ShowsFolderIcons(t *testing.T) {
	// Business context: Agents should see folders and files distinguished clearly
	// to navigate the memory tree efficiently.
	// Scenario: Create a file in a subfolder, list root.
	// Expected: Subfolder shown with folder icon.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "decisions/first.txt",
		"content": "# First decision",
	})

	resp := callTool(s, "list_pages", map[string]string{})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "📁") {
		t.Error("listing should contain folder icon")
	}
	if !strings.Contains(text, "decisions") {
		t.Error("listing should contain 'decisions' folder")
	}
}

func TestListPages_EmptyRoot_ReturnsEmpty(t *testing.T) {
	// Business context: A fresh project with no memories should get clear feedback.
	// Scenario: List an empty root.
	// Expected: "(empty directory)" message.
	s := setupTestServer(t)
	resp := callTool(s, "list_pages", map[string]string{})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "empty") {
		t.Errorf("expected 'empty' message, got: %s", text)
	}
}

func TestListPages_Subfolder_ListsContents(t *testing.T) {
	// Business context: Agents drill into specific folders to find relevant memories.
	// Scenario: Create files in a subfolder, list that subfolder.
	// Expected: Only files in that subfolder are shown.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "decisions/first.txt",
		"content": "# First",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "decisions/second.txt",
		"content": "# Second",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "root-file.txt",
		"content": "# Root",
	})

	resp := callTool(s, "list_pages", map[string]string{"path": "decisions"})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "first.txt") {
		t.Error("should list first.txt")
	}
	if !strings.Contains(text, "second.txt") {
		t.Error("should list second.txt")
	}
	if strings.Contains(text, "root-file.txt") {
		t.Error("should NOT list root-file.txt in decisions/ listing")
	}
}

// --- search_pages tests ---

func TestSearchPages_MatchExists_ReturnsResults(t *testing.T) {
	// Business context: search_pages is the killer tool — it lets agents find
	// relevant memories without reading every file, drastically reducing round trips.
	// Scenario: Create pages, search for a keyword.
	// Expected: Matching lines returned with file path and context.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "decisions/db-choice.txt",
		"content": "# Database Choice\n\nWe chose flat files over PostgreSQL.\nReason: zero dependencies.",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "patterns/errors.txt",
		"content": "# Error Handling\n\nReturn errors, don't panic.",
	})

	resp := callTool(s, "search_pages", map[string]string{"query": "PostgreSQL"})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)

	if !strings.Contains(text, "db-choice.txt") {
		t.Error("results should reference db-choice.txt")
	}
	if !strings.Contains(text, "PostgreSQL") {
		t.Error("results should contain the matching line")
	}
	if strings.Contains(text, "errors.txt") {
		t.Error("results should NOT contain errors.txt (no match)")
	}
}

func TestSearchPages_CaseInsensitive_FindsMatch(t *testing.T) {
	// Business context: Agents may not remember exact casing of terms.
	// Search must be case-insensitive to be practical.
	// Scenario: Search with lowercase for content written in mixed case.
	// Expected: Finds the match.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "notes.txt",
		"content": "# Important NOTE\n\nDon't forget this.",
	})

	resp := callTool(s, "search_pages", map[string]string{"query": "important note"})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "notes.txt") {
		t.Error("case-insensitive search should find match")
	}
}

func TestSearchPages_NoMatch_ReturnsMessage(t *testing.T) {
	// Business context: Clear "no results" feedback helps agents decide to create
	// new memory rather than keep searching.
	// Scenario: Search for a term that doesn't exist.
	// Expected: "no matches found" message.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "notes.txt",
		"content": "# Hello",
	})

	resp := callTool(s, "search_pages", map[string]string{"query": "nonexistent"})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "no matches") {
		t.Errorf("expected 'no matches' message, got: %s", text)
	}
}

// --- Regex search tests ---

func TestSearchPages_Regex_AlternationPattern_MatchesBoth(t *testing.T) {
	// Business context: Agents need to search for related terms in one call,
	// e.g. "(bcrypt|argon2)" to find any password hashing discussion.
	// Scenario: Two pages with different keywords, regex alternation.
	// Expected: Both pages found.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "crypto-a.txt",
		"content": "We use bcrypt for hashing.",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "crypto-b.txt",
		"content": "We considered argon2 but chose not to.",
	})

	resp := callTool(s, "search_pages", map[string]any{
		"query":   "(bcrypt|argon2)",
		"isRegex": true,
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "crypto-a.txt") {
		t.Error("regex should match crypto-a.txt")
	}
	if !strings.Contains(text, "crypto-b.txt") {
		t.Error("regex should match crypto-b.txt")
	}
}

func TestSearchPages_Regex_DotStar_MatchesPattern(t *testing.T) {
	// Business context: Pattern matching like "TODO.*auth" finds TODO items
	// about authentication specifically.
	// Scenario: File with "TODO: fix auth flow", search "TODO.*auth".
	// Expected: Match found.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "todos.txt",
		"content": "TODO: fix auth flow\nTODO: update docs",
	})

	resp := callTool(s, "search_pages", map[string]any{
		"query":   "TODO.*auth",
		"isRegex": true,
	})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "todos.txt") {
		t.Error("regex should match todos.txt")
	}
	if !strings.Contains(text, "auth") {
		t.Error("result should contain the matching line")
	}
}

func TestSearchPages_Regex_InvalidPattern_ReturnsError(t *testing.T) {
	// Business context: Bad regex should produce a clear error, not a panic.
	// Scenario: Invalid regex pattern.
	// Expected: Error about invalid regex.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path": "dummy.txt", "content": "x",
	})

	resp := callTool(s, "search_pages", map[string]any{
		"query":   "[invalid",
		"isRegex": true,
	})
	if !isToolError(resp) {
		t.Fatal("expected error for invalid regex")
	}
}

func TestSearchPages_PlainStillWorks_WhenRegexFalse(t *testing.T) {
	// Business context: Default substring search must still work when isRegex=false or omitted.
	// Scenario: Search with special regex chars but isRegex=false.
	// Expected: Treats query as literal text, finds match.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "special.txt",
		"content": "size limit is 100KB (max).",
	})

	resp := callTool(s, "search_pages", map[string]any{
		"query":   "(max)",
		"isRegex": false,
	})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "special.txt") {
		t.Error("plain search should still find literal match")
	}
}

// --- Path validation tests ---

func TestPutPage_PathTraversal_PreventEscape_ReturnsError(t *testing.T) {
	// Business context: Path traversal could let an agent write outside .txtscape/pages/.
	// This is a security boundary — the tool must NEVER write outside the pages root.
	// Scenario: Attempt to write with ".." in the path.
	// Expected: Rejected with error.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "../../../etc/passwd.txt",
		"content": "hacked",
	})

	if !isToolError(resp) {
		t.Fatal("expected tool error for path traversal, got success")
	}
}

func TestPutPage_NonTxtExtension_EnforceFormat_ReturnsError(t *testing.T) {
	// Business context: Only .txt files are allowed. This keeps the memory clean
	// and prevents binary/executable files from being stored.
	// Scenario: Attempt to write a .md file.
	// Expected: Rejected with error.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "notes.md",
		"content": "# Markdown",
	})

	if !isToolError(resp) {
		t.Fatal("expected tool error for non-.txt extension, got success")
	}
}

func TestPutPage_TooDeep_EnforceDepthLimit_ReturnsError(t *testing.T) {
	// Business context: Unlimited nesting could create unwieldy directory trees.
	// 10 levels is the maximum depth.
	// Scenario: Path with 10 folder segments + 1 filename = 11 levels.
	// Expected: Rejected.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "a/b/c/d/e/f/g/h/i/j/file.txt",
		"content": "too deep",
	})

	if !isToolError(resp) {
		t.Fatal("expected tool error for path too deep, got success")
	}
}

func TestPutPage_MaxDepth_AllowBoundary_Succeeds(t *testing.T) {
	// Business context: 9 folders + 1 filename = 10 levels, which is the maximum allowed.
	// Scenario: Path at exactly the depth limit.
	// Expected: File is created successfully.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "a/b/c/d/e/f/g/h/i/file.txt",
		"content": "just right",
	})

	if isToolError(resp) {
		t.Fatalf("should succeed at max depth, got error: %s", getTextContent(t, resp))
	}
}

func TestPutPage_EmptyPath_RequirePath_ReturnsError(t *testing.T) {
	// Business context: An empty path is meaningless — the agent must specify where to write.
	// Scenario: Put with empty path.
	// Expected: Error.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "",
		"content": "no path",
	})

	if !isToolError(resp) {
		t.Fatal("expected error for empty path")
	}
}

func TestPutPage_EmptyContent_RequireContent_ReturnsError(t *testing.T) {
	// Business context: Empty files are pointless in a memory system.
	// Scenario: Put with empty content.
	// Expected: Error.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "empty.txt",
		"content": "",
	})

	if !isToolError(resp) {
		t.Fatal("expected error for empty content")
	}
}

func TestPutPage_TooLarge_EnforceSizeLimit_ReturnsError(t *testing.T) {
	// Business context: 1MB max prevents accidental storage of massive content.
	// Scenario: Content that exceeds 1MB.
	// Expected: Rejected with error.
	s := setupTestServer(t)
	bigContent := strings.Repeat("x", maxSize+1)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "huge.txt",
		"content": bigContent,
	})

	if !isToolError(resp) {
		t.Fatal("expected error for content exceeding size limit")
	}
}

func TestPutPage_UppercaseFolder_EnforceLowercase_ReturnsError(t *testing.T) {
	// Business context: Folder names must be lowercase for consistency and
	// to avoid case-sensitivity issues across operating systems.
	// Scenario: Folder with uppercase characters.
	// Expected: Rejected with error.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "Decisions/choice.txt",
		"content": "test",
	})

	if !isToolError(resp) {
		t.Fatal("expected error for uppercase folder name")
	}
}

func TestPutPage_BackslashPath_PreventWindowsPaths_ReturnsError(t *testing.T) {
	// Business context: Backslashes could cause inconsistent behavior across OSes.
	// All paths must use forward slashes.
	// Scenario: Path with backslashes.
	// Expected: Rejected.
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    `decisions\choice.txt`,
		"content": "test",
	})

	if !isToolError(resp) {
		t.Fatal("expected error for backslash in path")
	}
}

// --- Unknown tool test ---

func TestValidatePath_LeadingSlash_NormalizeFriendlyInput_Accepted(t *testing.T) {
	// Business context: Agents may prepend '/' to paths. This should be
	// silently normalized rather than producing a cryptic empty-folder error.
	// Scenario: Path with leading slash.
	// Expected: Normalized and accepted.
	got, err := validatePath("/decisions/choice.txt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != "decisions/choice.txt" {
		t.Errorf("got %q, want %q", got, "decisions/choice.txt")
	}
}

func TestValidatePath_TrailingSlash_NormalizeFriendlyInput_Accepted(t *testing.T) {
	// Business context: Trailing slashes are a common typo. Should be
	// normalized rather than failing with "invalid filename".
	// Scenario: Path with trailing slash.
	// Expected: Normalized and accepted.
	got, err := validatePath("decisions/choice.txt/")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != "decisions/choice.txt" {
		t.Errorf("got %q, want %q", got, "decisions/choice.txt")
	}
}

func TestValidatePath_DoubleSlash_NormalizeFriendlyInput_Accepted(t *testing.T) {
	// Business context: Double slashes from string concatenation are common.
	// Should be collapsed rather than failing with empty folder name.
	// Scenario: Path with double slashes.
	// Expected: Normalized and accepted.
	got, err := validatePath("decisions//choice.txt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != "decisions/choice.txt" {
		t.Errorf("got %q, want %q", got, "decisions/choice.txt")
	}
}

func TestPutPage_NewFile_SignalCreated_ReturnsCreated(t *testing.T) {
	// Business context: Agents need to know whether put_page created a new file
	// or overwrote an existing one, to detect accidental overwrites.
	// Scenario: Write a page that doesn't exist yet.
	// Expected: Response says "created".
	s := setupTestServer(t)
	resp := callTool(s, "put_page", map[string]string{
		"path":    "new.txt",
		"content": "fresh content",
	})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "created") {
		t.Errorf("expected 'created' in response for new file, got: %s", text)
	}
}

func TestPutPage_ExistingFile_SignalUpdated_ReturnsUpdated(t *testing.T) {
	// Business context: Agents need to know when they're overwriting existing
	// content. Distinct "updated" signal prevents silent data loss.
	// Scenario: Write a page, then write it again with new content.
	// Expected: Second response says "updated".
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "existing.txt",
		"content": "original",
	})
	resp := callTool(s, "put_page", map[string]string{
		"path":    "existing.txt",
		"content": "replacement",
	})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "updated") {
		t.Errorf("expected 'updated' in response for existing file, got: %s", text)
	}
}

func TestDeletePage_LastFileInFolder_CleanupEmptyParent_RemovesFolder(t *testing.T) {
	// Business context: Ghost empty directories clutter list_pages and confuse
	// agents into thinking content exists in a folder. After deleting the last
	// file, the empty parent folders should be removed up to .txtscape/pages/.
	// Scenario: Create decisions/temp.txt, delete it, check list_pages root.
	// Expected: The "decisions" folder no longer appears.
	s := setupTestServer(t)

	// Create a file in a nested folder
	callTool(s, "put_page", map[string]string{
		"path":    "decisions/temp.txt",
		"content": "temporary",
	})

	// Delete it
	callTool(s, "delete_page", map[string]string{
		"path": "decisions/temp.txt",
	})

	// List root — "decisions" folder should be gone
	resp := callTool(s, "list_pages", map[string]string{})
	text := getTextContent(t, resp)
	if strings.Contains(text, "decisions") {
		t.Errorf("expected empty 'decisions' folder to be cleaned up, got: %s", text)
	}
}

func TestSearchPages_MatchesFilename_DiscoverByName_ReturnsResult(t *testing.T) {
	// Business context: Agents often search for a page by topic name. If the
	// file is named "tools.txt" but the content doesn't literally say "tools",
	// search should still find it by matching the path.
	// Scenario: File at architecture/tools.txt, content says "Five MCP endpoints".
	// Expected: Searching for "tools" matches the filename.
	s := setupTestServer(t)
	dir := filepath.Join(s.pagesRoot(), "arch")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "tools.txt"), []byte("Five MCP endpoints exposed via stdio."), 0o644)

	resp := callTool(s, "search_pages", map[string]string{"query": "tools"})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "arch/tools.txt") {
		t.Errorf("expected filename match for 'tools', got: %s", text)
	}
}

// --- move_page tests ---

func TestListPages_Recursive_FullTree_ReturnsAllFiles(t *testing.T) {
	// Business context: Agents exploring memory must call list_pages repeatedly
	// to walk the tree. A recursive option returns the full tree in one call,
	// saving many round trips and giving complete situational awareness.
	// Scenario: Create files in nested folders, list with recursive=true.
	// Expected: All files shown in tree format with relative paths.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path": "root.txt", "content": "# Root",
	})
	callTool(s, "put_page", map[string]string{
		"path": "decisions/flat-files.txt", "content": "# Flat files",
	})
	callTool(s, "put_page", map[string]string{
		"path": "decisions/stdio.txt", "content": "# Stdio",
	})
	callTool(s, "put_page", map[string]string{
		"path": "patterns/errors/retry.txt", "content": "# Retry",
	})

	resp := callTool(s, "list_pages", map[string]any{
		"path":      "",
		"recursive": true,
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)

	// Should contain all 4 files with full relative paths
	for _, want := range []string{"root.txt", "decisions/flat-files.txt", "decisions/stdio.txt", "patterns/errors/retry.txt"} {
		if !strings.Contains(text, want) {
			t.Errorf("recursive listing missing %q, got:\n%s", want, text)
		}
	}
}

func TestListPages_RecursiveSubfolder_OnlyThatSubtree(t *testing.T) {
	// Business context: Recursive listing of a subfolder should only show
	// files within that subtree, not the entire memory.
	// Scenario: Create files in multiple folders, list one recursively.
	// Expected: Only files under that folder appear.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path": "decisions/a.txt", "content": "# A",
	})
	callTool(s, "put_page", map[string]string{
		"path": "decisions/sub/b.txt", "content": "# B",
	})
	callTool(s, "put_page", map[string]string{
		"path": "patterns/c.txt", "content": "# C",
	})

	resp := callTool(s, "list_pages", map[string]any{
		"path":      "decisions",
		"recursive": true,
	})
	text := getTextContent(t, resp)

	if !strings.Contains(text, "a.txt") {
		t.Error("should contain a.txt")
	}
	if !strings.Contains(text, "sub/b.txt") {
		t.Error("should contain sub/b.txt")
	}
	if strings.Contains(text, "c.txt") {
		t.Error("should NOT contain c.txt from patterns/")
	}
}

func TestListPages_RecursiveEmpty_ReturnsEmpty(t *testing.T) {
	// Business context: Recursive on empty root should still return empty message.
	// Scenario: Recursive list on empty memory.
	// Expected: "(empty)" message.
	s := setupTestServer(t)
	resp := callTool(s, "list_pages", map[string]any{
		"recursive": true,
	})
	text := getTextContent(t, resp)
	if !strings.Contains(text, "empty") {
		t.Errorf("expected 'empty' message, got: %s", text)
	}
}

// --- append_page tests ---

func TestAppendPage_ExistingFile_AppendsContent(t *testing.T) {
	// Business context: Log-style pages (changelogs, session notes) need append
	// without the read-modify-write cycle of get+put.
	// Scenario: Create a page, append to it.
	// Expected: Content is concatenated.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "log.txt",
		"content": "line 1\n",
	})

	resp := callTool(s, "append_page", map[string]string{
		"path":    "log.txt",
		"content": "line 2\n",
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "appended") {
		t.Errorf("expected 'appended' in response, got: %s", text)
	}

	data, _ := os.ReadFile(filepath.Join(s.pagesRoot(), "log.txt"))
	if string(data) != "line 1\nline 2\n" {
		t.Errorf("content = %q, want %q", string(data), "line 1\nline 2\n")
	}
}

func TestAppendPage_NewFile_CreatesFile(t *testing.T) {
	// Business context: Appending to a non-existent page should create it,
	// so agents don't need to check existence first.
	// Scenario: Append to a page that doesn't exist.
	// Expected: File created with the appended content.
	s := setupTestServer(t)
	resp := callTool(s, "append_page", map[string]string{
		"path":    "new-log.txt",
		"content": "first entry\n",
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "created") {
		t.Errorf("expected 'created' for new file append, got: %s", text)
	}

	data, _ := os.ReadFile(filepath.Join(s.pagesRoot(), "new-log.txt"))
	if string(data) != "first entry\n" {
		t.Errorf("content = %q, want %q", string(data), "first entry\n")
	}
}

func TestAppendPage_ExceedsMaxSize_ReturnsError(t *testing.T) {
	// Business context: 1MB limit must be enforced for append too, counting
	// existing content + new content.
	// Scenario: File near max size, append pushes it over.
	// Expected: Error about size limit.
	s := setupTestServer(t)
	// Create a file that's almost at max
	callTool(s, "put_page", map[string]string{
		"path":    "big.txt",
		"content": strings.Repeat("x", maxSize-10),
	})

	resp := callTool(s, "append_page", map[string]string{
		"path":    "big.txt",
		"content": strings.Repeat("y", 20),
	})
	if !isToolError(resp) {
		t.Fatal("expected error when append would exceed max size")
	}
}

func TestAppendPage_EmptyContent_ReturnsError(t *testing.T) {
	// Business context: Appending nothing is pointless.
	// Scenario: Append with empty content.
	// Expected: Error.
	s := setupTestServer(t)
	resp := callTool(s, "append_page", map[string]string{
		"path":    "log.txt",
		"content": "",
	})
	if !isToolError(resp) {
		t.Fatal("expected error for empty content")
	}
}

func TestMovePage_SimpleRename_RelocatePage_MovesFile(t *testing.T) {
	// Business context: Agents need to reorganize memory without the 3-call
	// get+put+delete dance. move_page does it atomically in one call.
	// Scenario: Create a page, move it to a new path.
	// Expected: Old path gone, new path has the content.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "old-name.txt",
		"content": "moveable content",
	})

	resp := callTool(s, "move_page", map[string]string{
		"from": "old-name.txt",
		"to":   "new-name.txt",
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "moved") {
		t.Errorf("expected 'moved' in response, got: %s", text)
	}

	// Old path should not exist
	getResp := callTool(s, "get_page", map[string]string{"path": "old-name.txt"})
	if !isToolError(getResp) {
		t.Error("old path should not exist after move")
	}

	// New path should have content
	getResp = callTool(s, "get_page", map[string]string{"path": "new-name.txt"})
	if isToolError(getResp) {
		t.Fatalf("new path should exist: %s", getTextContent(t, getResp))
	}
	if getTextContent(t, getResp) != "moveable content" {
		t.Error("content should be preserved after move")
	}
}

func TestMovePage_AcrossFolders_RelocatePage_CreatesDestFolder(t *testing.T) {
	// Business context: Moving pages between folders (e.g. from drafts/ to decisions/)
	// should auto-create destination folders, just like put_page does.
	// Scenario: Move from root to a nested folder that doesn't exist.
	// Expected: Destination folder created, file moved.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "draft.txt",
		"content": "draft content",
	})

	resp := callTool(s, "move_page", map[string]string{
		"from": "draft.txt",
		"to":   "decisions/finalized.txt",
	})
	if isToolError(resp) {
		t.Fatalf("unexpected error: %s", getTextContent(t, resp))
	}

	// Verify new location
	data, err := os.ReadFile(filepath.Join(s.pagesRoot(), "decisions", "finalized.txt"))
	if err != nil {
		t.Fatalf("file not at new location: %v", err)
	}
	if string(data) != "draft content" {
		t.Error("content should be preserved")
	}
}

func TestMovePage_SourceNotFound_ReturnsError(t *testing.T) {
	// Business context: Moving a non-existent page should fail clearly.
	// Scenario: Move a page that doesn't exist.
	// Expected: Error with "not found" message.
	s := setupTestServer(t)
	resp := callTool(s, "move_page", map[string]string{
		"from": "ghost.txt",
		"to":   "target.txt",
	})
	if !isToolError(resp) {
		t.Fatal("expected error for missing source")
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found', got: %s", text)
	}
}

func TestMovePage_DestinationExists_PreventOverwrite_ReturnsError(t *testing.T) {
	// Business context: Accidental overwrites via move are dangerous.
	// If the destination already exists, refuse the move.
	// Scenario: Both source and destination exist.
	// Expected: Error indicating destination exists.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "source.txt",
		"content": "source",
	})
	callTool(s, "put_page", map[string]string{
		"path":    "dest.txt",
		"content": "existing",
	})

	resp := callTool(s, "move_page", map[string]string{
		"from": "source.txt",
		"to":   "dest.txt",
	})
	if !isToolError(resp) {
		t.Fatal("expected error when destination exists")
	}
	text := getTextContent(t, resp)
	if !strings.Contains(text, "already exists") {
		t.Errorf("expected 'already exists', got: %s", text)
	}
}

func TestMovePage_CleansUpEmptySourceFolder(t *testing.T) {
	// Business context: After moving the last file out of a folder, the empty
	// folder should be cleaned up (same as delete_page behavior).
	// Scenario: Move the only file out of a folder.
	// Expected: Source folder no longer exists.
	s := setupTestServer(t)
	callTool(s, "put_page", map[string]string{
		"path":    "old-folder/only-file.txt",
		"content": "lonely",
	})

	callTool(s, "move_page", map[string]string{
		"from": "old-folder/only-file.txt",
		"to":   "new-home.txt",
	})

	// old-folder should be gone
	resp := callTool(s, "list_pages", map[string]string{})
	text := getTextContent(t, resp)
	if strings.Contains(text, "old-folder") {
		t.Errorf("empty source folder should be cleaned up, got: %s", text)
	}
}

func TestToolCall_UnknownTool_ReturnsError(t *testing.T) {
	// Business context: Agents may call tools that don't exist (typos, wrong server).
	// Clear error messages help agents self-correct.
	// Scenario: Call a non-existent tool.
	// Expected: Error with tool name in message.
	s := setupTestServer(t)
	resp := callTool(s, "bogus_tool", map[string]string{})

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error for unknown tool")
	}
	if !strings.Contains(resp.Error.Message, "bogus_tool") {
		t.Errorf("error should name the unknown tool, got: %s", resp.Error.Message)
	}
}
