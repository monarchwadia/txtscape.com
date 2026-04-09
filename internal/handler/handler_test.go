package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractBearer_Valid_ParseHeader_ReturnsToken(t *testing.T) {
	token := extractBearer("Bearer abc123")
	if token != "abc123" {
		t.Fatalf("got %q, want %q", token, "abc123")
	}
}

func TestExtractBearer_Empty_MissingAuth_ReturnsEmpty(t *testing.T) {
	token := extractBearer("")
	if token != "" {
		t.Fatalf("got %q, want empty", token)
	}
}

func TestExtractBearer_NoBearerPrefix_MalformedAuth_ReturnsEmpty(t *testing.T) {
	token := extractBearer("Basic abc123")
	if token != "" {
		t.Fatalf("got %q, want empty", token)
	}
}

func TestExtractBearer_CaseInsensitive_FlexibleParsing_ReturnsToken(t *testing.T) {
	token := extractBearer("bearer abc123")
	if token != "abc123" {
		t.Fatalf("got %q, want %q", token, "abc123")
	}
}

func TestWriteError_BadRequest_ClientError_Returns400JSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}

	var resp jsonError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Error != "test error" {
		t.Fatalf("error = %q, want %q", resp.Error, "test error")
	}
}

func TestWriteJSON_Created_SuccessResponse_Returns201(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, jsonToken{Token: "tok123"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp jsonToken
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Token != "tok123" {
		t.Fatalf("token = %q, want %q", resp.Token, "tok123")
	}
}

func TestParseTildePath_Variations(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantUser    string
		wantRawPath string
	}{
		{"root user", "/~alice", "alice", ""},
		{"root user trailing slash", "/~alice/", "alice", ""},
		{"file at root", "/~alice/hello.txt", "alice", "hello.txt"},
		{"nested file", "/~alice/blog/post.txt", "alice", "blog/post.txt"},
		{"deep path", "/~alice/a/b/c/d.txt", "alice", "a/b/c/d.txt"},
		{"folder listing", "/~alice/blog/", "alice", "blog/"},
		{"empty", "/~", "", ""},
		{"no tilde", "/alice/hello.txt", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, rawPath := parseTildePath(tt.path)
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if rawPath != tt.wantRawPath {
				t.Errorf("rawPath = %q, want %q", rawPath, tt.wantRawPath)
			}
		})
	}
}
