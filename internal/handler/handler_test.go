package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrefersHTML_BrowserAccept_ContentNegotiation_ReturnsTrue(t *testing.T) {
	// Business context: Browsers send Accept headers containing text/html.
	// When a browser visits a .txt page, we want to render a styled HTML view
	// so the content is readable. Agents get raw plaintext.
	// Scenario: Request with a typical browser Accept header.
	// Expected: Returns true — browser wants HTML.

	tests := []struct {
		name   string
		accept string
		want   bool
		reason string
	}{
		{
			name:   "chrome default",
			accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			want:   true,
			reason: "Chrome sends text/html first — serve HTML",
		},
		{
			name:   "curl default",
			accept: "*/*",
			want:   false,
			reason: "curl sends */* — treat as agent, serve plaintext",
		},
		{
			name:   "no accept header",
			accept: "",
			want:   false,
			reason: "missing Accept — treat as agent, serve plaintext",
		},
		{
			name:   "explicit text/plain",
			accept: "text/plain",
			want:   false,
			reason: "client explicitly wants plaintext",
		},
		{
			name:   "agent with json",
			accept: "application/json",
			want:   false,
			reason: "API client — serve plaintext",
		},
		{
			name:   "firefox default",
			accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			want:   true,
			reason: "Firefox sends text/html — serve HTML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			got := prefersHTML(req)
			if got != tt.want {
				t.Errorf("prefersHTML(%q) = %v, want %v — %s", tt.accept, got, tt.want, tt.reason)
			}
		})
	}
}

func TestRenderHTML_BasicMarkdown_BrowserView_RendersStyledPage(t *testing.T) {
	// Business context: When a browser visits a .txt page, we render it as styled
	// HTML so humans can read it comfortably. The markdown content must be converted
	// to HTML with clickable links and styled headings, wrapped in the site's dark theme.
	// Scenario: Render a markdown string with headings, links, and plain text.
	// Expected: Output is valid HTML with the dark theme, headings become styled,
	// markdown links become clickable <a> tags, and plain text is preserved.

	input := "# My Page\n\nHello world.\n\n- [home](/~alice)\n- [blog](/~alice/blog/)\n\n## Section Two\n\nSome text with [a link](/~bob/hello.txt) inline."
	result := renderHTML("~alice/page.txt", input)

	// Must be valid HTML structure
	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(result, "<html") {
		t.Error("missing <html>")
	}
	if !strings.Contains(result, "</html>") {
		t.Error("missing </html>")
	}

	// Must have the dark theme background
	if !strings.Contains(result, "#0d1117") {
		t.Error("missing dark theme background color")
	}

	// Title should appear in <title> tag
	if !strings.Contains(result, "<title>~alice/page.txt") {
		t.Error("missing title tag with page path")
	}

	// Markdown links must become clickable <a> tags
	if !strings.Contains(result, `<a href="/~alice">home</a>`) {
		t.Error("missing rendered link for [home](/~alice)")
	}
	if !strings.Contains(result, `<a href="/~bob/hello.txt">a link</a>`) {
		t.Error("missing rendered link for [a link](/~bob/hello.txt)")
	}

	// Content-Type meta tag
	if !strings.Contains(result, `charset`) {
		t.Error("missing charset declaration")
	}
}

func TestRenderHTML_HTMLEscaping_PreventXSS_EscapesSpecialCharacters(t *testing.T) {
	// Business context: User-generated content may contain HTML special characters.
	// We must escape them to prevent XSS attacks. A malicious user could put
	// <script> tags in their .txt files attempting to execute JS in browser viewers.
	// Scenario: Content contains HTML tags and script injection attempts.
	// Expected: All HTML special characters are escaped, no raw <script> in output.

	input := `# Safe Page

Hello <script>alert("xss")</script> world.

Check [this](/~alice) & "that".`

	result := renderHTML("test.txt", input)

	if strings.Contains(result, "<script>") {
		t.Fatal("XSS: raw <script> tag found in output")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Error("HTML special characters not escaped")
	}
	if !strings.Contains(result, "&amp;") {
		t.Error("ampersand not escaped")
	}
}

