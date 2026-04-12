# txtscape

**Your AI agent forgets everything between conversations. Fix that.**

txtscape is an MCP server that gives your agent persistent, searchable memory — plain `.txt` files committed to git.

[Install](https://txtscape.com) · [Tutorial](https://txtscape.com/tutorial) · [npm](https://www.npmjs.com/package/@txtscape/mcp)

## Without txtscape vs. with

| ❌ Without | ✅ With txtscape |
|---|---|
| "Remind me, are we using Postgres or SQLite?" | "Read the architecture decisions and follow the existing patterns." |
| "What's our error handling pattern again?" | The agent reads your pages and already knows. |
| Every conversation starts from zero. | Knowledge accumulates in your repo. |

## Why txtscape

| | | |
|---|---|---|
| 📁 **Plain text in git** | 🔒 **Zero dependencies** | 🧠 **LLM-native** |
| Diffable, reviewable in PRs, portable | Pure Go stdlib. Nothing to audit | Plain text any model understands |
| 🚫 **No database** | 🏗️ **Configurable structure** | 🔓 **No lock-in** |
| Filesystem is the storage layer | Each project defines its own taxonomy | Stop using it — your `.txt` files stay |
| 🤫 **Stays out of your way** | ⚡ **Stdio subprocess** | 📜 **MIT licensed** |
| Pages live in `.txtscape/`, not scattered across your project | No ports, no background process, no attack surface | Free forever |

## Works with

VS Code (Copilot) · Cursor · Windsurf · Claude Desktop · Claude Code · Zed · JetBrains (via MCP plugin) · Neovim (via MCP plugins) — any MCP-compatible client.

## What it looks like

> **You:** "Record that we chose Postgres over SQLite. Reason: we need concurrent writes and the team already knows it."
>
> **Agent:** `put_page` → `decisions/database-choice.txt` created
>
> *— next day, new conversation —*
>
> **You:** "Add a users table migration."
>
> **Agent:** `search_pages` → found *decisions/database-choice.txt*
> "I see you chose Postgres for concurrent writes. I'll write the migration using PostgreSQL syntax with a serial primary key…"

## Get started

👉 **[Install in 2 minutes](https://txtscape.com)** — pick your editor, paste one config block, done.

👉 **[Try the tutorial](https://txtscape.com/tutorial)** — build a text adventure game in 30 minutes while learning how persistent memory works.

## License

MIT — see [LICENSE.md](LICENSE.md).
