//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// E2E tests run full multi-step journeys against the compiled binary,
// verifying that the MCP server works as a real agent would use it.

var binaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "txtscape-mcp-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "txtscape-mcp")

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	moduleRoot := filepath.Dir(wd)

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = moduleRoot
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

type mcpClient struct {
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	cmd     *exec.Cmd
	workDir string
}

func startServer(t *testing.T) *mcpClient {
	t.Helper()
	workDir := t.TempDir()
	pagesDir := filepath.Join(workDir, ".txtscape", "pages")
	os.MkdirAll(pagesDir, 0o755)

	cmd := exec.Command(binaryPath)
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Wait()
	})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	return &mcpClient{stdin: stdin, stdout: scanner, cmd: cmd, workDir: workDir}
}

func (c *mcpClient) call(t *testing.T, id int, method string, params any) map[string]any {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		t.Fatal(err)
	}
	if !c.stdout.Scan() {
		t.Fatal("no response from server")
	}
	var resp map[string]any
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, c.stdout.Text())
	}
	return resp
}

func (c *mcpClient) callTool(t *testing.T, id int, name string, args map[string]string) map[string]any {
	t.Helper()
	rawArgs, _ := json.Marshal(args)
	return c.call(t, id, "tools/call", map[string]any{
		"name":      name,
		"arguments": json.RawMessage(rawArgs),
	})
}

func extractText(t *testing.T, resp map[string]any) string {
	t.Helper()
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content in response: %v", resp)
	}
	item, _ := content[0].(map[string]any)
	return item["text"].(string)
}

func isError(resp map[string]any) bool {
	result, _ := resp["result"].(map[string]any)
	isErr, _ := result["isError"].(bool)
	return isErr
}

// --- Journeys ---

