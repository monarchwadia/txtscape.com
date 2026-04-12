//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- UI E2E helpers ---

type uiClient struct {
	cmd     *exec.Cmd
	baseURL string
	workDir string
}

func startUIServer(t *testing.T) *uiClient {
	t.Helper()
	workDir := t.TempDir()
	pagesDir := filepath.Join(workDir, ".txtscape", "pages")
	os.MkdirAll(pagesDir, 0o755)

	cmd := exec.Command(binaryPath, "ui")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "PORT=0") // any available port

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	// Read the first line to get the URL
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("no output from ui server")
	}
	line := scanner.Text()
	// Expected: "txtscape ui: http://localhost:PORT"
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected output: %q", line)
	}
	baseURL := parts[1]

	// Wait for the server to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/config")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	return &uiClient{cmd: cmd, baseURL: baseURL, workDir: workDir}
}

func (c *uiClient) get(t *testing.T, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(c.baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func (c *uiClient) getJSON(t *testing.T, path string) (int, any) {
	t.Helper()
	status, body := c.get(t, path)
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		t.Fatalf("invalid JSON from %s: %v\nbody: %s", path, err, body)
	}
	return status, data
}

func (c *uiClient) writePage(t *testing.T, path, content string) {
	t.Helper()
	full := filepath.Join(c.workDir, ".txtscape", "pages", filepath.FromSlash(path))
	dir := filepath.Dir(full)
	os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writePage: %v", err)
	}
}

