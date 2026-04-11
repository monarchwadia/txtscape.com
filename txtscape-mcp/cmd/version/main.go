package main

// Usage (run from the txtscape-mcp directory):
//
//	go run ./cmd/version get    — print current version (plaintext, for scripts)
//	go run ./cmd/version check  — verify all files agree, exit 0 or 1
//	go run ./cmd/version set    — interactive: prompt for next version, write all files

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type target struct {
	relPath string
	re      *regexp.Regexp
}

// All files that carry the version string.
var targets = []target{
	{relPath: "main.go", re: regexp.MustCompile(`("version":\s*")(\d+\.\d+\.\d+)(")`)},
	{relPath: "npm/txtscape-mcp/package.json", re: regexp.MustCompile(`("version":\s*")(\d+\.\d+\.\d+)(")`)},
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

type match struct {
	relPath string
	version string
	line    int
}

// readAll reads the version from every target file and returns the match
// with file path, version string, and line number.
func readAll(root string) ([]match, error) {
	var results []match
	for _, t := range targets {
		data, err := os.ReadFile(filepath.Join(root, t.relPath))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.relPath, err)
		}
		loc := t.re.FindIndex(data)
		if loc == nil {
			return nil, fmt.Errorf("%s: version pattern not found", t.relPath)
		}
		sub := t.re.FindSubmatch(data)
		line := 1 + strings.Count(string(data[:loc[0]]), "\n")
		results = append(results, match{relPath: t.relPath, version: string(sub[2]), line: line})
	}
	return results, nil
}

func writeAll(root, version string) error {
	for _, t := range targets {
		path := filepath.Join(root, t.relPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", t.relPath, err)
		}
		updated := t.re.ReplaceAll(data, []byte("${1}"+version+"${3}"))
		if err := os.WriteFile(path, updated, 0o644); err != nil {
			return fmt.Errorf("%s: %w", t.relPath, err)
		}
	}
	return nil
}

func consistent(ms []match) bool {
	for i := 1; i < len(ms); i++ {
		if ms[i].version != ms[0].version {
			return false
		}
	}
	return true
}

func printMismatch(ms []match) {
	fmt.Fprintln(os.Stderr, "version mismatch:")
	for _, m := range ms {
		fmt.Fprintf(os.Stderr, "  %s:%d  %s\n", m.relPath, m.line, m.version)
	}
}

func bumpPatch(v string) string {
	parts := strings.Split(v, ".")
	p, _ := strconv.Atoi(parts[2])
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], p+1)
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: version <get|check|set>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "get":
		cmdGet(root)
	case "check":
		cmdCheck(root)
	case "set":
		cmdSet(root)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\nusage: version <get|check|set>\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdGet(root string) {
	ms, err := readAll(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !consistent(ms) {
		printMismatch(ms)
		os.Exit(1)
	}
	fmt.Println(ms[0].version)
}

func cmdCheck(root string) {
	ms, err := readAll(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !consistent(ms) {
		printMismatch(ms)
		os.Exit(1)
	}
	fmt.Println("versions ok")
}

func cmdSet(root string) {
	ms, err := readAll(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !consistent(ms) {
		printMismatch(ms)
		os.Exit(1)
	}

	current := ms[0].version
	suggested := bumpPatch(current)

	fmt.Printf("current version: %s\n", current)
	fmt.Printf("   next version: [%s] ", suggested)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	next := suggested
	if input != "" {
		next = strings.TrimPrefix(input, "v")
	}
	if !semverRe.MatchString(next) {
		fmt.Fprintf(os.Stderr, "invalid version: %q\n", next)
		os.Exit(1)
	}

	if err := writeAll(root, next); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("updated to %s\n", next)
}
