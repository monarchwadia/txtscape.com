//go:build integration

package integration

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

// Integration tests verify the MCP server works end-to-end through stdio,
// as a real MCP client would use it. The binary is compiled, started as a
// subprocess, and communicated with via JSON-RPC over stdin/stdout.

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests.
	tmpDir, err := os.MkdirTemp("", "txtscape-mcp-integ-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "txtscape-mcp")

	// Find the module root (two levels up from tests/integration/)
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	moduleRoot := filepath.Dir(filepath.Dir(wd))

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = moduleRoot
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

type mcpClient struct {
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	cmd    *exec.Cmd
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

	return &mcpClient{stdin: stdin, stdout: scanner, cmd: cmd}
}

func (c *mcpClient) send(t *testing.T, req map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		t.Fatal(err)
	}

	if !c.stdout.Scan() {
		t.Fatal("no response from server")
	}

	var resp map[string]any
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\nraw: %s", err, c.stdout.Text())
	}
	return resp
}

func TestStdio_Initialize_ReturnsServerInfo(t *testing.T) {
	// Business context: Real MCP clients connect via stdio and send initialize
	// as their first message. This test proves the full binary works end-to-end
	// over the actual transport (stdin/stdout), not just unit-level function calls.
	// Scenario: Start the binary, send initialize via stdin, read response from stdout.
	// Expected: Response contains serverInfo.name = "txtscape".
	c := startServer(t)

	resp := c.send(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in response: %v", resp)
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("no serverInfo in result: %v", result)
	}
	if info["name"] != "txtscape" {
		t.Errorf("name = %v, want txtscape", info["name"])
	}
}

func TestStdio_PutThenGet_RoundTrip(t *testing.T) {
	// Business context: The most fundamental operation: write a page via MCP, then
	// read it back. This proves the binary correctly persists files and reads them.
	// Scenario: Put a page, then get it, all via stdio JSON-RPC.
	// Expected: Content matches.
	c := startServer(t)

	// Initialize first
	c.send(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	// Put
	putArgs, _ := json.Marshal(map[string]string{
		"path":    "hello.txt",
		"content": "# Hello World",
	})
	putResp := c.send(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "put_page",
			"arguments": json.RawMessage(putArgs),
		},
	})

	putResult, _ := putResp["result"].(map[string]any)
	if isErr, _ := putResult["isError"].(bool); isErr {
		t.Fatalf("put_page error: %v", putResult)
	}

	// Get
	getArgs, _ := json.Marshal(map[string]string{
		"path": "hello.txt",
	})
	getResp := c.send(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_page",
			"arguments": json.RawMessage(getArgs),
		},
	})

	getResult, _ := getResp["result"].(map[string]any)
	content, _ := getResult["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content in response: %v", getResp)
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if text != "# Hello World" {
		t.Errorf("content = %q, want %q", text, "# Hello World")
	}
}

func TestStdio_SearchAcrossFiles_FindsMatches(t *testing.T) {
	// Business context: Search is the killer feature for LLM ergonomics.
	// This test proves search works through the full stdio transport.
	// Scenario: Put two files, search for a term that appears in one.
	// Expected: Results include the matching file, not the other.
	c := startServer(t)

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})

	// Put two files
	for i, page := range []struct{ path, content string }{
		{"decisions/db.txt", "# Database\n\nWe chose flat files over PostgreSQL."},
		{"patterns/err.txt", "# Errors\n\nReturn errors, don't panic."},
	} {
		args, _ := json.Marshal(map[string]string{"path": page.path, "content": page.content})
		c.send(t, map[string]any{
			"jsonrpc": "2.0",
			"id":      i + 10,
			"method":  "tools/call",
			"params":  map[string]any{"name": "put_page", "arguments": json.RawMessage(args)},
		})
	}

	// Search
	searchArgs, _ := json.Marshal(map[string]string{"query": "PostgreSQL"})
	searchResp := c.send(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      20,
		"method":  "tools/call",
		"params":  map[string]any{"name": "search_pages", "arguments": json.RawMessage(searchArgs)},
	})

	searchResult, _ := searchResp["result"].(map[string]any)
	content, _ := searchResult["content"].([]any)
	if len(content) == 0 {
		t.Fatal("no content in search response")
	}
	text, _ := content[0].(map[string]any)["text"].(string)

	if !strings.Contains(text, "db.txt") {
		t.Error("search should find db.txt")
	}
	if strings.Contains(text, "err.txt") {
		t.Error("search should NOT find err.txt")
	}
}