func TestJourney_BuildProjectMemory_CoreUseCase(t *testing.T) {
	// Journey: Agent builds project memory from scratch
	//
	// Business context: This is the primary use case. An agent joins a project,
	// records decisions and patterns, then searches and browses them later.
	// This proves the full lifecycle: create → organize → discover → update → delete.
	//
	// Steps:
	//   1. Initialize the MCP connection
	//   2. List tools — verify all 5 are present
	//   3. List empty root — get "(empty directory)"
	//   4. Put several pages in organized folders
	//   5. List root — see folders
	//   6. List a subfolder — see files with previews
	//   7. Get a specific page — content matches
	//   8. Search across all pages — find relevant matches
	//   9. Update a page — content changes
	//   10. Delete a page — confirm gone
	//   11. Search for deleted content — no longer found
	//
	// Expected: All steps succeed. Memory is created, discoverable, updatable, deletable.

	c := startServer(t)

	// Step 1: Initialize
	initResp := c.call(t, 1, "initialize", nil)
	result, _ := initResp["result"].(map[string]any)
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "txtscape" {
		t.Fatalf("step 1: server name = %v, want txtscape", info["name"])
	}

	// Step 2: List tools
	toolsResp := c.call(t, 2, "tools/list", nil)
	toolsResult, _ := toolsResp["result"].(map[string]any)
	tools, _ := toolsResult["tools"].([]any)
	if len(tools) != 5 {
		t.Fatalf("step 2: got %d tools, want 5", len(tools))
	}

	// Step 3: List empty root
	listResp := c.callTool(t, 3, "list_pages", map[string]string{})
	listText := extractText(t, listResp)
	if !strings.Contains(listText, "empty") {
		t.Fatalf("step 3: expected empty directory, got: %s", listText)
	}

	// Step 4: Put several organized pages
	pages := []struct {
		path    string
		content string
	}{
		{"decisions/flat-files.txt", "# Use Flat Files\n\nWe chose flat .txt files over PostgreSQL.\nReason: zero dependencies, git-native, diffable."},
		{"decisions/mcp-only.txt", "# MCP Stdio Only\n\nNo HTTP server. The MCP stdio interface is the only entry point.\nReason: simplicity, no ports, no daemon."},
		{"patterns/error-handling.txt", "# Error Handling\n\nReturn errors, don't panic.\nUse fmt.Errorf with %%w for wrapping."},
		{"bugs/resolved/jwt-fix.txt", "# JWT Expiry Fix\n\nFixed by switching to RS256.\nRoot cause: symmetric key rotation was broken."},
	}
	for i, p := range pages {
		resp := c.callTool(t, 10+i, "put_page", map[string]string{
			"path":    p.path,
			"content": p.content,
		})
		if isError(resp) {
			t.Fatalf("step 4: put_page %s failed: %s", p.path, extractText(t, resp))
		}
	}

	// Step 5: List root — see folders
	rootResp := c.callTool(t, 20, "list_pages", map[string]string{})
	rootText := extractText(t, rootResp)
	if !strings.Contains(rootText, "📁 decisions") {
		t.Fatalf("step 5: root listing should show decisions folder, got: %s", rootText)
	}
	if !strings.Contains(rootText, "📁 patterns") {
		t.Fatalf("step 5: root listing should show patterns folder, got: %s", rootText)
	}
	if !strings.Contains(rootText, "📁 bugs") {
		t.Fatalf("step 5: root listing should show bugs folder, got: %s", rootText)
	}

	// Step 6: List decisions subfolder — see files with previews
	decResp := c.callTool(t, 21, "list_pages", map[string]string{"path": "decisions"})
	decText := extractText(t, decResp)
	if !strings.Contains(decText, "flat-files.txt") {
		t.Fatalf("step 6: decisions listing should contain flat-files.txt, got: %s", decText)
	}
	if !strings.Contains(decText, "# Use Flat Files") {
		t.Fatalf("step 6: listing should show first-line preview, got: %s", decText)
	}

	// Step 7: Get a specific page
	getResp := c.callTool(t, 22, "get_page", map[string]string{"path": "patterns/error-handling.txt"})
	getText := extractText(t, getResp)
	if !strings.Contains(getText, "Return errors, don't panic") {
		t.Fatalf("step 7: page content mismatch, got: %s", getText)
	}

	// Step 8: Search across all pages
	searchResp := c.callTool(t, 23, "search_pages", map[string]string{"query": "PostgreSQL"})
	searchText := extractText(t, searchResp)
	if !strings.Contains(searchText, "flat-files.txt") {
		t.Fatalf("step 8: search should find flat-files.txt, got: %s", searchText)
	}
	if strings.Contains(searchText, "error-handling.txt") {
		t.Fatalf("step 8: search should NOT find error-handling.txt")
	}

	// Step 9: Update a page
	updateResp := c.callTool(t, 24, "put_page", map[string]string{
		"path":    "decisions/flat-files.txt",
		"content": "# Use Flat Files (Updated)\n\nWe chose flat .txt files over PostgreSQL.\nReason: zero dependencies.\nUpdate: confirmed this works great after dogfooding.",
	})
	if isError(updateResp) {
		t.Fatalf("step 9: update failed: %s", extractText(t, updateResp))
	}
	rereadResp := c.callTool(t, 25, "get_page", map[string]string{"path": "decisions/flat-files.txt"})
	rereadText := extractText(t, rereadResp)
	if !strings.Contains(rereadText, "Updated") {
		t.Fatalf("step 9: content should contain 'Updated', got: %s", rereadText)
	}

	// Step 10: Delete a page
	delResp := c.callTool(t, 26, "delete_page", map[string]string{"path": "bugs/resolved/jwt-fix.txt"})
	if isError(delResp) {
		t.Fatalf("step 10: delete failed: %s", extractText(t, delResp))
	}
	// Verify 404
	goneResp := c.callTool(t, 27, "get_page", map[string]string{"path": "bugs/resolved/jwt-fix.txt"})
	if !isError(goneResp) {
		t.Fatal("step 10: deleted page should return error on get")
	}

	// Step 11: Search for deleted content
	searchGoneResp := c.callTool(t, 28, "search_pages", map[string]string{"query": "RS256"})
	searchGoneText := extractText(t, searchGoneResp)
	if !strings.Contains(searchGoneText, "no matches") {
		t.Fatalf("step 11: search for deleted content should find nothing, got: %s", searchGoneText)
	}
}

func TestJourney_PathValidationBoundaries_SecurityBoundary(t *testing.T) {
	// Journey: Agent tests the limits of what paths are allowed
	//
	// Business context: Path validation is a security boundary. The tool must never
	// write outside .txtscape/pages/. This journey tries various attack vectors.
	//
	// Steps:
	//   1. Path traversal — rejected
	//   2. Non-.txt extension — rejected
	//   3. Too deep — rejected
	//   4. Exactly at depth limit — accepted
	//   5. Uppercase folder — rejected
	//   6. Backslash path — rejected
	//
	// Expected: All attacks rejected, valid boundary case accepted.

	c := startServer(t)
	c.call(t, 1, "initialize", nil)

	attacks := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"path traversal", "../../../etc/passwd.txt", true},
		{"non-txt extension", "notes.md", true},
		{"too deep (11 levels)", "a/b/c/d/e/f/g/h/i/j/file.txt", true},
		{"max depth (10 levels)", "a/b/c/d/e/f/g/h/i/file.txt", false},
		{"uppercase folder", "Decisions/bad.txt", true},
		{"backslash path", `decisions\bad.txt`, true},
	}

	for i, a := range attacks {
		resp := c.callTool(t, 100+i, "put_page", map[string]string{
			"path":    a.path,
			"content": "test content",
		})
		gotErr := isError(resp)
		if gotErr != a.wantErr {
			t.Errorf("%s: isError = %v, want %v (response: %s)", a.name, gotErr, a.wantErr, extractText(t, resp))
		}
	}
}
