# txtscape — Project Context Document

## What It Is

txtscape is a local MCP server that gives AI agents persistent, structured project memory. Pages are plain `.txt` files stored in `.txtscape/pages/` and committed to git.

The domain txtscape.com hosts a landing page describing the project.

## The Problem

AI agents lose context between conversations. Every session starts from scratch — re-reading code, re-discovering architecture, re-learning project conventions. Chat history disappears. There is no durable, structured memory that agents can read and write across sessions.

## The Solution

A local MCP server (`txtscape-mcp`) that manages a `.txtscape/` directory in the project root. Agents write decisions, architecture notes, learnings, and plans as `.txt` pages. The pages are:

- **Persistent** — committed to git, survive across sessions
- **Structured** — organized into configurable "concern" folders
- **Searchable** — full-text and regex search across all pages
- **Diffable** — plain text in git, reviewable in PRs
- **Cross-referenced** — related_pages discovers incoming and outgoing links

## Architecture

### Storage

Plain `.txt` files in `.txtscape/pages/`. No database. The filesystem is the storage layer.

Constraints:
- Files must end in `.txt`
- 1MB maximum per file
- Max 10 levels of folder nesting
- Folder names: max 50 characters, lowercase alphanumeric plus `-` and `_`

### Transport

stdio only. The IDE/agent launches `txtscape-mcp` as a subprocess and communicates via JSON-RPC over stdin/stdout. No HTTP server, no ports, no daemon.

### Concerns Config

Optional `.txtscape/config.json` defines a project's memory taxonomy:

```json
{
  "concerns": [
    {
      "folderName": "decisions",
      "label": "Architecture Decisions",
      "description": "Record technical and product choices with rationale.",
      "template": "# {title}\n\n## Context\n...\n\n## Decision\n...\n\n## Consequences\n..."
    }
  ]
}
```

On `initialize`, concern descriptions are appended to the MCP instructions string. Agents see the taxonomy automatically.

### Tools

11 MCP tools:

**CRUD**: `get_page`, `put_page`, `append_page`, `delete_page`, `move_page`
**Discovery**: `list_pages`, `search_pages`, `related_pages`
**Editing**: `str_replace_page`
**Bulk**: `snapshot`, `page_history`

All mutation tools support optimistic concurrency via SHA-256 content hashing (`expected_hash` parameter).

## Technical Stack

- **Language**: Go (standard library only, zero external dependencies)
- **Transport**: stdio JSON-RPC (MCP protocol)
- **Storage**: filesystem (`.txtscape/pages/`)
- **Version control**: git (pages are committed alongside code)

## Landing Page

`txtscape.com` is a simple Go HTTP server serving a single `content/index.html` page. Deployed on Railway. No database, no auth, no API.
