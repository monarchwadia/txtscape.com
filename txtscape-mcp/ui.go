package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed ui/static
var staticFiles embed.FS

// uiServer serves the SPA and REST API for browsing txtscape pages.
type uiServer struct {
	root string // absolute path to directory containing .txtscape/pages

	mu       sync.Mutex
	activity []activityEntry
}

type activityEntry struct {
	Time string `json:"time"`
	Kind string `json:"kind"` // "create", "update", "delete"
	Path string `json:"path"`
}

func newUIServer(root string) *uiServer {
	return &uiServer{root: root}
}

func (u *uiServer) pagesRoot() string {
	return filepath.Join(u.root, pagesDir)
}

func (u *uiServer) configPath() string {
	return filepath.Join(u.root, configFile)
}

// handler returns the root http.Handler with all routes.
func (u *uiServer) handler() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/config", u.handleAPIConfig)
	mux.HandleFunc("/api/pages/", u.handleAPIPages)
	mux.HandleFunc("/api/pages", u.handleAPIPagesList)
	mux.HandleFunc("/api/search", u.handleAPISearch)
	mux.HandleFunc("/api/events", u.handleAPIEvents)
	mux.HandleFunc("/api/activity", u.handleAPIActivity)

	// Static files — serve the embedded SPA
	staticSub, err := fs.Sub(staticFiles, "ui/static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(staticSub))

	// SPA fallback: serve index.html for unknown paths (client-side routing)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the static file first
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Check if file exists in the embedded FS
		f, err := staticSub.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback — serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	return mux
}

// --- API handlers ---

func (u *uiServer) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := os.ReadFile(u.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"concerns":[]}`))
			return
		}
		http.Error(w, "reading config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Validate it's valid JSON before serving
	var cfg txtscapeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		http.Error(w, "invalid config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// pageTreeEntry represents a page or folder in the tree.
type pageTreeEntry struct {
	Name     string          `json:"name"`
	Path     string          `json:"path,omitempty"`
	IsDir    bool            `json:"isDir"`
	Preview  string          `json:"preview,omitempty"`
	Children []pageTreeEntry `json:"children,omitempty"`
}

func (u *uiServer) handleAPIPagesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root := u.pagesRoot()
	tree := u.buildTree(root, root)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (u *uiServer) buildTree(dir string, root string) []pageTreeEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []pageTreeEntry
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		relPath, _ := filepath.Rel(root, fullPath)
		relPath = filepath.ToSlash(relPath)
		if e.IsDir() {
			children := u.buildTree(fullPath, root)
			result = append(result, pageTreeEntry{
				Name:     e.Name(),
				Path:     relPath,
				IsDir:    true,
				Children: children,
			})
		} else if strings.HasSuffix(e.Name(), ".txt") {
			result = append(result, pageTreeEntry{
				Name:    e.Name(),
				Path:    relPath,
				IsDir:   false,
				Preview: firstLine(fullPath),
			})
		}
	}
	return result
}

func (u *uiServer) handleAPIPages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Extract path after /api/pages/
	pagePath := strings.TrimPrefix(r.URL.Path, "/api/pages/")
	if pagePath == "" {
		u.handleAPIPagesList(w, r)
		return
	}

	clean, err := validatePath(pagePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(u.pagesRoot(), filepath.FromSlash(clean))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}
		http.Error(w, "reading page: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path":    clean,
		"content": string(data),
	})
}

func (u *uiServer) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	queryLower := strings.ToLower(query)
	type searchResult struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Context string `json:"context"`
	}
	var results []searchResult

	filepath.Walk(u.pagesRoot(), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(u.pagesRoot(), path)
		relPath = filepath.ToSlash(relPath)

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), queryLower) {
				// Context: the matching line + 1 before/after
				start := i - 1
				if start < 0 {
					start = 0
				}
				end := i + 2
				if end > len(lines) {
					end = len(lines)
				}
				ctx := strings.Join(lines[start:end], "\n")
				results = append(results, searchResult{
					Path:    relPath,
					Line:    i + 1,
					Context: ctx,
				})
				if len(results) >= 100 {
					return fmt.Errorf("limit")
				}
			}
		}
		return nil
	})

	if results == nil {
		results = []searchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleAPIEvents serves an SSE stream. The client receives events when
// pages or config change. We poll the filesystem to keep zero deps.
func (u *uiServer) handleAPIEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush headers immediately so the client knows the connection is established
	flusher.Flush()

	// Build initial snapshot of file mod times
	snapshot := u.fileSnapshot()
	configMod := u.configModTime()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			newSnapshot := u.fileSnapshot()
			newConfigMod := u.configModTime()

			changes := false

			// Check for new/modified/deleted pages
			for path, modTime := range newSnapshot {
				if oldMod, exists := snapshot[path]; !exists || oldMod != modTime {
					changes = true
					break
				}
			}
			if !changes {
				for path := range snapshot {
					if _, exists := newSnapshot[path]; !exists {
						changes = true
						break
					}
				}
			}

			configChanged := newConfigMod != configMod

			if changes || configChanged {
				kind := "pages"
				if configChanged {
					kind = "config"
				}
				evt := fmt.Sprintf("event: change\ndata: {\"kind\":%q}\n\n", kind)
				fmt.Fprint(w, evt)
				flusher.Flush()

				// Record activity for page changes
				if changes {
					u.recordChanges(snapshot, newSnapshot)
				}

				snapshot = newSnapshot
				configMod = newConfigMod
			}
		}
	}
}

func (u *uiServer) fileSnapshot() map[string]time.Time {
	result := make(map[string]time.Time)
	filepath.Walk(u.pagesRoot(), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		relPath, _ := filepath.Rel(u.pagesRoot(), path)
		relPath = filepath.ToSlash(relPath)
		result[relPath] = info.ModTime()
		return nil
	})
	return result
}

func (u *uiServer) configModTime() time.Time {
	info, err := os.Stat(u.configPath())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (u *uiServer) recordChanges(old, new map[string]time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)

	for path, modTime := range new {
		oldMod, existed := old[path]
		if !existed {
			u.activity = append(u.activity, activityEntry{Time: now, Kind: "create", Path: path})
		} else if oldMod != modTime {
			u.activity = append(u.activity, activityEntry{Time: now, Kind: "update", Path: path})
		}
	}
	for path := range old {
		if _, exists := new[path]; !exists {
			u.activity = append(u.activity, activityEntry{Time: now, Kind: "delete", Path: path})
		}
	}

	// Keep last 200 entries
	if len(u.activity) > 200 {
		u.activity = u.activity[len(u.activity)-200:]
	}
}

func (u *uiServer) handleAPIActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u.mu.Lock()
	activity := make([]activityEntry, len(u.activity))
	copy(activity, u.activity)
	u.mu.Unlock()

	// Reverse so newest first
	for i, j := 0, len(activity)-1; i < j; i, j = i+1, j-1 {
		activity[i], activity[j] = activity[j], activity[i]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activity)
}

// serveUI starts the HTTP server for the SPA.
func serveUI(root string) {
	u := newUIServer(root)

	// Find an available port
	port := "3000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		// Try any available port
		listener, err = net.Listen("tcp", ":0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot start server: %v\n", err)
			os.Exit(1)
		}
	}

	addr := listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d", addr.Port)
	fmt.Printf("txtscape ui: %s\n", url)

	srv := &http.Server{Handler: u.handler()}
	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