func TestRenderHTML_HeadingRendering_VisualHierarchy_HeadingsStyled(t *testing.T) {
	// Business context: Markdown headings (# and ##) provide visual structure.
	// In the HTML view, they should be rendered distinctly from body text so
	// humans can scan the page structure.
	// Scenario: Content with h1 and h2 headings.
	// Expected: Headings are wrapped in heading tags or styled elements.

	input := "# Title\n\nBody text.\n\n## Subtitle\n\nMore text.\n\n### Third Level"
	result := renderHTML("test.txt", input)

	// Headings should be rendered as HTML heading elements
	if !strings.Contains(result, ">Title</") {
		t.Error("h1 heading not rendered")
	}
	if !strings.Contains(result, ">Subtitle</") {
		t.Error("h2 heading not rendered")
	}
	if !strings.Contains(result, ">Third Level</") {
		t.Error("h3 heading not rendered")
	}
}

func TestRenderHTML_ListItems_DirectoryListing_RendersAsHTMLList(t *testing.T) {
	// Business context: Directory listings use markdown list items (- [name](url)).
	// In the HTML view, these should render as a proper <ul>/<li> list, not inside
	// a <pre> block, so they look like a clean file browser.
	// Scenario: A typical directory listing with folders and files.
	// Expected: List items render as <li> inside <ul>, with clickable links.

	input := "# ~alice\n\n- 📁 [blog/](/~alice/blog/)\n- 📄 [hello.txt](/~alice/hello.txt)"
	result := renderHTML("~alice/", input)

	if !strings.Contains(result, "<ul") {
		t.Error("missing <ul> — list items should render as HTML list")
	}
	if !strings.Contains(result, "<li>") {
		t.Error("missing <li> — each list item should be a <li>")
	}
	if !strings.Contains(result, `<a href="/~alice/blog/">`) {
		t.Error("missing folder link")
	}
	if !strings.Contains(result, `<a href="/~alice/hello.txt">`) {
		t.Error("missing file link")
	}
	if !strings.Contains(result, "📁") {
		t.Error("missing folder icon")
	}
	if !strings.Contains(result, "📄") {
		t.Error("missing file icon")
	}
	// Should NOT be inside a <pre> block
	if strings.Contains(result, "<pre") && strings.Contains(result, "📁") {
		// Check that the list items are not wrapped in <pre>
		preIdx := strings.Index(result, "<pre")
		iconIdx := strings.Index(result, "📁")
		if preIdx < iconIdx {
			preEnd := strings.Index(result[preIdx:], "</pre>")
			if preEnd == -1 || preIdx+preEnd > iconIdx {
				t.Error("list items should not be inside <pre> block")
			}
		}
	}
}

func TestExtractBearer_Valid_ParseHeader_ReturnsToken(t *testing.T) {
	token := extractBearer("Bearer abc123")
	if token != "abc123" {
		t.Fatalf("got %q, want %q", token, "abc123")
	}
}

func TestExtractBearer_Empty_MissingAuth_ReturnsEmpty(t *testing.T) {
	token := extractBearer("")
	if token != "" {
		t.Fatalf("got %q, want empty", token)
	}
}

func TestExtractBearer_NoBearerPrefix_MalformedAuth_ReturnsEmpty(t *testing.T) {
	token := extractBearer("Basic abc123")
	if token != "" {
		t.Fatalf("got %q, want empty", token)
	}
}

func TestExtractBearer_CaseInsensitive_FlexibleParsing_ReturnsToken(t *testing.T) {
	token := extractBearer("bearer abc123")
	if token != "abc123" {
		t.Fatalf("got %q, want %q", token, "abc123")
	}
}

func TestWriteError_BadRequest_ClientError_Returns400JSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}

	var resp jsonError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Error != "test error" {
		t.Fatalf("error = %q, want %q", resp.Error, "test error")
	}
}

