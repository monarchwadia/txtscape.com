package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/txtscape/txtscape.com/internal/auth"
	"github.com/txtscape/txtscape.com/internal/pages"
)

const maxBodySize = 102400 // 100KB

type jsonError struct {
	Error string `json:"error"`
}

type jsonToken struct {
	Token string `json:"token"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, jsonError{Error: msg})
}

// HandleSignup handles POST /signup.
func HandleSignup(users *auth.UserStore, tokens *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form data")
			return
		}
		username := r.PostFormValue("username")
		password := r.PostFormValue("password")

		if err := auth.ValidateUsername(username); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := auth.ValidatePassword(password); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		hash, err := auth.HashPassword(password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := users.Create(r.Context(), username, hash); err != nil {
			if errors.Is(err, auth.ErrUserExists) {
				writeError(w, http.StatusConflict, "username already taken")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		plaintext, tokenHash, err := auth.GenerateToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := tokens.Create(r.Context(), username, "default", tokenHash); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusCreated, jsonToken{Token: plaintext})
	}
}

// HandleLogin handles POST /login.
func HandleLogin(users *auth.UserStore, tokens *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form data")
			return
		}
		username := r.PostFormValue("username")
		password := r.PostFormValue("password")

		if username == "" || password == "" {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		hash, err := users.GetPasswordHash(r.Context(), username)
		if err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := auth.CheckPassword(password, hash); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		plaintext, tokenHash, err := auth.GenerateToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := tokens.Create(r.Context(), username, "default", tokenHash); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, jsonToken{Token: plaintext})
	}
}

// authenticate checks the Bearer token against stored hashes for the given user.
// Returns true if the token is valid.
func authenticate(ctx context.Context, tokenStore *auth.TokenStore, username, bearerToken string) bool {
	hashes, err := tokenStore.GetHashesByUsername(ctx, username)
	if err != nil || len(hashes) == 0 {
		return false
	}
	for _, h := range hashes {
		if auth.CheckToken(bearerToken, h) {
			return true
		}
	}
	return false
}

// extractBearer extracts the token from "Bearer <token>" header value.
func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return header[len(prefix):]
	}
	return ""
}

// parseTildePath extracts username and remaining path from URLs like /~alice/blog/post.txt.
// Returns (username, remainingPath). remainingPath may be empty for /~alice.
func parseTildePath(urlPath string) (string, string) {
	// Must start with /~
	if len(urlPath) < 2 || urlPath[:2] != "/~" {
		return "", ""
	}
	rest := urlPath[2:] // "alice/blog/post.txt" or "alice"
	slashIdx := strings.IndexByte(rest, '/')
	if slashIdx == -1 {
		return rest, ""
	}
	return rest[:slashIdx], rest[slashIdx+1:]
}

// HandleTilde is a single handler for all /~username/... routes.
// It dispatches by HTTP method to the appropriate action.
func HandleTilde(tokenStore *auth.TokenStore, pageStore *pages.PageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, rawPath := parseTildePath(r.URL.Path)
		if username == "" {
			writeError(w, http.StatusBadRequest, "invalid user path")
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleGetPage(w, r, pageStore, username, rawPath)
		case http.MethodPut:
			handlePutPage(w, r, tokenStore, pageStore, username, rawPath)
		case http.MethodDelete:
			handleDeletePage(w, r, tokenStore, pageStore, username, rawPath)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

func handlePutPage(w http.ResponseWriter, r *http.Request, tokenStore *auth.TokenStore, pageStore *pages.PageStore, username, rawPath string) {
	if rawPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	token := extractBearer(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}

	if !authenticate(r.Context(), tokenStore, username, token) {
		writeError(w, http.StatusForbidden, "invalid token")
		return
	}

	parsed, err := pages.ParsePath(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read body")
		return
	}
	if len(body) > maxBodySize {
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds 100KB limit")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "body is empty")
		return
	}

	contents := string(body)
	if err := pageStore.Upsert(r.Context(), username, parsed.FolderPath, parsed.FileName, contents); err != nil {
		if errors.Is(err, pages.ErrTooManyFiles) {
			writeError(w, http.StatusConflict, "folder already has 100 files")
			return
		}
		if errors.Is(err, pages.ErrTooManyDirs) {
			writeError(w, http.StatusConflict, "folder already has 10 subfolders")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleDeletePage(w http.ResponseWriter, r *http.Request, tokenStore *auth.TokenStore, pageStore *pages.PageStore, username, rawPath string) {
	if rawPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	token := extractBearer(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}

	if !authenticate(r.Context(), tokenStore, username, token) {
		writeError(w, http.StatusForbidden, "invalid token")
		return
	}

	parsed, err := pages.ParsePath(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := pageStore.Delete(r.Context(), username, parsed.FolderPath, parsed.FileName); err != nil {
		if errors.Is(err, pages.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "page not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleGetPage(w http.ResponseWriter, r *http.Request, pageStore *pages.PageStore, username, rawPath string) {
	// No path or trailing slash = directory listing
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		serveListing(w, r, pageStore, username, rawPath)
		return
	}

	parsed, err := pages.ParsePath(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	contents, err := pageStore.Get(r.Context(), username, parsed.FolderPath, parsed.FileName)
	if err != nil {
		if errors.Is(err, pages.ErrPageNotFound) {
			writeError(w, http.StatusNotFound, "page not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, contents)
}

func serveListing(w http.ResponseWriter, r *http.Request, pageStore *pages.PageStore, username, rawPath string) {
	// Convert raw path to folder_path format
	folderPath := "/"
	if rawPath != "" {
		// Strip trailing slash, then wrap: "blog/2026/" -> "/blog/2026/"
		trimmed := strings.TrimSuffix(rawPath, "/")
		folderPath = "/" + trimmed + "/"
	}

	// First check if there's an index.txt in this folder
	contents, err := pageStore.Get(r.Context(), username, folderPath, "index.txt")
	if err == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, contents)
		return
	}

	// No index.txt — generate a directory listing
	entries, err := pageStore.ListFolder(r.Context(), username, folderPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	listing := pages.GenerateListing(username, folderPath, entries)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, listing)
}

// HandleStaticFile serves a static file from the filesystem,
// detecting content type from the file extension.
func HandleStaticFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// UserStat holds public stats for a single user.
type UserStat = auth.UserStat

// UserLister retrieves user statistics.
type UserLister interface {
	ListUserStats(ctx context.Context) ([]UserStat, error)
}

// HandleUsers serves GET /users.txt — a public directory of all users.
func HandleUsers(lister UserLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := lister.ListUserStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		var b strings.Builder
		b.WriteString("# users\n\n")
		for _, u := range stats {
			fmt.Fprintf(&b, "- [~%s](/~%s) — %d pages, joined %s\n",
				u.Username, u.Username, u.Pages, u.JoinedAt.Format("2006-01-02"))
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, b.String())
	}
}

// HandleRoot redirects / to /index.txt.
func HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	http.Redirect(w, r, "/index.txt", http.StatusMovedPermanently)
}
