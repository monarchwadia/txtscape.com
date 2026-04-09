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