func TestServeError_BrowserAccept_ContentNegotiation_ReturnsStyledHTML(t *testing.T) {
	// Business context: When a browser hits a 404 or other error on a page URL,
	// it should see a styled HTML error page matching the site theme, not raw JSON.
	// Scenario: GET request with browser Accept header hits a missing page.
	// Expected: Returns text/html with styled error page containing the error message.

	req := httptest.NewRequest("GET", "/~alice/missing.txt", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	w := httptest.NewRecorder()

	serveError(w, req, http.StatusNotFound, "page not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html for browser", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "#0d1117") {
		t.Error("missing dark theme in error page")
	}
	if !strings.Contains(body, "page not found") {
		t.Error("missing error message in HTML output")
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE in error page")
	}
}

func TestServeError_AgentAccept_ContentNegotiation_ReturnsJSON(t *testing.T) {
	// Business context: Agents and API clients should continue to receive JSON
	// error responses so they can parse error messages programmatically.
	// Scenario: GET request with */* Accept header hits a missing page.
	// Expected: Returns application/json with the error.

	req := httptest.NewRequest("GET", "/~alice/missing.txt", nil)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	serveError(w, req, http.StatusNotFound, "page not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json for agent", ct)
	}
	var resp jsonError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Error != "page not found" {
		t.Fatalf("error = %q, want %q", resp.Error, "page not found")
	}
}

func TestWriteJSON_Created_SuccessResponse_Returns201(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, jsonToken{Token: "tok123"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp jsonToken
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Token != "tok123" {
		t.Fatalf("token = %q, want %q", resp.Token, "tok123")
	}
}

func TestParseTildePath_Variations(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantUser    string
		wantRawPath string
	}{
		{"root user", "/~alice", "alice", ""},
		{"root user trailing slash", "/~alice/", "alice", ""},
		{"file at root", "/~alice/hello.txt", "alice", "hello.txt"},
		{"nested file", "/~alice/blog/post.txt", "alice", "blog/post.txt"},
		{"deep path", "/~alice/a/b/c/d.txt", "alice", "a/b/c/d.txt"},
		{"folder listing", "/~alice/blog/", "alice", "blog/"},
		{"empty", "/~", "", ""},
		{"no tilde", "/alice/hello.txt", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, rawPath := parseTildePath(tt.path)
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if rawPath != tt.wantRawPath {
				t.Errorf("rawPath = %q, want %q", rawPath, tt.wantRawPath)
			}
		})
	}
}

func TestHandleSignup_QueryStringCredentials_PreventURLLeakage_IgnoresQueryParams(t *testing.T) {
	// Business context: Credentials in URL query strings leak via server logs,
	// proxy logs, and Referer headers. FormValue() reads both query strings and
	// POST body, so we must use PostFormValue() to only accept body params.
	// Scenario: POST /signup with credentials ONLY in the query string, empty body.
	// Expected: Returns 400 because PostFormValue returns empty strings.

	// We need a handler that doesn't need real stores — just test the form parsing.
	// HandleSignup with nil stores will panic on DB call, but validation runs first.
	// If username comes from query string, FormValue returns it but PostFormValue won't.
	handler := HandleSignup(nil, nil)

	req := httptest.NewRequest("POST", "/signup?username=alice&password=secretpass", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 — query string credentials should be rejected", w.Code)
	}
}

func TestHandleStaticFile_HTMLFile_ServeWithCorrectContentType_ReturnsTextHTML(t *testing.T) {
	// Business context: The landing page is an HTML file with OG meta tags for
	// social media previews. HandleStaticFile must detect the content type from
	// the file extension so crawlers receive text/html, not text/plain.
	// Scenario: Serve a .html file via HandleStaticFile.
	// Expected: Content-Type is text/html, status 200, body matches file content.

	dir := t.TempDir()
	htmlPath := dir + "/test.html"
	os.WriteFile(htmlPath, []byte("<html><head><title>test</title></head></html>"), 0644)

	handler := HandleStaticFile(htmlPath)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
}

