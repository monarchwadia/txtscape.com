# Methodology: Walking Skeleton + Red/Green TDD

This is the core discipline. Every piece of code in txtscape is built this way.

## The Walking Skeleton

Before building any feature fully, build the thinnest possible end-to-end slice through all layers. Prove the wiring works before adding logic.

### Example: building the signup feature

**Phase 1 — Steel thread (skeleton)**

Build a handler that does nothing real but proves the plumbing works:

```go
// Step 1: Write the test (RED)
func TestSignup_Endpoint_Exists_Returns200(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("POST /signup", handleSignup(nil))
    srv := httptest.NewServer(mux)
    defer srv.Close()

    resp, _ := http.Post(srv.URL+"/signup", "application/x-www-form-urlencoded", 
        strings.NewReader("username=alice&password=secret"))
    if resp.StatusCode != 200 {
        t.Fatalf("got %d, want 200", resp.StatusCode)
    }
}

// Step 2: Write the minimum code to pass (GREEN)
func handleSignup(db UserStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        json.NewEncoder(w).Encode(map[string]string{"token": "fake"})
    }
}
```

Run it. It passes. The skeleton stands.

**Phase 2 — Fill in the bones, one at a time**

Now replace the fake with real, one test at a time:

```
RED:   TestSignup_ValidInput_AllowNewUsers_CreatesUserInDB
GREEN: Add parsing + DB insert
RED:   TestSignup_DuplicateUsername_PreventImpersonation_Returns409
GREEN: Add unique check
RED:   TestSignup_EmptyUsername_RequireIdentity_Returns400
GREEN: Add validation
RED:   TestSignup_ShortPassword_EnforceMinSecurity_Returns400
GREEN: Add password length check
```

Each RED/GREEN cycle is ONE test and ONE small code change.

### Example: building the whole server

The walking skeleton for the entire project:

```
1. main.go that starts an HTTP server on $PORT → verify it responds
2. GET /health returns 200 "ok" → verify with curl
3. POST /signup returns hardcoded {"token":"fake"} → test passes
4. POST /signup actually writes to Postgres → test with real DB
5. PUT /~alice/hello.txt returns 201 with hardcoded response → test passes
6. PUT /~alice/hello.txt actually writes to DB → test with real DB
7. GET /~alice/hello.txt returns the content → test passes
8. Wire up auth middleware (token check) → test passes
9. DELETE endpoint → test passes
10. Directory listings → test passes
```

Each step builds on the proven foundation of the previous step. If step 4 breaks, you know the problem is in step 4, not a cascade.

## Red/Green/Refactor Cycle

### RED: Write a failing test

1. Write exactly ONE test
2. Run it: `go test ./internal/auth/ -run TestSignup_ValidInput`
3. Confirm it fails (compile error or assertion failure)
4. If it passes without writing new code, the test isn't testing anything new — rewrite it

### GREEN: Write the minimum code to pass

1. Write the simplest, dumbest code that makes the test pass
2. This might mean hardcoding a return value — that's fine temporarily
3. Run the test again: confirm it passes
4. Run ALL tests in the package: confirm nothing else broke

### REFACTOR: Clean up only if needed

1. Look for duplication or awkwardness in the code you just wrote
2. Refactor only if the code is actively bad
3. Run tests again after refactoring
4. Do NOT add features during refactor

### The discipline

- **Never write production code without a failing test demanding it**
- **Never write more than one test before making it pass**
- **Never skip running the tests between steps**
- **A little token waste from incremental steps is better than a big broken generation**

## Why this matters for LLMs

LLMs are tempted to generate entire files at once. This fails because:

1. **Compounding errors**: a mistake in line 20 cascades to lines 50, 80, 120
2. **No verification boundary**: you don't know which part broke
3. **Context overload**: generating 200 lines means holding 200 lines of context
4. **Hard to debug**: "the whole file is wrong" vs "this one test fails"

The stepwise approach:

1. **Each step is small enough to get right**
2. **Tests verify correctness at each step** — you build on proven foundations
3. **Errors are caught immediately** — the last 5 lines you wrote are the suspect
4. **The user can course-correct early** — not after 200 lines of generated code

## When to run tests

- After writing each failing test (confirm RED)
- After writing the implementation (confirm GREEN)
- After any refactor
- Use `go test -v -run TestSpecificName ./package/` for targeted runs
- Use `go test -v ./package/` to confirm no regressions

## Ordering principle

Build from the inside out:

1. **Pure functions first** — validation, parsing, formatting (no deps, easy to test)
2. **Store interfaces + implementations** — database layer
3. **Handlers** — HTTP layer that calls the store
4. **Wiring** — main.go connecting everything

Each layer gets its own red/green progression. Don't jump to handlers until the store tests pass. Don't jump to wiring until handler tests pass.

## Walking skeleton for txtscape specifically

```
Milestone 1: Server boots
  - main.go, GET /health → 200

Milestone 2: Signup works
  - POST /signup → fake token → real token with DB

Milestone 3: Login works
  - POST /login → validates password → returns token

Milestone 4: Publish works
  - PUT /~user/file.txt → fake 201 → real DB write with auth

Milestone 5: Read works
  - GET /~user/file.txt → returns content from DB

Milestone 6: Delete works
  - DELETE /~user/file.txt → removes from DB

Milestone 7: Directory listings
  - GET /~user → auto-generated listing when no index.txt

Milestone 8: Limits
  - 100KB file size, 10 levels, 10 subfolders, 100 files per folder

Milestone 9: MCP surf
  - stdin/stdout MCP tool that fetches any URL

Milestone 10: Static pages
  - GET /index.txt, GET /spec.txt served from DB or filesystem
```

Each milestone is a walking skeleton that gets fleshed out through red/green cycles.
