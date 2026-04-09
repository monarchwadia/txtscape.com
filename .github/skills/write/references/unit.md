# Unit Tests

## Scope

Test a single function or method in isolation. All external dependencies (DB, HTTP, filesystem) are mocked via interfaces.

## Mocking Strategy

Functions under test accept interfaces, not concrete types:

```go
type UserStore interface {
    CreateUser(ctx context.Context, username, passwordHash string) error
    GetUser(ctx context.Context, username string) (*User, error)
}

type PageStore interface {
    GetPage(ctx context.Context, username, folderPath, fileName string) (*Page, error)
    PutPage(ctx context.Context, page *Page) error
    DeletePage(ctx context.Context, username, folderPath, fileName string) error
    ListFolder(ctx context.Context, username, folderPath string) ([]FolderEntry, error)
}
```

Mock with simple structs:

```go
type mockUserStore struct {
    createErr error
    getUser   *User
    getErr    error
}

func (m *mockUserStore) CreateUser(ctx context.Context, username, hash string) error {
    return m.createErr
}

func (m *mockUserStore) GetUser(ctx context.Context, username string) (*User, error) {
    return m.getUser, m.getErr
}
```

## What to unit test

- **Validation functions**: path parsing, username validation, filename validation
- **Pure logic**: directory listing generation, path splitting, size checking
- **Handler logic**: with mocked stores — correct status codes, correct response bodies
- **Token generation**: produces hex strings of expected length
- **Password hashing**: bcrypt round-trip

## What NOT to unit test

- Database queries (that's integration testing)
- HTTP routing (that's integration testing)
- Third-party library internals

## File placement

Same package as the code:
- `internal/auth/auth.go` → `internal/auth/auth_test.go`
- `internal/pages/path.go` → `internal/pages/path_test.go`

No build tags needed — unit tests run by default with `go test ./...`
