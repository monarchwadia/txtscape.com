# E2E Tests

## Scope

Multi-step user journeys. Each test tells a story with business meaning.

## How E2E differs

| Level | Scope | Example |
|-------|-------|---------|
| Unit | Single function, mocked deps | `TestParseFilePath_ValidInput_...` |
| Integration | Single endpoint, real DB | `TestPutPage_ValidBody_...` |
| **E2E** | **Multi-step journey, real DB** | **signup → publish → browse → delete → verify 404** |

## Build tag

```go
//go:build e2e
```

Run with: `go test -tags=e2e ./e2e/`

## File placement

```
e2e/
  e2e_test.go                 — TestMain, shared setup, helpers
  journey_publish_test.go     — publishing journeys
  journey_browse_test.go      — browsing/navigation journeys
  journey_auth_test.go        — auth journeys
  journey_limits_test.go      — limit enforcement journeys
```

## Test documentation

Every E2E test has a narrative comment:

```go
// Journey: New user publishes their first page
//
// Business context: This is the core use case of txtscape. A new user should
// go from zero to a published, publicly accessible .txt page in one session.
//
// Steps:
//   1. Sign up with username + password → receive token
//   2. PUT /~username/hello.txt with token → 201
//   3. GET /~username/hello.txt (no auth) → 200, content matches
//   4. GET /~username (no auth) → directory listing includes hello.txt
//
// Expected: All steps succeed. Page is publicly readable immediately.
```

## Journeys to cover

### Publishing
1. **First page** — signup → put → verify accessible → verify listing
2. **Folder organization** — create pages in nested folders → verify listings at each level
3. **Update existing** — put → put same path with new content → verify changed
4. **Delete** — put → delete → verify 404 → verify removed from listing
5. **Custom index.txt** — put index.txt → verify served at ~user → delete → verify listing returns

### Auth
6. **Cross-user protection** — signup as alice → try PUT to ~bob → 403
7. **Multiple tokens** — create second token → both work
8. **Wrong password** — login with wrong password → rejected

### Limits
9. **File too large** — PUT >100KB → 413, not created
10. **Folder depth** — 10 levels OK, 11 rejected
11. **Files per folder** — 100 OK, 101 rejected
12. **Subfolders per folder** — 10 OK, 11 rejected

### Browsing
13. **Cross-user links** — two users link to each other → both resolve
14. **Subdirectory listing** — files in /blog/ → GET /~user/blog/ shows listing

## Helpers

E2E tests use shared helpers. See [test-helpers.md](test-helpers.md).

## Database setup

See [db-setup.md](db-setup.md). Same TestMain pattern as integration tests.
