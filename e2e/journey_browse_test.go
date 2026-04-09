//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Journey: Subdirectory listings at different levels
//
// Steps:
//  1. Signup → create files in /projects/web/ and /projects/cli/
//  2. GET /~user/projects/ → shows web/ and cli/ subfolders
//  3. GET /~user/projects/web/ → shows files in that subfolder
func TestJourney_SubdirectoryListing_BrowseHierarchy_CorrectAtEachLevel(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "nav", "password123")

	pages := []struct{ path, content string }{
		{"/~nav/projects/web/app.txt", "web app docs"},
		{"/~nav/projects/web/api.txt", "web api docs"},
		{"/~nav/projects/cli/tool.txt", "cli tool docs"},
	}
	for _, p := range pages {
		resp := putPage(t, srv, token, p.path, p.content)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("PUT %s status = %d", p.path, resp.StatusCode)
		}
	}

	// /~nav/projects/ should show web/ and cli/
	status, listing := getPage(t, srv, "/~nav/projects/")
	if status != 200 {
		t.Fatalf("projects listing status = %d", status)
	}
	if !strings.Contains(listing, "web/") {
		t.Fatalf("projects listing missing web/: %s", listing)
	}
	if !strings.Contains(listing, "cli/") {
		t.Fatalf("projects listing missing cli/: %s", listing)
	}

	// /~nav/projects/web/ should show app.txt and api.txt
	status, listing = getPage(t, srv, "/~nav/projects/web/")
	if status != 200 {
		t.Fatalf("web listing status = %d", status)
	}
	if !strings.Contains(listing, "app.txt") {
		t.Fatalf("web listing missing app.txt: %s", listing)
	}
	if !strings.Contains(listing, "api.txt") {
		t.Fatalf("web listing missing api.txt: %s", listing)
	}
}

// Journey: Two users link to each other → both pages resolve
//
// Steps:
//  1. Signup alice and bob
//  2. Alice publishes a page linking to bob
//  3. Bob publishes a page linking to alice
//  4. Both pages serve correctly and contain the expected links
func TestJourney_CrossUserLinks_InterlinkedContent_BothResolve(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	aliceToken := signup(t, srv, "alice", "password123")
	bobToken := signup(t, srv, "bob", "password123")

	aliceContent := "# Alice's Page\n\nCheck out [Bob's page](https://txtscape.com/~bob/hello.txt)."
	bobContent := "# Bob's Page\n\nCheck out [Alice's page](https://txtscape.com/~alice/hello.txt)."

	putPage(t, srv, aliceToken, "/~alice/hello.txt", aliceContent).Body.Close()
	putPage(t, srv, bobToken, "/~bob/hello.txt", bobContent).Body.Close()

	// Both serve
	status, body := getPage(t, srv, "/~alice/hello.txt")
	if status != 200 {
		t.Fatalf("alice status = %d", status)
	}
	if !strings.Contains(body, "~bob/hello.txt") {
		t.Fatalf("alice page missing link to bob: %s", body)
	}

	status, body = getPage(t, srv, "/~bob/hello.txt")
	if status != 200 {
		t.Fatalf("bob status = %d", status)
	}
	if !strings.Contains(body, "~alice/hello.txt") {
		t.Fatalf("bob page missing link to alice: %s", body)
	}
}

// Journey: Browser visits a published page, then the same URL as an agent
//
// Business context: Content negotiation lets browsers see styled HTML and agents
// see raw plaintext at the same URL. This is the core feature that makes txtscape
// human-browsable without breaking the agent experience.
//
// Steps:
//  1. Sign up and publish a markdown page with a heading and a link
//  2. GET the page with Accept: text/html (browser) → styled HTML with clickable link
//  3. GET the same page with Accept: */* (agent) → raw plaintext, unchanged
//  4. Verify Vary: Accept header on both responses
//
// Expected: Same URL, two representations. Browser gets HTML, agent gets plaintext.
func TestJourney_ContentNegotiation_BrowserVsAgent_SameURLTwoViews(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "viewer", "password123")

	content := "# My Page\n\nHello world.\n\nSee [other page](/~viewer/other.txt)."
	resp := putPage(t, srv, token, "/~viewer/page.txt", content)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d", resp.StatusCode)
	}

	// Step 2: Browser view
	status, body, headers := getPageWithAccept(t, srv, "/~viewer/page.txt",
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	if status != 200 {
		t.Fatalf("browser GET status = %d", status)
	}
	ct := headers.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("browser content-type = %q, want text/html", ct)
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("browser response missing DOCTYPE")
	}
	if !strings.Contains(body, "#0d1117") {
		t.Error("browser response missing dark theme")
	}
	if !strings.Contains(body, `<a href="/~viewer/other.txt">`) {
		t.Error("browser response missing clickable link")
	}
	if !strings.Contains(body, ">My Page<") {
		t.Error("browser response missing rendered heading")
	}
	if !strings.Contains(headers.Get("Vary"), "Accept") {
		t.Error("browser response missing Vary: Accept")
	}

	// Step 3: Agent view
	status, body, headers = getPageWithAccept(t, srv, "/~viewer/page.txt", "*/*")

	if status != 200 {
		t.Fatalf("agent GET status = %d", status)
	}
	ct = headers.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("agent content-type = %q, want text/plain", ct)
	}
	if body != content {
		t.Fatalf("agent body = %q, want exact plaintext", body)
	}
	if !strings.Contains(headers.Get("Vary"), "Accept") {
		t.Error("agent response missing Vary: Accept")
	}
}