func TestHandleStaticFile_PNGFile_ServeWithCorrectContentType_ReturnsImagePNG(t *testing.T) {
	// Business context: The OG image for social media previews is a PNG file.
	// HandleStaticFile must serve it with image/png content type so crawlers
	// recognize it as an image, not a text file.
	// Scenario: Serve a .png file via HandleStaticFile.
	// Expected: Content-Type is image/png, status 200, body matches file content.

	dir := t.TempDir()
	pngPath := dir + "/test.png"
	// Minimal PNG header (8 bytes signature)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	os.WriteFile(pngPath, pngData, 0644)

	handler := HandleStaticFile(pngPath)
	req := httptest.NewRequest("GET", "/og-image.png", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("content-type = %q, want image/png", ct)
	}
	if w.Body.Len() != len(pngData) {
		t.Fatalf("body length = %d, want %d", w.Body.Len(), len(pngData))
	}
}

func TestHandleStaticFile_TxtBrowserAccept_ContentNegotiation_ReturnsHTML(t *testing.T) {
	// Business context: When a browser (Accept: text/html) visits a .txt URL like
	// /index.txt or /spec.txt, we want to render a styled HTML page so humans can
	// read comfortably. Agents get raw plaintext at the same URL.
	// Scenario: Serve a .txt file with a browser Accept header.
	// Expected: Returns text/html with styled content, not text/plain.

	dir := t.TempDir()
	txtPath := dir + "/test.txt"
	os.WriteFile(txtPath, []byte("# Hello\n\nWorld."), 0644)

	h := HandleStaticFile(txtPath)

	// Browser request
	req := httptest.NewRequest("GET", "/test.txt", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html for browser", ct)
	}
	if !strings.Contains(w.Body.String(), "#0d1117") {
		t.Error("missing dark theme in HTML output")
	}
	if !strings.Contains(w.Body.String(), "World.") {
		t.Error("missing page content in HTML output")
	}
	vary := w.Header().Get("Vary")
	if !strings.Contains(vary, "Accept") {
		t.Error("missing Vary: Accept header for caching")
	}
}

func TestHandleStaticFile_TxtAgentAccept_ContentNegotiation_ReturnsPlaintext(t *testing.T) {
	// Business context: Agents, curl, and MCP clients must get raw plaintext at
	// the same URL so they can parse it directly. Content negotiation must not
	// break the existing agent experience.
	// Scenario: Serve a .txt file with no Accept or with */*.
	// Expected: Returns text/plain with raw content, unchanged from current behavior.

	dir := t.TempDir()
	txtPath := dir + "/test.txt"
	os.WriteFile(txtPath, []byte("# Hello\n\nWorld."), 0644)

	h := HandleStaticFile(txtPath)

	// Agent request (curl-style)
	req := httptest.NewRequest("GET", "/test.txt", nil)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain for agent", ct)
	}
	if w.Body.String() != "# Hello\n\nWorld." {
		t.Fatalf("body = %q, want raw plaintext", w.Body.String())
	}
}

func TestHandleStaticFile_NonTxtBrowserAccept_NoNegotiation_ReturnsOriginal(t *testing.T) {
	// Business context: Content negotiation only applies to .txt files. HTML files,
	// PNGs, and other assets must be served as-is regardless of Accept header.
	// Scenario: Serve a .html file with a browser Accept header.
	// Expected: Returns text/html with original content (no double-wrapping).

	dir := t.TempDir()
	htmlPath := dir + "/test.html"
	os.WriteFile(htmlPath, []byte("<html><body>hi</body></html>"), 0644)

	h := HandleStaticFile(htmlPath)

	req := httptest.NewRequest("GET", "/test.html", nil)
	req.Header.Set("Accept", "text/html,*/*;q=0.8")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Should serve original HTML, not wrap it
	if w.Body.String() != "<html><body>hi</body></html>" {
		t.Fatalf("body = %q, original HTML should not be modified", w.Body.String())
	}
}

