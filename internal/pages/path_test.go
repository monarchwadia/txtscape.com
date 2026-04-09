package pages

import (
	"strings"
	"testing"
)

func TestParsePath_RootFile_AllowHomepage_ReturnsParts(t *testing.T) {
	// Business context: Users must be able to publish files at their root (~user/hello.txt).
	// Scenario: Parse a simple filename with no folder prefix.
	// Expected: FolderPath="/", FileName="hello.txt".
	p, err := ParsePath("hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.FolderPath != "/" {
		t.Errorf("FolderPath: got %q, want %q", p.FolderPath, "/")
	}
	if p.FileName != "hello.txt" {
		t.Errorf("FileName: got %q, want %q", p.FileName, "hello.txt")
	}
}

func TestParsePath_IndexTxt_AllowCustomHomepage_ReturnsParts(t *testing.T) {
	// Business context: index.txt at root is served as the user's homepage at ~username.
	// Scenario: Parse "index.txt".
	// Expected: FolderPath="/", FileName="index.txt".
	p, err := ParsePath("index.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.FolderPath != "/" || p.FileName != "index.txt" {
		t.Errorf("got folder=%q file=%q, want / index.txt", p.FolderPath, p.FileName)
	}
}

func TestParsePath_Nested_AllowOrganizedContent_ReturnsParts(t *testing.T) {
	// Business context: Users organize content in folders like a filesystem.
	// Scenario: Parse "blog/2026/post.txt".
	// Expected: FolderPath="/blog/2026/", FileName="post.txt".
	p, err := ParsePath("blog/2026/post.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.FolderPath != "/blog/2026/" {
		t.Errorf("FolderPath: got %q, want %q", p.FolderPath, "/blog/2026/")
	}
	if p.FileName != "post.txt" {
		t.Errorf("FileName: got %q, want %q", p.FileName, "post.txt")
	}
}

func TestParsePath_Variations(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFolder string
		wantFile   string
		wantErr    bool
		reason     string
	}{
		{"single-folder", "blog/post.txt", "/blog/", "post.txt", false, "one level of nesting"},
		{"max-depth-10", "a/b/c/d/e/f/g/h/i/file.txt", "/a/b/c/d/e/f/g/h/i/", "file.txt", false, "9 folders + 1 file = 10 levels is the max"},
		{"too-deep-11", "a/b/c/d/e/f/g/h/i/j/file.txt", "", "", true, "10 folders + 1 file = 11 levels exceeds limit"},
		{"empty-path", "", "", "", true, "empty path is invalid"},
		{"no-txt-ext", "hello.md", "", "", true, "only .txt files allowed"},
		{"uppercase-file", "Hello.txt", "", "", true, "filenames must be lowercase"},
		{"uppercase-folder", "Blog/post.txt", "", "", true, "folder names must be lowercase"},
		{"long-folder-11", "abcdefghijk/post.txt", "", "", true, "folder names max 10 chars"},
		{"exact-folder-10", "abcdefghij/post.txt", "/abcdefghij/", "post.txt", false, "folder name exactly 10 chars is OK"},
		{"path-traversal", "../secret.txt", "", "", true, "prevent path traversal attacks"},
		{"deep-traversal", "blog/../../etc/passwd.txt", "", "", true, "prevent deep path traversal"},
		{"backslash", `blog\post.txt`, "", "", true, "backslashes not allowed"},
		{"dot-in-folder", "my.folder/post.txt", "", "", true, "dots not allowed in folder names"},
		{"space-in-folder", "my folder/post.txt", "", "", true, "spaces not allowed in folder names"},
		{"hyphen-underscore", "my-blog/my_post.txt", "/my-blog/", "my_post.txt", false, "hyphens and underscores allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("[%s] err=%v, wantErr=%v", tt.reason, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if p.FolderPath != tt.wantFolder {
				t.Errorf("[%s] FolderPath: got %q, want %q", tt.reason, p.FolderPath, tt.wantFolder)
			}
			if p.FileName != tt.wantFile {
				t.Errorf("[%s] FileName: got %q, want %q", tt.reason, p.FileName, tt.wantFile)
			}
		})
	}
}

func TestParsePath_SQLInjection_ParameterizedQueriesProtect_ReturnsError(t *testing.T) {
	// Business context: Even though we use parameterized queries, the validation
	// layer should reject obviously malicious input before it reaches the DB.
	// Scenario: Pass SQL injection attempts as path components.
	// Expected: All rejected by validation regex.
	inputs := []string{
		"'; DROP TABLE pages;--.txt",
		"folder/'; DELETE FROM users;--.txt",
		"1 OR 1=1.txt",
	}
	for _, input := range inputs {
		_, err := ParsePath(input)
		if err == nil {
			t.Errorf("expected error for SQL injection attempt %q, got nil", input)
		}
	}
}

func TestParsePath_LongFileName_PreventAbuse_ReturnsError(t *testing.T) {
	// Business context: Extremely long filenames waste storage and could cause issues.
	// The regex limits to 245 chars + ".txt" = 249 total.
	// Scenario: Filename with 246 chars before .txt.
	// Expected: Returns error.
	name := strings.Repeat("a", 246) + ".txt"
	_, err := ParsePath(name)
	if err == nil {
		t.Fatal("expected error for 246-char filename, got nil")
	}
}

func TestParsePath_MaxFileName_Boundary_ReturnsNil(t *testing.T) {
	// Business context: 245 chars + .txt = 249 total is the max allowed filename.
	// Scenario: Filename with exactly 245 chars before .txt.
	// Expected: Accepted.
	name := strings.Repeat("a", 245) + ".txt"
	p, err := ParsePath(name)
	if err != nil {
		t.Fatalf("expected nil for 245-char filename, got %v", err)
	}
	if p.FileName != name {
		t.Errorf("FileName: got %q, want %q", p.FileName, name)
	}
}
