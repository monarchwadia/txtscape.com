//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Journey: New user publishes their first page
//
// Business context: This is the core use case of txtscape. A new user should
// go from zero to a published, publicly accessible .txt page in one session.
//
// Steps:
//  1. Sign up with username + password → receive token
//  2. PUT /~username/hello.txt with token → 204
//  3. GET /~username/hello.txt (no auth) → 200, content matches
//  4. GET /~username (no auth) → directory listing includes hello.txt
func TestJourney_FirstPage_CoreUseCase_PublishAndBrowse(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	// 1. Sign up
	token := signup(t, srv, "alice", "password123")
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}

	// 2. Publish
	resp := putPage(t, srv, token, "/~alice/hello.txt", "# Hello from Alice\n\nThis is my first page.")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204", resp.StatusCode)
	}

	// 3. Read back (no auth)
	status, body := getPage(t, srv, "/~alice/hello.txt")
	if status != http.StatusOK {
		t.Fatalf("GET page status = %d, want 200", status)
	}
	if body != "# Hello from Alice\n\nThis is my first page." {
		t.Fatalf("body = %q", body)
	}

	// 4. Directory listing shows the file
	status, listing := getPage(t, srv, "/~alice")
	if status != http.StatusOK {
		t.Fatalf("GET listing status = %d, want 200", status)
	}
	if !strings.Contains(listing, "hello.txt") {
		t.Fatalf("listing missing hello.txt: %s", listing)
	}
	if !strings.Contains(listing, "# ~alice") {
		t.Fatalf("listing missing header: %s", listing)
	}
}

// Journey: User organizes content in nested folders
//
// Steps:
//  1. Signup → put files in /blog/ and /blog/2026/
//  2. Verify /~user/blog/ listing shows subfolder + files
//  3. Verify /~user/blog/2026/post.txt serves correctly
//  4. Verify root listing shows blog/ folder
func TestJourney_FolderOrganization_NestedContent_ListingsCorrect(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "bob", "password123")

	// Create files at various depths
	for _, p := range []struct{ path, content string }{
		{"/~bob/readme.txt", "# Bob's homepage"},
		{"/~bob/blog/intro.txt", "# Welcome to my blog"},
		{"/~bob/blog/2026/jan.txt", "# January update"},
		{"/~bob/blog/2026/feb.txt", "# February update"},
	} {
		resp := putPage(t, srv, token, p.path, p.content)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("PUT %s status = %d", p.path, resp.StatusCode)
		}
	}

	// Verify nested file serves
	status, body := getPage(t, srv, "/~bob/blog/2026/jan.txt")
	if status != 200 || body != "# January update" {
		t.Fatalf("nested file: status=%d body=%q", status, body)
	}

	// Root listing shows blog/ folder and readme.txt
	status, listing := getPage(t, srv, "/~bob")
	if status != 200 {
		t.Fatalf("root listing status = %d", status)
	}
	if !strings.Contains(listing, "blog/") {
		t.Fatalf("root listing missing blog/: %s", listing)
	}
	if !strings.Contains(listing, "readme.txt") {
		t.Fatalf("root listing missing readme.txt: %s", listing)
	}

	// Blog listing shows subfolder 2026/ and intro.txt
	status, listing = getPage(t, srv, "/~bob/blog/")
	if status != 200 {
		t.Fatalf("blog listing status = %d", status)
	}
	if !strings.Contains(listing, "2026/") {
		t.Fatalf("blog listing missing 2026/: %s", listing)
	}
	if !strings.Contains(listing, "intro.txt") {
		t.Fatalf("blog listing missing intro.txt: %s", listing)
	}
}

// Journey: Update existing page and verify change
//
// Steps:
//  1. Signup → put page → verify v1
//  2. Put same path with new content → verify v2
//  3. Original v1 is gone
func TestJourney_UpdateExisting_EditContent_NewVersionServed(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "carol", "password123")

	// v1
	resp := putPage(t, srv, token, "/~carol/draft.txt", "version 1")
	resp.Body.Close()
	status, body := getPage(t, srv, "/~carol/draft.txt")
	if status != 200 || body != "version 1" {
		t.Fatalf("v1: status=%d body=%q", status, body)
	}

	// v2
	resp = putPage(t, srv, token, "/~carol/draft.txt", "version 2 - updated")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT v2 status = %d", resp.StatusCode)
	}

	status, body = getPage(t, srv, "/~carol/draft.txt")
	if status != 200 || body != "version 2 - updated" {
		t.Fatalf("v2: status=%d body=%q", status, body)
	}
}

// Journey: Delete a page and verify cleanup
//
// Steps:
//  1. Signup → put page → verify accessible
//  2. Delete page → 204
//  3. GET returns 404
//  4. Directory listing no longer includes it
func TestJourney_DeletePage_RemoveContent_CleanupComplete(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "dave", "password123")

	// Create two pages
	putPage(t, srv, token, "/~dave/keep.txt", "staying").Body.Close()
	putPage(t, srv, token, "/~dave/remove.txt", "going away").Body.Close()

	// Verify both exist
	status, _ := getPage(t, srv, "/~dave/remove.txt")
	if status != 200 {
		t.Fatal("page should exist before delete")
	}

	// Delete one
	resp := deletePage(t, srv, token, "/~dave/remove.txt")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d", resp.StatusCode)
	}

	// Verify 404
	status, _ = getPage(t, srv, "/~dave/remove.txt")
	if status != 404 {
		t.Fatalf("GET after delete: status = %d, want 404", status)
	}

	// Listing should only show keep.txt
	_, listing := getPage(t, srv, "/~dave")
	if strings.Contains(listing, "remove.txt") {
		t.Fatalf("listing still contains remove.txt: %s", listing)
	}
	if !strings.Contains(listing, "keep.txt") {
		t.Fatalf("listing missing keep.txt: %s", listing)
	}
}

// Journey: Custom index.txt overrides directory listing
//
// Steps:
//  1. Signup → PUT index.txt → GET /~user serves index.txt content
//  2. Delete index.txt → GET /~user serves auto-generated listing again
func TestJourney_CustomIndex_OverrideListing_FallbackOnDelete(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "eve", "password123")

	// Put another file so listing has content
	putPage(t, srv, token, "/~eve/about.txt", "about me").Body.Close()

	// Without index.txt → auto listing
	status, listing := getPage(t, srv, "/~eve")
	if status != 200 {
		t.Fatalf("listing status = %d", status)
	}
	if !strings.Contains(listing, "about.txt") {
		t.Fatalf("auto listing missing about.txt: %s", listing)
	}

	// Put custom index.txt
	putPage(t, srv, token, "/~eve/index.txt", "# Welcome to Eve's space").Body.Close()

	// Now /~eve serves the custom index
	status, body := getPage(t, srv, "/~eve")
	if status != 200 || body != "# Welcome to Eve's space" {
		t.Fatalf("custom index: status=%d body=%q", status, body)
	}

	// Delete index.txt → falls back to auto listing
	deletePage(t, srv, token, "/~eve/index.txt").Body.Close()
	status, listing = getPage(t, srv, "/~eve")
	if status != 200 {
		t.Fatalf("post-delete listing status = %d", status)
	}
	if !strings.Contains(listing, "about.txt") {
		t.Fatalf("fallback listing missing about.txt: %s", listing)
	}
	if strings.Contains(listing, "# Welcome") {
		t.Fatalf("custom index still served after delete")
	}
}
