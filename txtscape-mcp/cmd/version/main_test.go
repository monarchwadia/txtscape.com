package main

import (
	"os"
	"path/filepath"
	"testing"
)

// setupVersionDir creates a temp directory with fake main.go and package.json
// containing the given version strings.
func setupVersionDir(t *testing.T, goVersion, npmVersion string) string {
	t.Helper()
	root := t.TempDir()

	goContent := `package main
var info = map[string]any{
	"name":    "txtscape",
	"version": "` + goVersion + `",
}
`
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(goContent), 0o644); err != nil {
		t.Fatal(err)
	}

	npmDir := filepath.Join(root, "npm", "txtscape-mcp")
	os.MkdirAll(npmDir, 0o755)
	npmContent := `{
    "name": "@txtscape/mcp",
    "version": "` + npmVersion + `"
}
`
	if err := os.WriteFile(filepath.Join(npmDir, "package.json"), []byte(npmContent), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestReadAll_ConsistentVersions_ReturnsBothMatches(t *testing.T) {
	// Business context: readAll must find the version in every target file so
	// we can compare them for consistency.
	// Scenario: Both files have the same version.
	// Expected: Two matches, both with version "1.2.3" and correct line numbers.
	root := setupVersionDir(t, "1.2.3", "1.2.3")

	ms, err := readAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("got %d matches, want 2", len(ms))
	}
	if ms[0].version != "1.2.3" {
		t.Errorf("main.go version = %q, want 1.2.3", ms[0].version)
	}
	if ms[1].version != "1.2.3" {
		t.Errorf("package.json version = %q, want 1.2.3", ms[1].version)
	}
	if ms[0].line < 1 {
		t.Errorf("main.go line = %d, want >= 1", ms[0].line)
	}
	if ms[1].line < 1 {
		t.Errorf("package.json line = %d, want >= 1", ms[1].line)
	}
}

func TestReadAll_MissingFile_ReturnsError(t *testing.T) {
	// Business context: If a target file is missing, readAll should fail clearly
	// rather than silently ignoring it.
	// Scenario: Only main.go exists, package.json is missing.
	// Expected: Error mentioning the missing file.
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte(`"version": "1.0.0"`), 0o644)

	_, err := readAll(root)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadAll_NoVersionPattern_ReturnsError(t *testing.T) {
	// Business context: If a file exists but doesn't contain the version pattern,
	// we should fail rather than silently skip it.
	// Scenario: main.go has no version string.
	// Expected: Error mentioning the pattern was not found.
	root := setupVersionDir(t, "1.0.0", "1.0.0")
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644)

	_, err := readAll(root)
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestConsistent_AllSame_ReturnsTrue(t *testing.T) {
	// Business context: consistent() gates the set and check commands.
	// Scenario: All matches have the same version.
	// Expected: Returns true.
	ms := []match{
		{version: "1.0.0"},
		{version: "1.0.0"},
	}
	if !consistent(ms) {
		t.Error("expected consistent to return true")
	}
}

func TestConsistent_Different_ReturnsFalse(t *testing.T) {
	// Business context: Mismatched versions must be caught before release.
	// Scenario: Two matches have different versions.
	// Expected: Returns false.
	ms := []match{
		{version: "1.0.0"},
		{version: "2.0.0"},
	}
	if consistent(ms) {
		t.Error("expected consistent to return false")
	}
}

func TestConsistent_Single_ReturnsTrue(t *testing.T) {
	// Business context: Edge case — only one file.
	// Scenario: Single match.
	// Expected: Returns true (no mismatch possible).
	ms := []match{{version: "1.0.0"}}
	if !consistent(ms) {
		t.Error("expected consistent to return true for single match")
	}
}

func TestBumpPatch_IncrementsLastPart(t *testing.T) {
	// Business context: bumpPatch provides the default suggestion in interactive set.
	// Scenario: Patch version is bumped.
	// Expected: 1.2.3 → 1.2.4.
	if got := bumpPatch("1.2.3"); got != "1.2.4" {
		t.Errorf("bumpPatch(1.2.3) = %q, want 1.2.4", got)
	}
}

func TestBumpPatch_ZeroPatch_BumpsToOne(t *testing.T) {
	// Business context: Initial versions often start at x.x.0.
	// Scenario: Patch is 0.
	// Expected: 0.0.0 → 0.0.1.
	if got := bumpPatch("0.0.0"); got != "0.0.1" {
		t.Errorf("bumpPatch(0.0.0) = %q, want 0.0.1", got)
	}
}

func TestWriteAll_UpdatesBothFiles(t *testing.T) {
	// Business context: writeAll must update every target file atomically
	// so a partial write doesn't leave versions inconsistent.
	// Scenario: Both files start at 1.0.0, write 2.0.0.
	// Expected: Both files read back as 2.0.0.
	root := setupVersionDir(t, "1.0.0", "1.0.0")

	if err := writeAll(root, "2.0.0"); err != nil {
		t.Fatal(err)
	}

	ms, err := readAll(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.version != "2.0.0" {
			t.Errorf("%s version = %q, want 2.0.0", m.relPath, m.version)
		}
	}
}

func TestWriteAll_PreservesFileContent(t *testing.T) {
	// Business context: writeAll should only change the version, not rewrite
	// other content in the file.
	// Scenario: main.go has code around the version string.
	// Expected: Non-version content is preserved after write.
	root := setupVersionDir(t, "1.0.0", "1.0.0")

	if err := writeAll(root, "3.5.0"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	content := string(data)
	if got := "package main"; !contains(content, got) {
		t.Errorf("main.go lost non-version content")
	}
	if !contains(content, `"version": "3.5.0"`) {
		t.Errorf("main.go version not updated to 3.5.0")
	}
}

func TestReadAll_ReportsCorrectLineNumber(t *testing.T) {
	// Business context: Line numbers are shown in mismatch output for debugging.
	// Scenario: Version is on line 4 of main.go.
	// Expected: match.line == 4.
	root := setupVersionDir(t, "1.0.0", "1.0.0")

	ms, err := readAll(root)
	if err != nil {
		t.Fatal(err)
	}
	// In setupVersionDir, the version is on line 4 of main.go
	if ms[0].line != 4 {
		t.Errorf("main.go line = %d, want 4", ms[0].line)
	}
	// In package.json, the version is on line 3
	if ms[1].line != 3 {
		t.Errorf("package.json line = %d, want 3", ms[1].line)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
