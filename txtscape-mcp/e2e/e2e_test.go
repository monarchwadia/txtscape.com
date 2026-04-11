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

func (c *mcpClient) callTool(t *testing.T, id int, name string, args any) map[string]any {
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

func extractHash(resp map[string]any) string {
	result, _ := resp["result"].(map[string]any)
	meta, _ := result["_meta"].(map[string]any)
	if meta == nil {
		return ""
	}
	h, _ := meta["hash"].(string)
	return h
}

func runGitE2E(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
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
	if len(tools) != 11 {
		t.Fatalf("step 2: got %d tools, want 11", len(tools))
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

	// Step 5: List root — default is recursive, shows all files with full paths
	rootResp := c.callTool(t, 20, "list_pages", map[string]string{})
	rootText := extractText(t, rootResp)
	if !strings.Contains(rootText, "decisions/flat-files.txt") {
		t.Fatalf("step 5: root listing should show decisions/flat-files.txt, got: %s", rootText)
	}
	if !strings.Contains(rootText, "patterns/error-handling.txt") {
		t.Fatalf("step 5: root listing should show patterns/error-handling.txt, got: %s", rootText)
	}
	if !strings.Contains(rootText, "bugs/resolved/jwt-fix.txt") {
		t.Fatalf("step 5: root listing should show bugs/resolved/jwt-fix.txt, got: %s", rootText)
	}
	if !strings.Contains(rootText, "# Use Flat Files") {
		t.Fatalf("step 5: root listing should show first-line preview, got: %s", rootText)
	}

	// Step 5b: List root with recursive:false — see folder icons
	shallowResp := c.callTool(t, 50, "list_pages", map[string]any{"recursive": false})
	shallowText := extractText(t, shallowResp)
	if !strings.Contains(shallowText, "📁 decisions") {
		t.Fatalf("step 5b: shallow listing should show decisions folder, got: %s", shallowText)
	}
	if !strings.Contains(shallowText, "📁 patterns") {
		t.Fatalf("step 5b: shallow listing should show patterns folder, got: %s", shallowText)
	}

	// Step 6: List decisions subfolder — see files with full paths and previews
	decResp := c.callTool(t, 21, "list_pages", map[string]string{"path": "decisions"})
	decText := extractText(t, decResp)
	if !strings.Contains(decText, "decisions/flat-files.txt") {
		t.Fatalf("step 6: decisions listing should contain decisions/flat-files.txt, got: %s", decText)
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

func TestJourney_OptimisticConcurrency_SafetyBoundary(t *testing.T) {
	// Journey: Agent uses hash chain to prevent lost writes
	//
	// Business context: Two concurrent agents editing the same page must not
	// silently overwrite each other. The hash chain detects stale writes.
	//
	// Steps:
	//   1. Create a page, get its hash from put_page response
	//   2. Read the page — hash matches
	//   3. str_replace with correct hash — succeeds, returns new hash
	//   4. Try str_replace with the OLD (now stale) hash — rejected
	//   5. put_page with stale hash — rejected
	//   6. append_page with stale hash — rejected
	//   7. Operations without hash still work (opt-in safety)
	//
	// Expected: Stale hash always rejected, no hash always accepted.

	c := startServer(t)
	c.call(t, 1, "initialize", nil)

	// Step 1: Create page, capture hash
	putResp := c.callTool(t, 10, "put_page", map[string]string{
		"path":    "shared.txt",
		"content": "version 1\nline two\n",
	})
	hash1 := extractHash(putResp)
	if hash1 == "" {
		t.Fatal("step 1: put_page should return _meta.hash")
	}

	// Step 2: get_page hash matches
	getResp := c.callTool(t, 11, "get_page", map[string]string{"path": "shared.txt"})
	getHash := extractHash(getResp)
	if getHash != hash1 {
		t.Fatalf("step 2: get hash %q != put hash %q", getHash, hash1)
	}

	// Step 3: str_replace with correct hash
	replResp := c.callTool(t, 12, "str_replace_page", map[string]any{
		"path":          "shared.txt",
		"old_str":       "version 1",
		"new_str":       "version 2",
		"expected_hash": hash1,
	})
	if isError(replResp) {
		t.Fatalf("step 3: str_replace with correct hash should succeed: %s", extractText(t, replResp))
	}
	hash2 := extractHash(replResp)
	if hash2 == "" || hash2 == hash1 {
		t.Fatalf("step 3: expected new different hash, got %q (old was %q)", hash2, hash1)
	}

	// Step 4: str_replace with stale hash1 — should fail
	staleResp := c.callTool(t, 13, "str_replace_page", map[string]any{
		"path":          "shared.txt",
		"old_str":       "version 2",
		"new_str":       "version 3",
		"expected_hash": hash1,
	})
	if !isError(staleResp) {
		t.Fatal("step 4: str_replace with stale hash should fail")
	}
	if !strings.Contains(extractText(t, staleResp), "hash mismatch") {
		t.Errorf("step 4: expected 'hash mismatch', got: %s", extractText(t, staleResp))
	}

	// Step 5: put_page with stale hash — should fail
	stalePutResp := c.callTool(t, 14, "put_page", map[string]any{
		"path":          "shared.txt",
		"content":       "overwrite attempt",
		"expected_hash": hash1,
	})
	if !isError(stalePutResp) {
		t.Fatal("step 5: put_page with stale hash should fail")
	}

	// Step 6: append with stale hash — should fail
	staleAppendResp := c.callTool(t, 15, "append_page", map[string]any{
		"path":          "shared.txt",
		"content":       "\nextra line",
		"expected_hash": hash1,
	})
	if !isError(staleAppendResp) {
		t.Fatal("step 6: append_page with stale hash should fail")
	}

	// Step 7: put_page WITHOUT hash still works (opt-in)
	noHashResp := c.callTool(t, 16, "put_page", map[string]string{
		"path":    "shared.txt",
		"content": "version 3 (no hash check)",
	})
	if isError(noHashResp) {
		t.Fatalf("step 7: put_page without hash should succeed: %s", extractText(t, noHashResp))
	}
}

func TestJourney_IncrementalEditing_CoreUseCase(t *testing.T) {
	// Journey: Agent edits a page incrementally without full rewrites
	//
	// Business context: Agents should be able to append and surgically edit
	// pages rather than rewriting the entire content each time.
	//
	// Steps:
	//   1. Create initial page with put_page
	//   2. Append a section
	//   3. str_replace to fix a typo
	//   4. str_replace with empty new_str to delete a line
	//   5. Get final content — verify all edits applied
	//
	// Expected: Each edit is surgical, final content is correct.

	c := startServer(t)
	c.call(t, 1, "initialize", nil)

	// Step 1: Create
	c.callTool(t, 10, "put_page", map[string]string{
		"path":    "notes.txt",
		"content": "# Meeting Notes\n\nDecision: use flat files\n",
	})

	// Step 2: Append
	appendResp := c.callTool(t, 11, "append_page", map[string]string{
		"path":    "notes.txt",
		"content": "Action: write migration script\n",
	})
	if isError(appendResp) {
		t.Fatalf("step 2: append failed: %s", extractText(t, appendResp))
	}

	// Step 3: str_replace to fix content
	fixResp := c.callTool(t, 12, "str_replace_page", map[string]string{
		"path":    "notes.txt",
		"old_str": "use flat files",
		"new_str": "use flat .txt files",
	})
	if isError(fixResp) {
		t.Fatalf("step 3: str_replace failed: %s", extractText(t, fixResp))
	}
	fixText := extractText(t, fixResp)
	// Should show diff with - and + markers
	if !strings.Contains(fixText, "-") || !strings.Contains(fixText, "+") {
		t.Errorf("step 3: expected diff markers in response, got: %s", fixText)
	}

	// Step 4: Delete a line via empty new_str
	delResp := c.callTool(t, 13, "str_replace_page", map[string]string{
		"path":    "notes.txt",
		"old_str": "Action: write migration script\n",
		"new_str": "",
	})
	if isError(delResp) {
		t.Fatalf("step 4: delete via str_replace failed: %s", extractText(t, delResp))
	}
	delText := extractText(t, delResp)
	if !strings.Contains(delText, "deleted") {
		t.Errorf("step 4: expected 'deleted' in response, got: %s", delText)
	}

	// Step 5: Verify final content
	getResp := c.callTool(t, 14, "get_page", map[string]string{"path": "notes.txt"})
	finalText := extractText(t, getResp)
	if !strings.Contains(finalText, "use flat .txt files") {
		t.Errorf("step 5: fix not applied, got: %s", finalText)
	}
	if strings.Contains(finalText, "migration script") {
		t.Errorf("step 5: deleted line still present, got: %s", finalText)
	}
}

func TestJourney_ReorganizeMemory_CoreUseCase(t *testing.T) {
	// Journey: Agent restructures project memory as understanding evolves
	//
	// Business context: Early in a project, pages are flat. As structure emerges,
	// agents move pages into folders. This proves move + list + snapshot.
	//
	// Steps:
	//   1. Create several flat pages
	//   2. Move them into organized folders
	//   3. List root — see folder structure
	//   4. Snapshot a folder — see contents
	//   5. Snapshot nonexistent folder — get error
	//   6. Move returns hash for destination
	//
	// Expected: File organization works, snapshot reads the new structure.

	c := startServer(t)
	c.call(t, 1, "initialize", nil)

	// Step 1: Create flat pages
	c.callTool(t, 10, "put_page", map[string]string{
		"path": "db-choice.txt", "content": "Use flat files for storage.",
	})
	c.callTool(t, 11, "put_page", map[string]string{
		"path": "error-pattern.txt", "content": "Return errors, don't panic.",
	})
	c.callTool(t, 12, "put_page", map[string]string{
		"path": "jwt-fix.txt", "content": "Fixed JWT by switching to RS256.",
	})

	// Step 2: Move into folders
	moveResp := c.callTool(t, 13, "move_page", map[string]string{
		"path": "db-choice.txt", "new_path": "decisions/db-choice.txt",
	})
	if isError(moveResp) {
		t.Fatalf("step 2: move failed: %s", extractText(t, moveResp))
	}

	// Step 6 (checked inline): move returns hash
	moveHash := extractHash(moveResp)
	if moveHash == "" {
		t.Error("step 6: move_page should return _meta.hash for destination")
	}

	c.callTool(t, 14, "move_page", map[string]string{
		"path": "error-pattern.txt", "new_path": "patterns/error-pattern.txt",
	})
	c.callTool(t, 15, "move_page", map[string]string{
		"path": "jwt-fix.txt", "new_path": "bugs/jwt-fix.txt",
	})

	// Step 3: List root — default recursive, shows all files with full paths
	listResp := c.callTool(t, 16, "list_pages", map[string]string{})
	listText := extractText(t, listResp)
	if !strings.Contains(listText, "decisions/db-choice.txt") {
		t.Errorf("step 3: root should show decisions/db-choice.txt, got: %s", listText)
	}
	if !strings.Contains(listText, "patterns/error-pattern.txt") {
		t.Errorf("step 3: root should show patterns/error-pattern.txt, got: %s", listText)
	}
	if !strings.Contains(listText, "bugs/jwt-fix.txt") {
		t.Errorf("step 3: root should show bugs/jwt-fix.txt, got: %s", listText)
	}

	// Step 4: Snapshot a folder
	snapResp := c.callTool(t, 17, "snapshot", map[string]string{"path": "decisions"})
	snapText := extractText(t, snapResp)
	if !strings.Contains(snapText, "db-choice.txt") {
		t.Errorf("step 4: snapshot should contain db-choice.txt, got: %s", snapText)
	}
	if !strings.Contains(snapText, "Use flat files") {
		t.Errorf("step 4: snapshot should contain file content, got: %s", snapText)
	}

	// Step 5: Snapshot nonexistent folder
	badSnapResp := c.callTool(t, 18, "snapshot", map[string]string{"path": "nonexistent"})
	if !isError(badSnapResp) {
		t.Fatalf("step 5: snapshot of nonexistent folder should error, got: %s", extractText(t, badSnapResp))
	}
}

func TestJourney_CrossReferencesAndHistory_DiscoveryAudit(t *testing.T) {
	// Journey: Agent discovers relationships and audits changes via git
	//
	// Business context: Project memory pages reference each other. Agents need
	// to discover these links and trace how decisions evolved over time.
	//
	// Steps:
	//   1. Init git repo in work dir
	//   2. Create pages that reference each other
	//   3. Commit them
	//   4. Update a page, commit again
	//   5. related_pages — find cross-references
	//   6. page_history — see commit log
	//   7. page_history with include_diff — see patches
	//   8. page_history for nonexistent file — get error
	//
	// Expected: Cross-references discovered, history with diffs available.

	c := startServer(t)
	c.call(t, 1, "initialize", nil)

	// Step 1: Init git
	runGitE2E(t, c.workDir, "init")
	runGitE2E(t, c.workDir, "config", "user.email", "test@test.com")
	runGitE2E(t, c.workDir, "config", "user.name", "Test")

	// Step 2: Create cross-referencing pages
	c.callTool(t, 10, "put_page", map[string]string{
		"path":    "decisions/flat-files.txt",
		"content": "# Use Flat Files\n\nSee also: patterns/error-handling.txt\n",
	})
	c.callTool(t, 11, "put_page", map[string]string{
		"path":    "patterns/error-handling.txt",
		"content": "# Error Handling\n\nRelated decision: decisions/flat-files.txt\n",
	})

	// Step 3: Commit
	runGitE2E(t, c.workDir, "add", ".txtscape/")
	runGitE2E(t, c.workDir, "commit", "-m", "initial pages")

	// Step 4: Update and commit
	c.callTool(t, 12, "str_replace_page", map[string]string{
		"path":    "decisions/flat-files.txt",
		"old_str": "# Use Flat Files",
		"new_str": "# Use Flat .txt Files",
	})
	runGitE2E(t, c.workDir, "add", ".txtscape/")
	runGitE2E(t, c.workDir, "commit", "-m", "clarify title")

	// Step 5: related_pages
	relResp := c.callTool(t, 13, "related_pages", map[string]string{
		"path": "decisions/flat-files.txt",
	})
	if isError(relResp) {
		t.Fatalf("step 5: related_pages failed: %s", extractText(t, relResp))
	}
	relText := extractText(t, relResp)
	// Should find outgoing reference to error-handling.txt
	if !strings.Contains(relText, "error-handling.txt") {
		t.Errorf("step 5: should find outgoing ref to error-handling.txt, got: %s", relText)
	}
	// Should find incoming reference from error-handling.txt
	if !strings.Contains(relText, "←") {
		t.Errorf("step 5: should find incoming ref, got: %s", relText)
	}

	// Step 6: page_history
	histResp := c.callTool(t, 14, "page_history", map[string]string{
		"path": "decisions/flat-files.txt",
	})
	if isError(histResp) {
		t.Fatalf("step 6: page_history failed: %s", extractText(t, histResp))
	}
	histText := extractText(t, histResp)
	if !strings.Contains(histText, "initial pages") {
		t.Errorf("step 6: should contain first commit message, got: %s", histText)
	}
	if !strings.Contains(histText, "clarify title") {
		t.Errorf("step 6: should contain second commit message, got: %s", histText)
	}

	// Step 7: page_history with diff
	diffResp := c.callTool(t, 15, "page_history", map[string]any{
		"path":         "decisions/flat-files.txt",
		"include_diff": true,
	})
	if isError(diffResp) {
		t.Fatalf("step 7: page_history with diff failed: %s", extractText(t, diffResp))
	}
	diffText := extractText(t, diffResp)
	if !strings.Contains(diffText, "-# Use Flat Files") {
		t.Errorf("step 7: diff should show removed line, got: %s", diffText)
	}
	if !strings.Contains(diffText, "+# Use Flat .txt Files") {
		t.Errorf("step 7: diff should show added line, got: %s", diffText)
	}

	// Step 8: page_history for nonexistent file
	ghostResp := c.callTool(t, 16, "page_history", map[string]string{
		"path": "ghost.txt",
	})
	if !isError(ghostResp) {
		t.Fatalf("step 8: page_history for nonexistent file should error, got: %s", extractText(t, ghostResp))
	}
}
