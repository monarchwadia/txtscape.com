package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupTestDB creates a fresh temporary database, runs migrations, and returns
// a connection pool plus a cleanup function. Skips the test if DATABASE_URL is not set.
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
		t.Fatalf("connecting to admin db: %v", err)
	}
	_, err = adminConn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	if err != nil {
		t.Fatalf("creating test db: %v", err)
	}
	adminConn.Close(ctx)

	testURL := replaceDBName(adminURL, dbName)
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}

	runMigrations(t, pool)

	cleanup := func() {
		pool.Close()
		conn, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			return
		}
		conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize())
		conn.Close(ctx)
	}

	return pool, cleanup
}

func replaceDBName(connURL, dbName string) string {
	u, err := url.Parse(connURL)
	if err != nil {
		return connURL
	}
	u.Path = "/" + dbName
	return u.String()
}

func runMigrations(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()

	// Try several relative paths to find the migration file from different test directories
	paths := []string{
		"migrations/001_init.sql",
		"../migrations/001_init.sql",
		"../../migrations/001_init.sql",
		"../../../migrations/001_init.sql",
	}

	var migration []byte
	var err error
	for _, p := range paths {
		migration, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("reading migration: %v", err)
	}

	_, err = pool.Exec(context.Background(), string(migration))
	if err != nil {
		t.Fatalf("running migration: %v", err)
	}
}