func TestLandingPage_OGTags_SocialMediaPreview_ContainsAllRequiredTags(t *testing.T) {
	// Business context: Social media platforms (Facebook, Twitter/X, LinkedIn,
	// Discord, Slack) use OG meta tags to render link previews. Missing tags
	// mean broken or ugly previews, hurting discoverability.
	// Scenario: Serve content/index.html and verify all required OG meta tags are present.
	// Expected: Response body contains og:title, og:description, og:image, og:type,
	// og:url, twitter:card, twitter:title, twitter:description, twitter:image.

	handler := HandleStaticFile("../../content/index.html")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()

	requiredTags := []struct {
		tag    string
		reason string
	}{
		// Open Graph — Facebook, LinkedIn, Discord, Slack
		{`og:type" content="website"`, "Facebook requires og:type for proper card rendering"},
		{`og:url" content="https://txtscape.com/"`, "canonical URL for share deduplication"},
		{`og:title" content="txtscape"`, "title shown in link preview cards"},
		{`og:description"`, "description shown below title in previews"},
		{`og:image" content="https://txtscape.com/og-image.png"`, "preview image URL for all OG consumers"},
		{`og:image:width" content="1200"`, "explicit dimensions prevent layout shift on Facebook"},
		{`og:image:height" content="630"`, "explicit dimensions prevent layout shift on Facebook"},
		{`og:image:alt"`, "accessibility text for the preview image"},
		{`og:site_name" content="txtscape"`, "site name shown above title on Facebook"},

		// Twitter/X
		{`twitter:card" content="summary_large_image"`, "large image card for maximum visibility on X"},
		{`twitter:title" content="txtscape"`, "title for Twitter card"},
		{`twitter:description"`, "description for Twitter card"},
		{`twitter:image" content="https://txtscape.com/og-image.png"`, "image URL for Twitter card"},
		{`twitter:image:alt"`, "accessibility text for Twitter card image"},
	}

	for _, tt := range requiredTags {
		if !strings.Contains(body, tt.tag) {
			t.Errorf("missing OG tag %q — %s", tt.tag, tt.reason)
		}
	}
}

