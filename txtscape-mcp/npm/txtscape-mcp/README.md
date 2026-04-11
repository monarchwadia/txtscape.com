# @txtscape/mcp

Persistent project memory for AI agents — committable, structured, and zero-dependency.

## Install

**Option A: use with npx (no install needed)**

Configure your MCP host to run txtscape-mcp directly via npx:

**VS Code** (`.vscode/mcp.json`):
```json
{
  "servers": {
    "txtscape": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@txtscape/mcp"],
      "cwd": "${workspaceFolder}"
    }
  }
}
```

**Option B: install globally**

```sh
npm install -g @txtscape/mcp
```

Then configure your MCP host:

**VS Code** (`.vscode/mcp.json`):
```json
{
  "servers": {
    "txtscape": {
      "type": "stdio",
      "command": "txtscape-mcp",
      "cwd": "${workspaceFolder}"
    }
  }
}
```

**Cursor / Windsurf** (`.cursor/mcp.json` or `.windsurf/mcp.json`):
```json
{
  "mcpServers": {
    "txtscape": {
      "command": "txtscape-mcp",
      "args": []
    }
  }
}
```

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "txtscape": {
      "command": "txtscape-mcp",
      "args": []
    }
  }
}
```

## What it does

txtscape-mcp gives AI agents 11 tools to manage a persistent knowledge base stored as plain `.txt` files in `.txtscape/pages/`. Files are committed to git alongside your code.

| Tool | Description |
|---|---|
| `put_page` | Create or replace a page |
| `get_page` | Read a page |
| `str_replace_page` | Edit a page surgically |
| `list_pages` | Browse directory with previews |
| `search_pages` | Full-text search |
| `delete_page` | Delete a page |
| `rename_page` | Move or rename a page |
| `snapshot` | Load all pages in one call |
| `related_pages` | Find cross-referenced pages |
| `page_history` | Inspect git commit history |
| `mcp_append_page` | Append content to a page |

## Supported platforms

- macOS ARM64 (Apple Silicon)
- macOS x64
- Linux x64
- Linux ARM64
- Windows x64

All platform binaries are bundled in this package. The correct one is selected automatically at runtime.
