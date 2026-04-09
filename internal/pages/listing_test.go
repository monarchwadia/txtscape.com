package pages

import "testing"

func TestGenerateListing_RootWithFiles_AgentReadable_ReturnsMarkdown(t *testing.T) {
	// Business context: When a user has no index.txt, ~username serves an auto-generated
	// directory listing in markdown so agents can discover the user's content.
	// Scenario: Root folder with two files and one subfolder.
	// Expected: Markdown with links to each entry.
	entries := []FolderEntry{
		{Name: "blog", IsFolder: true},
		{Name: "hello.txt", IsFolder: false},
		{Name: "about.txt", IsFolder: false},
	}
	got := GenerateListing("alice", "/", entries)

	want := "# ~alice\n\n" +
		"- [blog/](https://txtscape.com/~alice/blog/)\n" +
		"- [hello.txt](https://txtscape.com/~alice/hello.txt)\n" +
		"- [about.txt](https://txtscape.com/~alice/about.txt)\n"

	if got != want {
		t.Errorf("listing mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateListing_Subfolder_NestedNavigation_ReturnsCorrectURLs(t *testing.T) {
	// Business context: Agents browsing into a subfolder need correct URLs
	// to navigate deeper or access files within that subfolder.
	// Scenario: Listing for /blog/ with nested content.
	// Expected: URLs include the full path prefix.
	entries := []FolderEntry{
		{Name: "2026", IsFolder: true},
		{Name: "intro.txt", IsFolder: false},
	}
	got := GenerateListing("alice", "/blog/", entries)

	want := "# ~alice/blog/\n\n" +
		"- [2026/](https://txtscape.com/~alice/blog/2026/)\n" +
		"- [intro.txt](https://txtscape.com/~alice/blog/intro.txt)\n"

	if got != want {
		t.Errorf("listing mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateListing_Empty_NewUser_ReturnsHeaderOnly(t *testing.T) {
	// Business context: A new user with no content should still get a valid listing.
	// Scenario: Empty folder.
	// Expected: Just the header, no entries.
	got := GenerateListing("newuser", "/", nil)
	want := "# ~newuser\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
