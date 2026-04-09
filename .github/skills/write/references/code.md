# Production Code Conventions

## Architecture

Single Go binary. Deployed on Railway with Railway Postgres.

```
cmd/server/main.go          — entry point, wiring
internal/auth/               — signup, login, token CRUD
internal/pages/              — PUT/DELETE/GET pages, directory listings
internal/mcp/                — MCP surf(url) tool (stdin/stdout JSON-RPC)
internal/testutil/           — shared test helpers (DB setup/teardown)
migrations/                  — SQL migration files
e2e/                         — end-to-end journey tests
```

## Dependencies

- `github.com/jackc/pgx/v5` — Postgres driver (use `pgxpool` for connection pooling)
- `golang.org/x/crypto/bcrypt` — password and token hashing
- Go stdlib for everything else: `net/http`, `encoding/json`, `crypto/rand`, `log`

## HTTP Routing

Use Go 1.22+ pattern matching in `http.ServeMux`:

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /signup", handleSignup(db))
mux.HandleFunc("POST /login", handleLogin(db))
mux.HandleFunc("PUT /~{username}/{path...}", requireAuth(handlePutPage(db), tokenStore))
mux.HandleFunc("DELETE /~{username}/{path...}", requireAuth(handleDeletePage(db), tokenStore))
mux.HandleFunc("GET /~{username}", handleUserRoot(db))
mux.HandleFunc("GET /~{username}/{path...}", handleGetPage(db))
mux.HandleFunc("GET /{path...}", handleStaticPage(db))
```

## Handler Pattern

Handlers are closures:

```go
func handleSignup(store UserStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ...
    }
}
```

## Error Responses

Plain text errors:

```go
http.Error(w, "username already taken", http.StatusConflict)
```

## Testability

Handlers accept interfaces, not concrete types. See [unit.md](unit.md) for interface definitions.

## Configuration

Environment variables only:

```go
port := os.Getenv("PORT")
dbURL := os.Getenv("DATABASE_URL")
```

## Code Style

- No global variables except in `main()`
- Return errors, don't panic
- Use `context.Context` for all DB and HTTP operations
- `log.Printf` for logging (no logging framework)
- No comments on obvious code. Comment non-obvious business logic
- Always parameterized queries (`$1`, `$2`). Never interpolate user input into SQL
