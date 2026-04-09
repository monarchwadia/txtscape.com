# DB Test Setup

Database lifecycle for integration and E2E tests.

## Strategy

Each test **package** gets its own temporary Postgres database. Tests within a package run sequentially (no `t.Parallel()`).

## TestMain Pattern

```go
var db *pgxpool.Pool

func TestMain(m *testing.M) {
    // 1. Connect to Postgres using DATABASE_URL
    adminURL := os.Getenv("DATABASE_URL")
    if adminURL == "" {
        fmt.Println("DATABASE_URL not set, skipping integration tests")
        os.Exit(0)
    }

    // 2. Create a unique database for this test package
    dbName := fmt.Sprintf("test_%s_%d", packageName(), time.Now().UnixNano())
    adminConn, _ := pgx.Connect(context.Background(), adminURL)
    adminConn.Exec(context.Background(), "CREATE DATABASE "+dbName)
    adminConn.Close(context.Background())

    // 3. Connect to the new database
    testURL := replaceDBName(adminURL, dbName)
    db, _ = pgxpool.New(context.Background(), testURL)

    // 4. Run migrations
    runMigrations(db)

    // 5. Run tests
    code := m.Run()

    // 6. Cleanup — always, even on failure
    db.Close()
    adminConn2, _ := pgx.Connect(context.Background(), adminURL)
    adminConn2.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName)
    adminConn2.Close(context.Background())

    os.Exit(code)
}
```

## Shared Helper (internal/testutil)

Put the DB lifecycle in a shared package so integration and e2e tests don't duplicate it:

```go
// testutil.SetupTestDB creates a fresh database, runs migrations, returns pool + cleanup.
func SetupTestDB(t testing.TB) (*pgxpool.Pool, func()) {
    t.Helper()

    adminURL := os.Getenv("DATABASE_URL")
    if adminURL == "" {
        t.Skip("DATABASE_URL not set")
    }

    ctx := context.Background()
    dbName := fmt.Sprintf("test_%d", time.Now().UnixNano())

    adminConn, err := pgx.Connect(ctx, adminURL)
    if err != nil {
        t.Fatal(err)
    }
    _, err = adminConn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
    if err != nil {
        t.Fatal(err)
    }
    adminConn.Close(ctx)

    testURL := replaceDBName(adminURL, dbName)
    pool, err := pgxpool.New(ctx, testURL)
    if err != nil {
        t.Fatal(err)
    }
    runMigrations(pool)

    cleanup := func() {
        pool.Close()
        conn, _ := pgx.Connect(ctx, adminURL)
        conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize())
        conn.Close(ctx)
    }

    return pool, cleanup
}
```

## Why per-package, not per-test

- Packages already run in parallel (`go test ./...` parallelizes across packages)
- Tests within a package are sequential by default — no conflicts
- Creating a DB per test is slow (~100ms per CREATE DATABASE) and unnecessary
- If test isolation within a package is ever needed, use DELETE FROM tables between tests

## Running

```sh
# Unit tests only (no DB needed)
go test ./...

# Integration tests
DATABASE_URL=postgres://user:pass@localhost:5432/postgres go test -tags=integration ./...

# E2E tests
DATABASE_URL=postgres://user:pass@localhost:5432/postgres go test -tags=e2e ./e2e/
```
