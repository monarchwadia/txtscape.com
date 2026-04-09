//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Journey: File too large is rejected
//
// Steps:
//  1. Signup → PUT a 100KB+1 body → 413
//  2. Verify page was NOT created
//  3. PUT exactly 100KB → 204 (succeeds)
func TestJourney_FileTooLarge_EnforceLimit_Rejected(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "limit-a", "password123")

	// 100KB + 1 byte
	oversized := strings.Repeat("x", 102401)
	resp := putPage(t, srv, token, "/~limit-a/big.txt", oversized)
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized PUT status = %d, want 413", resp.StatusCode)
	}

	// Not created
	status, _ := getPage(t, srv, "/~limit-a/big.txt")
	if status != 404 {
		t.Fatalf("oversized file should not exist, status = %d", status)
	}

	// Exactly 100KB succeeds
	exact := strings.Repeat("y", 102400)
	resp = putPage(t, srv, token, "/~limit-a/exact.txt", exact)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("100KB PUT status = %d, want 204", resp.StatusCode)
	}
}

// Journey: Folder depth limit — 10 levels OK, 11 rejected
//
// Steps:
//  1. Signup → PUT file at depth 9 folders → 204
//  2. PUT file at depth 10 folders → 400 (exceeds max)
func TestJourney_FolderDepth_EnforceNesting_TenOKElevenRejected(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "limit-b", "password123")

	// Depth 9 (9 folder parts + filename = 10 path segments, within limit)
	depth9 := "a/b/c/d/e/f/g/h/i/file.txt"
	resp := putPage(t, srv, token, "/~limit-b/"+depth9, "deep content")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("depth 9 PUT status = %d, want 204", resp.StatusCode)
	}

	// Depth 10 (10 folder parts → exceeds max)
	depth10 := "a/b/c/d/e/f/g/h/i/j/file.txt"
	resp = putPage(t, srv, token, "/~limit-b/"+depth10, "too deep")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("depth 10 PUT status = %d, want 400", resp.StatusCode)
	}
}

// Journey: Max 100 files per folder, 101st rejected
//
// Steps:
//  1. Signup → PUT 100 files in root → all 204
//  2. PUT file 101 → 409
func TestJourney_FilesPerFolder_EnforceHundred_ExcessRejected(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "limit-c", "password123")

	// Create 100 files
	for i := 1; i <= 100; i++ {
		name := fmt.Sprintf("file-%03d.txt", i)
		resp := putPage(t, srv, token, "/~limit-c/"+name, fmt.Sprintf("content %d", i))
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("file %d PUT status = %d, want 204", i, resp.StatusCode)
		}
	}

	// 101st should fail
	resp := putPage(t, srv, token, "/~limit-c/overflow.txt", "one too many")
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("101st file PUT status = %d, want 409", resp.StatusCode)
	}
}

// Journey: Max 10 subfolders per folder, 11th rejected
//
// Steps:
//  1. Signup → create files in 10 different subfolders → all 204
//  2. Create file in 11th subfolder → 409
func TestJourney_SubfoldersPerFolder_EnforceTen_ExcessRejected(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "limit-d", "password123")

	// Create files in 10 subfolders
	for i := 1; i <= 10; i++ {
		path := fmt.Sprintf("/~limit-d/dir%d/file.txt", i)
		resp := putPage(t, srv, token, path, fmt.Sprintf("folder %d", i))
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("subfolder %d PUT status = %d, want 204", i, resp.StatusCode)
		}
	}

	// 11th subfolder should fail
	resp := putPage(t, srv, token, "/~limit-d/dir11/file.txt", "one too many")
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("11th subfolder PUT status = %d, want 409", resp.StatusCode)
	}
}