func TestLandingPage_OGImage_ReferencedURLMatchesRoute_ImageIsAccessible(t *testing.T) {
	// Business context: The og:image URL must point to an actual servable image.
	// If the PNG doesn't exist or the path is wrong, crawlers get a 404 and
	// render no preview image.
	// Scenario: Set up a mux with the OG image route and the landing page,
	// extract the og:image URL, and verify the image is reachable.
	// Expected: GET /og-image.png returns 200 with image/png content type.

	mux := http.NewServeMux()
	mux.HandleFunc("GET /og-image.png", HandleStaticFile("../../content/og-image.png"))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/og-image.png")
	if err != nil {
		t.Fatalf("GET /og-image.png: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("content-type = %q, want image/png", ct)
	}
}

func TestLandingPage_RootRoute_CrawlerGetsHTML_ReturnsHTMLNotPlainText(t *testing.T) {
	// Business context: Social media crawlers fetch the root URL. They need HTML
	// with meta tags, not plain text. If root serves text/plain, no preview renders.
	// Scenario: Build a mux mimicking main.go's root route, GET /.
	// Expected: Returns text/html content type with status 200.

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			HandleStaticFile("../../content/index.html")(w, r)
			return
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
}

func TestHandleLogin_QueryStringCredentials_PreventURLLeakage_IgnoresQueryParams(t *testing.T) {
	// Business context: Same as signup — credentials must come from POST body only.
	// Scenario: POST /login with credentials ONLY in the query string, empty body.
	// Expected: Returns 400 or 401 because PostFormValue returns empty strings.

	handler := HandleLogin(nil, nil)

	req := httptest.NewRequest("POST", "/login?username=alice&password=secretpass", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// With PostFormValue, username is empty → GetPasswordHash gets "" → either
	// validation fails or user not found. Either way, not 200.
	if w.Code == http.StatusOK {
		t.Fatal("status = 200 — query string credentials should not auth successfully")
	}
}

// fakeUserLister implements UserLister for unit tests.
type fakeUserLister struct {
	stats []UserStat
	err   error
}

func (f *fakeUserLister) ListUserStats(ctx context.Context) ([]UserStat, error) {
	return f.stats, f.err
}

func TestHandleUsers_BrowserAccept_ContentNegotiation_ReturnsHTML(t *testing.T) {
	// Business context: When a browser visits /users.txt, we want to render a styled
	// HTML page with clickable links to user profiles. Agents still get raw plaintext.
	// Scenario: GET /users.txt with browser Accept header.
	// Expected: Returns text/html with clickable user links and dark theme.

	joined := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	lister := &fakeUserLister{
		stats: []UserStat{
			{Username: "alice", Pages: 5, TotalSizeBytes: 2048, JoinedAt: joined},
		},
	}

	h := HandleUsers(lister)

	req := httptest.NewRequest("GET", "/users.txt", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html for browser", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "#0d1117") {
		t.Error("missing dark theme")
	}
	if !strings.Contains(body, `<a href="/~alice">`) {
		t.Error("missing clickable link to ~alice")
	}
	vary := w.Header().Get("Vary")
	if !strings.Contains(vary, "Accept") {
		t.Error("missing Vary: Accept header")
	}
}

func TestHandleUsers_MultipleUsers_PublicDirectory_ReturnsFormattedListing(t *testing.T) {
	// Business context: /users.txt is a public directory of all users with basic
	// stats, allowing agents and humans to discover who is publishing content.
	// Scenario: Two users exist with different page counts and join dates.
	// Expected: Returns text/plain with a formatted listing showing username,
	// page count, and join date for each user.

	joined := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	lister := &fakeUserLister{
		stats: []UserStat{
			{Username: "alice", Pages: 5, TotalSizeBytes: 2048, JoinedAt: joined},
			{Username: "bob", Pages: 12, TotalSizeBytes: 51200, JoinedAt: joined.Add(24 * time.Hour)},
		},
	}

	h := HandleUsers(lister)
	req := httptest.NewRequest("GET", "/users.txt", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Fatalf("content-type = %q, want text/plain; charset=utf-8", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "alice") {
		t.Error("body missing alice")
	}
	if !strings.Contains(body, "bob") {
		t.Error("body missing bob")
	}
	if !strings.Contains(body, "5 pages") {
		t.Error("body missing alice's page count")
	}
	if !strings.Contains(body, "12 pages") {
		t.Error("body missing bob's page count")
	}
	if !strings.Contains(body, "/~alice") {
		t.Error("body missing link to alice's profile")
	}
	if !strings.Contains(body, "/~bob") {
		t.Error("body missing link to bob's profile")
	}
}

func TestHandleUsers_NoUsers_EmptyNetwork_ReturnsHeaderOnly(t *testing.T) {
	// Business context: When no one has signed up yet, the page should still render
	// cleanly with a header but no user entries.
	// Scenario: Empty user list.
	// Expected: Returns 200 with a header but no user lines.

	lister := &fakeUserLister{stats: []UserStat{}}

	h := HandleUsers(lister)
	req := httptest.NewRequest("GET", "/users.txt", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "# users") {
		t.Error("missing header")
	}
}

func TestHandleUsers_DBError_GracefulFailure_Returns500(t *testing.T) {
	// Business context: If the database is unreachable, the handler should return
	// a clean error rather than crashing or returning partial data.
	// Scenario: UserLister returns an error.
	// Expected: Returns 500 with a JSON error.

	lister := &fakeUserLister{err: errors.New("connection refused")}

	h := HandleUsers(lister)
	req := httptest.NewRequest("GET", "/users.txt", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
