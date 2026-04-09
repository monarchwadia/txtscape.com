//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/txtscape/txtscape.com/internal/auth"
	"github.com/txtscape/txtscape.com/internal/handler"
	"github.com/txtscape/txtscape.com/internal/pages"
	"github.com/txtscape/txtscape.com/internal/testutil"
)

type tokenResp struct {
	Token string `json:"token"`
}

type errorResp struct {
	Error string `json:"error"`
}

func setupServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	pool, cleanup := testutil.SetupTestDB(t)

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

func putPage(t *testing.T, srv *httptest.Server, token, path, content string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("PUT", srv.URL+path, strings.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func getPage(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func deletePage(t *testing.T, srv *httptest.Server, token, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("DELETE", srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}
