# Test Helpers

Shared helpers for integration and E2E tests. Live in test files, not production code.

## HTTP Helpers

```go
// signUp creates a new user and returns the token.
func signUp(t *testing.T, baseURL, username, password string) string {
    t.Helper()
    body := fmt.Sprintf("username=%s&password=%s", url.QueryEscape(username), url.QueryEscape(password))
    resp, err := http.Post(baseURL+"/signup", "application/x-www-form-urlencoded", strings.NewReader(body))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    requireStatus(t, resp, 200)
    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    if result["token"] == "" {
        t.Fatal("signup returned empty token")
    }
    return result["token"]
}

// login logs in and returns the token.
func login(t *testing.T, baseURL, username, password string) string {
    t.Helper()
    body := fmt.Sprintf("username=%s&password=%s", url.QueryEscape(username), url.QueryEscape(password))
    resp, err := http.Post(baseURL+"/login", "application/x-www-form-urlencoded", strings.NewReader(body))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    requireStatus(t, resp, 200)
    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    return result["token"]
}

// putPage publishes a page and returns the response.
func putPage(t *testing.T, baseURL, path, token, content string) *http.Response {
    t.Helper()
    req, _ := http.NewRequest("PUT", baseURL+path, strings.NewReader(content))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "text/plain")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatal(err)
    }
    return resp
}

// deletePage deletes a page and returns the response.
func deletePage(t *testing.T, baseURL, path, token string) *http.Response {
    t.Helper()
    req, _ := http.NewRequest("DELETE", baseURL+path, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatal(err)
    }
    return resp
}

// getPage fetches a page (no auth) and returns the response.
func getPage(t *testing.T, baseURL, path string) *http.Response {
    t.Helper()
    resp, err := http.Get(baseURL + path)
    if err != nil {
        t.Fatal(err)
    }
    return resp
}

// readBody reads and returns the response body as a string, closing it.
func readBody(t *testing.T, resp *http.Response) string {
    t.Helper()
    defer resp.Body.Close()
    b, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatal(err)
    }
    return string(b)
}

// requireStatus fails the test immediately if status doesn't match.
func requireStatus(t *testing.T, resp *http.Response, want int) {
    t.Helper()
    if resp.StatusCode != want {
        body, _ := io.ReadAll(resp.Body)
        t.Fatalf("status: got %d, want %d. Body: %s", resp.StatusCode, want, body)
    }
}
```

## Usage

These helpers keep test bodies focused on the journey, not HTTP boilerplate:

```go
func TestJourney_NewUserPublishesFirstPage_CoreUseCase_PageIsAccessible(t *testing.T) {
    token := signUp(t, srv.URL, "alice", "secret123")

    resp := putPage(t, srv.URL, "/~alice/hello.txt", token, "# Hello\nThis is my first page.")
    requireStatus(t, resp, 201)
    resp.Body.Close()

    resp = getPage(t, srv.URL, "/~alice/hello.txt")
    requireStatus(t, resp, 200)
    body := readBody(t, resp)
    if !strings.Contains(body, "Hello") {
        t.Errorf("page content missing expected text, got: %s", body)
    }
}
```