func (c *uiClient) writeConfig(t *testing.T, cfg string) {
	t.Helper()
	dir := filepath.Join(c.workDir, ".txtscape")
	os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

// --- UI E2E Journeys ---

func TestUIJourney_BrowsePages_CoreUseCase(t *testing.T) {
	// Journey: User starts the UI, creates pages on disk, browses them.
	//
	// 1. Start the UI server
	// 2. Verify empty state
	// 3. Create pages on disk
	// 4. Verify pages appear in listing
	// 5. Read a specific page
	// 6. Search for content
	// 7. Verify 404 for missing page

	c := startUIServer(t)

	// Step 1: Empty state — API returns empty
	status, data := c.getJSON(t, "/api/pages")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	arr, ok := data.([]any)
	if ok && len(arr) > 0 {
		t.Fatalf("expected empty pages, got %d", len(arr))
	}

	// Step 2: Config endpoint with no config
	status, body := c.get(t, "/api/config")
	if status != 200 {
		t.Fatalf("expected 200 for config, got %d", status)
	}
	if !strings.Contains(body, `"concerns"`) {
		t.Fatalf("expected concerns key in config, got %s", body)
	}

	// Step 3: Create some pages
	c.writePage(t, "hello.txt", "Hello from txtscape")
	c.writePage(t, "decisions/use-go.txt", "# We chose Go\nFor performance and simplicity.")
	c.writePage(t, "decisions/flat-files.txt", "# Flat files\nFilesystem is the database.")

	// Step 4: Verify page listing
	status, data = c.getJSON(t, "/api/pages")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	tree := data.([]any)
	if len(tree) != 2 {
		t.Fatalf("expected 2 root entries (decisions/ + hello.txt), got %d", len(tree))
	}

	// Step 5: Read a specific page
	status, data = c.getJSON(t, "/api/pages/decisions/use-go.txt")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	page := data.(map[string]any)
	if page["content"] != "# We chose Go\nFor performance and simplicity." {
		t.Fatalf("unexpected content: %v", page["content"])
	}

	// Step 6: Search
	status, data = c.getJSON(t, "/api/search?q=performance")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	results := data.([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	hit := results[0].(map[string]any)
	if hit["path"] != "decisions/use-go.txt" {
		t.Fatalf("expected decisions/use-go.txt, got %v", hit["path"])
	}

	// Step 7: 404 for missing page
	status, _ = c.get(t, "/api/pages/missing.txt")
	if status != 404 {
		t.Fatalf("expected 404 for missing page, got %d", status)
	}
}

func TestUIJourney_StaticFileServing_SPA(t *testing.T) {
	// Journey: Verify static files and SPA fallback work via real HTTP.
	//
	// 1. index.html served at /
	// 2. CSS and JS served
	// 3. Unknown paths get SPA fallback

	c := startUIServer(t)

	// HTML at root
	status, body := c.get(t, "/")
	if status != 200 {
		t.Fatalf("expected 200 for /, got %d", status)
	}
	if !strings.Contains(body, "<title>txtscape</title>") {
		t.Fatal("expected index.html content")
	}

	// CSS
	status, body = c.get(t, "/style.css")
	if status != 200 {
		t.Fatalf("expected 200 for /style.css, got %d", status)
	}
	if !strings.Contains(body, "--bg:") {
		t.Fatal("expected CSS content")
	}

	// JS
	status, body = c.get(t, "/app.js")
	if status != 200 {
		t.Fatalf("expected 200 for /app.js, got %d", status)
	}
	if !strings.Contains(body, "function") {
		t.Fatal("expected JS content")
	}

	// SPA fallback
	status, body = c.get(t, "/explore/decisions")
	if status != 200 {
		t.Fatalf("expected 200 for SPA fallback, got %d", status)
	}
	if !strings.Contains(body, "<title>txtscape</title>") {
		t.Fatal("expected SPA fallback to serve index.html")
	}
}

func TestUIJourney_PathTraversalSecurity_SecurityBoundary(t *testing.T) {
	// Journey: Verify path traversal attacks are blocked via real HTTP.
	//
	// 1. Encoded path traversal
	// 2. Invalid file types

	c := startUIServer(t)

	// Path traversal — Go's HTTP server cleans most of these,
	// but our validatePath catches remaining cases
	status, _ := c.get(t, "/api/pages/foo/..%2f..%2fetc/passwd.txt")
	if status != 400 {
		t.Fatalf("expected 400 for path traversal, got %d", status)
	}

	// Non-.txt file
	status, _ = c.get(t, "/api/pages/evil.exe")
	if status != 400 {
		t.Fatalf("expected 400 for non-.txt, got %d", status)
	}
}

func TestUIJourney_ConfigReloading_LiveUpdate(t *testing.T) {
	// Journey: Verify config changes are reflected without restart.
	//
	// 1. Start with no config → empty concerns
	// 2. Write config
	// 3. Re-read config → new concerns appear

	c := startUIServer(t)

	// No config initially
	status, data := c.getJSON(t, "/api/config")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	cfg := data.(map[string]any)
	concerns := cfg["concerns"].([]any)
	if len(concerns) != 0 {
		t.Fatalf("expected empty concerns, got %d", len(concerns))
	}

	// Write config
	c.writeConfig(t, `{"concerns":[{"folderName":"decisions","label":"Decisions","description":"Tech choices"}]}`)

	// Read again — should see new config without restart
	status, data = c.getJSON(t, "/api/config")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	cfg = data.(map[string]any)
	concerns = cfg["concerns"].([]any)
	if len(concerns) != 1 {
		t.Fatalf("expected 1 concern after config update, got %d", len(concerns))
	}
	c0 := concerns[0].(map[string]any)
	if c0["folderName"] != "decisions" {
		t.Fatalf("expected 'decisions', got %v", c0["folderName"])
	}
}

func TestUIJourney_SSE_ReceivesChanges(t *testing.T) {
	// Journey: Verify SSE stream sends events when pages change.
	//
	// 1. Connect to SSE
	// 2. Wait for initial snapshot to be captured by server
	// 3. Create a page
	// 4. Receive change event

	c := startUIServer(t)

	// Connect to SSE
	resp, err := http.Get(c.baseURL + "/api/events")
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	// Wait for the server to take its initial filesystem snapshot.
	// The SSE handler polls every 1s, so waiting 1.5s ensures the
	// initial snapshot is captured before we create the file.
	time.Sleep(1500 * time.Millisecond)

	// Now create a page — next poll cycle will detect the change
	c.writePage(t, "sse-test.txt", "SSE test page")

	// Read from SSE — should receive an event within 3 seconds
	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				done <- line
				return
			}
		}
	}()

	select {
	case line := <-done:
		if !strings.Contains(line, `"pages"`) {
			t.Fatalf("expected pages change event, got %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestUIJourney_SearchEdgeCases_Robustness(t *testing.T) {
	// Journey: Search handles edge cases correctly.
	//
	// 1. Empty query → 400
	// 2. Special characters in query
	// 3. Search across multiple files

	c := startUIServer(t)

	// Empty query
	status, _ := c.get(t, "/api/search?q=")
	if status != 400 {
		t.Fatalf("expected 400 for empty query, got %d", status)
	}

	// Create pages for search
	c.writePage(t, "a.txt", "function hello() { return 42; }")
	c.writePage(t, "b.txt", "SELECT * FROM users WHERE id = 1")
	c.writePage(t, "c.txt", "nothing relevant here")

	// Special chars in query (should work as plain text, not regex)
	status, data := c.getJSON(t, "/api/search?q="+fmt.Sprintf("%s", "hello()"))
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	results := data.([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'hello()', got %d", len(results))
	}

	// Search across files
	status, data = c.getJSON(t, "/api/search?q=return")
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	results = data.([]any)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result for 'return'")
	}
}
