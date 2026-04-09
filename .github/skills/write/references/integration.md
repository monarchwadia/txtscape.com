# Integration Tests

## Scope

Test a single API endpoint end-to-end against a real Postgres database.
HTTP request → router → handler → database → HTTP response.

## What to test

1. **Auth endpoints**: POST /signup, POST /login — correct status codes, token issuance, error cases
2. **Page CRUD**: PUT/DELETE/GET ~username/path.txt — content storage, retrieval, deletion
3. **Postgres constraints**: CHECK constraints fire correctly (bad filenames, bad paths, oversized)
4. **Application limits**: 100 files per folder, 10 subfolders, 10 levels deep
5. **Auth middleware**: Bearer token validation on PUT/DELETE, rejection without token
6. **Directory listings**: auto-generated listing when no index.txt exists

## Build tag

```go
//go:build integration
```

Run with: `go test -tags=integration ./...`

## File placement

Same package as the code, separate file:
- `internal/auth/auth_integration_test.go`
- `internal/pages/pages_integration_test.go`

## Test structure

```go
func TestSignupAPI_ValidCredentials_AllowNewUsers_Returns200(t *testing.T) {
    srv := httptest.NewServer(NewRouter(db))
    defer srv.Close()

    body := "username=alice&password=secret123"
    req, _ := http.NewRequest("POST", srv.URL+"/signup", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        t.Errorf("got status %d, want 200", resp.StatusCode)
    }
}
```

## Database setup

See [db-setup.md](db-setup.md) for the TestMain pattern and per-package database lifecycle.

## Edge cases specific to integration

- Concurrent writes to the same path (UNIQUE constraint should reject)
- PUT to another user's namespace (must be rejected by auth middleware)
- Token from one user used to write to another user's space
- DELETE a file that doesn't exist (should 404, not 500)
- GET a file that was just deleted (should 404)
- Empty body on PUT
- Postgres CHECK constraint violations (bad filenames, bad paths)
