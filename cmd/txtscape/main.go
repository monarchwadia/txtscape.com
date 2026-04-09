package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/txtscape/txtscape.com/internal/auth"
	"github.com/txtscape/txtscape.com/internal/handler"
	"github.com/txtscape/txtscape.com/internal/mcp"
	"github.com/txtscape/txtscape.com/internal/pages"
)

func main() {
	// MCP mode: run as stdio JSON-RPC server
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		mcp.Serve()
		return
	}

	// HTTP server mode
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	userStore := &auth.UserStore{DB: pool}
	tokenStore := &auth.TokenStore{DB: pool}
	pageStore := &pages.PageStore{DB: pool}

	mux := http.NewServeMux()

	// Static content
	mux.HandleFunc("GET /index.txt", handler.HandleStaticFile("content/index.txt"))
	mux.HandleFunc("GET /spec.txt", handler.HandleStaticFile("content/spec.txt"))

	// Auth
	mux.HandleFunc("POST /signup", handler.HandleSignup(userStore, tokenStore))
	mux.HandleFunc("POST /login", handler.HandleLogin(userStore, tokenStore))

	// User pages — catch-all for /~ paths
	tildeHandler := handler.HandleTilde(tokenStore, pageStore)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/~") {
			tildeHandler(w, r)
			return
		}
		// Default: redirect root to index.txt, 404 everything else
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			http.Redirect(w, r, "/index.txt", http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
	})

	// Root
	// (handled by catch-all above)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("txtscape listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
