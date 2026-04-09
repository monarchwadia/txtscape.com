//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/txtscape/txtscape.com/internal/auth"
	"github.com/txtscape/txtscape.com/internal/handler"
	"github.com/txtscape/txtscape.com/internal/pages"
	"github.com/txtscape/txtscape.com/internal/testutil"
)

var db *pgxpool.Pool

func setupServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	pool, cleanup := testutil.SetupTestDB(t)
	db = pool

	userStore := &auth.UserStore{DB: pool}
	tokenStore := &auth.TokenStore{DB: pool}
	pageStore := &pages.PageStore{DB: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", handler.HandleSignup(userStore, tokenStore))
	mux.HandleFunc("POST /login", handler.HandleLogin(userStore, tokenStore))
	tildeHandler := handler.HandleTilde(tokenStore, pageStore)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/~") {
			tildeHandler(w, r)
			return
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	return srv, func() {
		srv.Close()
		cleanup()
	}
}

type tokenResp struct {
	Token string `json:"token"`
}

type errorResp struct {
	Error string `json:"error"`
}

func signup(t *testing.T, srv *httptest.Server, username, password string) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/signup",
		"application/x-www-form-urlencoded",
		strings.NewReader("username="+username+"&password="+password))
	if err != nil {
		t.Fatalf("signup request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("signup status = %d, body = %s", resp.StatusCode, body)
	}
	var tok tokenResp
	json.NewDecoder(resp.Body).Decode(&tok)
	return tok.Token
}

func TestSignup_NewUser_Register_ReturnsToken(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "alice", "password123")
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
}

func TestSignup_DuplicateUser_PreventConflict_Returns409(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "alice", "password123")

	resp, err := http.Post(srv.URL+"/signup",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=alice&password=password456"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

func TestLogin_ValidCredentials_Authenticate_ReturnsToken(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "bob", "password123")

	resp, err := http.Post(srv.URL+"/login",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=bob&password=password123"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var tok tokenResp
	json.NewDecoder(resp.Body).Decode(&tok)
	if tok.Token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLogin_WrongPassword_RejectIntruder_Returns401(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "charlie", "password123")

	resp, err := http.Post(srv.URL+"/login",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=charlie&password=wrongpassword"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPutGet_PublishAndRead_FullCycle_ReturnsContent(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "dave", "password123")

	// PUT a page
	req, _ := http.NewRequest("PUT", srv.URL+"/~dave/hello.txt", strings.NewReader("# Hello World"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204", resp.StatusCode)
	}

	// GET the page
	resp, err = http.Get(srv.URL + "/~dave/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "# Hello World" {
		t.Fatalf("body = %q, want %q", string(body), "# Hello World")
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Fatalf("content-type = %q, want text/plain; charset=utf-8", ct)
	}
}

func TestPut_NoAuth_Unauthorized_Returns401(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "eve", "password123")

	req, _ := http.NewRequest("PUT", srv.URL+"/~eve/hello.txt", strings.NewReader("content"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPut_WrongToken_Forbidden_Returns403(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "frank", "password123")

	req, _ := http.NewRequest("PUT", srv.URL+"/~frank/hello.txt", strings.NewReader("content"))
	req.Header.Set("Authorization", "Bearer invalidtoken1234567890123456789012345678901234567890123456789012")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestDelete_ExistingPage_RemovePage_Returns204(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "grace", "password123")

	// PUT a page
	req, _ := http.NewRequest("PUT", srv.URL+"/~grace/hello.txt", strings.NewReader("content"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// DELETE it
	req, _ = http.NewRequest("DELETE", srv.URL+"/~grace/hello.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}

	// GET should 404
	resp, _ = http.Get(srv.URL + "/~grace/hello.txt")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after delete status = %d, want 404", resp.StatusCode)
	}
}

func TestDirectoryListing_NoIndex_AutoGenerate_ReturnsMarkdown(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "heidi", "password123")

	// PUT some pages
	for _, name := range []string{"about.txt", "blog.txt"} {
		req, _ := http.NewRequest("PUT", srv.URL+"/~heidi/"+name, strings.NewReader("content"))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}

	// GET user root (directory listing)
	resp, err := http.Get(srv.URL + "/~heidi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	listing := string(body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(listing, "# ~heidi") {
		t.Fatalf("listing missing header, got: %s", listing)
	}
	if !strings.Contains(listing, "about.txt") {
		t.Fatalf("listing missing about.txt, got: %s", listing)
	}
	if !strings.Contains(listing, "blog.txt") {
		t.Fatalf("listing missing blog.txt, got: %s", listing)
	}
}

func TestDirectoryListing_WithIndex_ServeIndex_ReturnsIndexContent(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "ivan", "password123")

	// PUT index.txt
	req, _ := http.NewRequest("PUT", srv.URL+"/~ivan/index.txt", strings.NewReader("# Ivan's Homepage"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// GET user root should serve index.txt
	resp, err := http.Get(srv.URL + "/~ivan")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "# Ivan's Homepage" {
		t.Fatalf("body = %q, want %q", string(body), "# Ivan's Homepage")
	}
}

func TestPutUpdate_ModifyExisting_UpdateContent_ReturnsNewContent(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "judy", "password123")

	// PUT v1
	req, _ := http.NewRequest("PUT", srv.URL+"/~judy/page.txt", strings.NewReader("version 1"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// PUT v2 (update)
	req, _ = http.NewRequest("PUT", srv.URL+"/~judy/page.txt", strings.NewReader("version 2"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT update status = %d, want 204", resp.StatusCode)
	}

	// GET should return v2
	resp, err := http.Get(srv.URL + "/~judy/page.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "version 2" {
		t.Fatalf("body = %q, want %q", string(body), "version 2")
	}
}

func TestNestedPath_DeepFolder_OrganizedContent_Works(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token := signup(t, srv, "karl", "password123")

	// PUT a nested page
	req, _ := http.NewRequest("PUT", srv.URL+"/~karl/blog/2026/post.txt", strings.NewReader("deep content"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204", resp.StatusCode)
	}

	// GET nested page
	resp, err := http.Get(srv.URL + "/~karl/blog/2026/post.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "deep content" {
		t.Fatalf("body = %q, want %q", string(body), "deep content")
	}
}
