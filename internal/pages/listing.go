package pages

import (
	"fmt"
	"strings"
)

// FolderEntry represents a file or subfolder in a directory listing.
type FolderEntry struct {
	Name     string
	IsFolder bool
}

// GenerateListing creates a markdown directory listing for a given user/folder.
func GenerateListing(username, folderPath string, entries []FolderEntry) string {
	var buf strings.Builder
	if folderPath == "/" {
		buf.WriteString(fmt.Sprintf("# ~%s\n\n", username))
	} else {
		buf.WriteString(fmt.Sprintf("# ~%s%s\n\n", username, folderPath))
	}
	for _, e := range entries {
		if e.IsFolder {
			buf.WriteString(fmt.Sprintf("- 📁 [%s/](/~%s%s%s/)\n", e.Name, username, folderPath, e.Name))
		} else {
			buf.WriteString(fmt.Sprintf("- 📄 [%s](/~%s%s%s)\n", e.Name, username, folderPath, e.Name))
		}
	}
	return buf.String()
}
