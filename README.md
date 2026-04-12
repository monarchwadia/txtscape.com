# txtscape

Persistent project memory for AI agents.

## What This Is

An MCP server that gives your AI agent a structured, searchable memory that lives in your repo. Plain `.txt` pages, committed to git, organized however your project needs.

Your agent remembers decisions, patterns, and context across conversations. The knowledge stays in the repo — versioned, diffable, reviewable in PRs — not locked in a chat history that disappears.

## Why txtscape

- **Zero dependencies** — pure Go standard library. Nothing to audit, nothing to break.
- **No database** — the filesystem is the storage layer. Plain `.txt` files in `.txtscape/pages/`.
- **Plain text in git** — diffable, reviewable in PRs, portable across tools.
- **Configurable taxonomy** — each project defines its own memory structure via `config.json`.
- **No lock-in** — stop using txtscape and your `.txt` files stay. They're just files.
- **Stays out of your way** — pages live in `.txtscape/`, not scattered markdown files across your project. Add a `.ignore` file to exclude them from code search.
- **LLM-native** — plain text that any model already understands. No special format to parse.
- **Stdio subprocess** — your IDE launches it, talks over stdin/stdout, and it exits when done. No ports, no background process, no attack surface.
- **MIT licensed**

### Works with

VS Code (Copilot) · Cursor · Windsurf · Claude Desktop · Claude Code · Zed · JetBrains (via MCP plugin) · Neovim (via MCP plugins) — any MCP-compatible client.

## Install

TBD — installation instructions coming soon.

## How It Works

txtscape-mcp runs as a stdio MCP server. Your IDE launches it as a subprocess and communicates via JSON-RPC over stdin/stdout. Pages are stored as plain `.txt` files in `.txtscape/pages/` in your repo.

```
.txtscape/
├── config.json          # optional: define your project's memory structure
└── pages/
    ├── decisions/
    │   └── use-postgres.txt
    ├── architecture/
    │   └── api-design.txt
    └── learnings/
        └── auth-gotchas.txt
```

### Concerns

The optional `config.json` defines "concerns" — named folders with descriptions and templates. Your agent sees this taxonomy at session start and knows where to put things. Every project can define its own shape.

### Tools

11 MCP tools:

| Tool | Description |
|------|-------------|
| `get_page` | Read a page (returns content + hash for concurrency) |
| `put_page` | Create or update a page |
| `append_page` | Append to an existing page |
| `delete_page` | Delete a page |
| `move_page` | Rename or move a page |
| `str_replace_page` | Surgical find-and-replace edit |
| `list_pages` | Browse folders (with first-line previews) |
| `search_pages` | Full-text or regex search across all pages |
| `snapshot` | Bulk-read all pages in a subtree |
| `related_pages` | Find cross-references (incoming + outgoing) |
| `page_history` | Git commit history for a page |

## Build

```
make mcp            # build txtscape-mcp binary
make mcp-test       # run all tests (unit + integration + e2e)
make mcp-test-unit  # unit tests only
```

## Landing Page

The `txtscape.com` site is a simple landing page describing the project:

```
make build  # build the landing page server
make dev    # run locally on :8080
```

## License

MIT — see [LICENSE.txt](LICENSE.txt).