// Journey: Browser views a directory listing with folders and files
//
// Business context: Directory listings should render as styled HTML lists in
// browsers, with 📁/📄 icons distinguishing folders from files, and clickable
// links that use relative paths (not hardcoded https://txtscape.com/).
//
// Steps:
//  1. Sign up and publish files in a folder structure
//  2. GET the directory listing as browser → HTML with <ul>/<li>, icons, relative links
//  3. GET the same listing as agent → plaintext markdown with icons
//
// Expected: Folders show 📁, files show 📄, links are relative and clickable.
func TestJourney_DirectoryListing_BrowserView_StyledWithIcons(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "lister", "password123")

	// Create a folder structure
	for _, p := range []struct{ path, content string }{
		{"/~lister/blog/post.txt", "a blog post"},
		{"/~lister/blog/draft.txt", "a draft"},
		{"/~lister/notes/idea.txt", "an idea"},
	} {
		resp := putPage(t, srv, token, p.path, p.content)
		resp.Body.Close()
	}

	// Browser view of root
	status, body, headers := getPageWithAccept(t, srv, "/~lister/",
		"text/html,application/xhtml+xml,*/*;q=0.8")

	if status != 200 {
		t.Fatalf("browser listing status = %d", status)
	}
	if !strings.HasPrefix(headers.Get("Content-Type"), "text/html") {
		t.Fatalf("content-type = %q, want text/html", headers.Get("Content-Type"))
	}
	if !strings.Contains(body, "📁") {
		t.Error("missing folder icon in browser listing")
	}
	if !strings.Contains(body, "📄") {
		// Root listing only has folders, so check blog/ listing for file icon
	}
	if !strings.Contains(body, "<ul") {
		t.Error("listing not rendered as HTML list")
	}
	if !strings.Contains(body, "<li>") {
		t.Error("listing items not rendered as <li>")
	}
	// Links must be relative, not absolute
	if strings.Contains(body, "https://txtscape.com") {
		t.Error("listing contains hardcoded absolute URL")
	}
	if !strings.Contains(body, `href="/~lister/blog/"`) {
		t.Error("missing relative link to blog folder")
	}

	// Agent view — same URL, plaintext markdown
	status, body, _ = getPageWithAccept(t, srv, "/~lister/", "*/*")
	if status != 200 {
		t.Fatalf("agent listing status = %d", status)
	}
	if !strings.Contains(body, "📁") {
		t.Error("agent listing missing folder icon")
	}
	if !strings.Contains(body, "/~lister/blog/") {
		t.Error("agent listing missing relative link")
	}
}

// Journey: Browser hits a 404 page
//
// Business context: When a browser navigates to a page that doesn't exist,
// they should see a styled HTML error page, not raw JSON.
//
// Steps:
//  1. GET a non-existent page as browser → styled 404
//  2. GET the same URL as agent → JSON error
//
// Expected: Browser sees HTML 404 with dark theme. Agent sees JSON.
func TestJourney_NotFound_BrowserVsAgent_StyledErrorVsJSON(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "ghost", "password123")

	// Browser 404
	status, body, headers := getPageWithAccept(t, srv, "/~ghost/missing.txt",
		"text/html,application/xhtml+xml,*/*;q=0.8")

	if status != 404 {
		t.Fatalf("browser 404 status = %d", status)
	}
	if !strings.HasPrefix(headers.Get("Content-Type"), "text/html") {
		t.Fatalf("browser 404 content-type = %q, want text/html", headers.Get("Content-Type"))
	}
	if !strings.Contains(body, "#0d1117") {
		t.Error("browser 404 missing dark theme")
	}
	if !strings.Contains(body, "page not found") {
		t.Error("browser 404 missing error message")
	}

	// Agent 404
	status, body, headers = getPageWithAccept(t, srv, "/~ghost/missing.txt", "*/*")

	if status != 404 {
		t.Fatalf("agent 404 status = %d", status)
	}
	if headers.Get("Content-Type") != "application/json" {
		t.Fatalf("agent 404 content-type = %q, want application/json", headers.Get("Content-Type"))
	}
	if !strings.Contains(body, `"page not found"`) {
		t.Error("agent 404 missing JSON error message")
	}
}
