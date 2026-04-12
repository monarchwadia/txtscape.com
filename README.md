# txtscape

**Your AI agent forgets everything between conversations. Fix that.**

txtscape is an MCP server that gives your agent persistent, searchable memory — plain `.txt` files committed to git.

[Install](https://txtscape.com) · [Tutorial](https://txtscape.com/tutorial) · [npm](https://www.npmjs.com/package/@txtscape/mcp)

## Without txtscape vs. with

| ❌ Without | ✅ With txtscape |
|---|---|
| "Remind me, are we using Postgres or SQLite?" | "Read the architecture decisions and follow the existing patterns." |
| "What's our error handling pattern again?" | The agent reads your pages and already knows. |
| Random `.md` files proliferating across your project | Everything in one `.txtscape/` folder — your tree stays clean |
| Every conversation starts from zero. | Knowledge accumulates in your repo. |

## Why txtscape

<table>
<tr>
<td valign="top" align="left">📁 <strong>Plain text in git</strong><hr>Diffable, reviewable in PRs, and portable across any tool. Your agent's memory gets the same code review process as your code.</td>
<td valign="top" align="left">🔒 <strong>Zero dependencies</strong><hr>Pure Go standard library. One binary, nothing to audit, no supply chain to worry about.</td>
<td valign="top" align="left">🧠 <strong>LLM-native</strong><hr>Plain text with markdown formatting that any model already understands. No custom schema, no parsing layer.</td>
</tr>
<tr>
<td valign="top" align="left">🚫 <strong>No database</strong><hr>The filesystem is the storage layer. No install, no migrations, no process to keep running.</td>
<td valign="top" align="left">🏗️ <strong>Configurable structure</strong><hr>Define your own taxonomy — <code>decisions/</code>, <code>runbooks/</code>, <code>architecture/</code> — with optional templates so every page follows the same shape.</td>
<td valign="top" align="left">🔓 <strong>No lock-in</strong><hr>Stop using txtscape and your <code>.txt</code> files stay right where they are. They're just files in your repo.</td>
</tr>
<tr>
<td valign="top" align="left">✨ <strong>No file sprawl</strong><hr>Everything lives in one <code>.txtscape/</code> folder. No random markdown files cluttering your project tree or polluting search results.</td>
<td valign="top" align="left">⚡ <strong>Stdio subprocess</strong><hr>Your IDE launches it, talks over stdin/stdout, and it exits when done. No ports to open, no daemon to manage, no attack surface.</td>
<td valign="top" align="left">📜 <strong>MIT licensed</strong><hr>Free and open source forever. Use it however you want, commercially or otherwise.</td>
</tr>
</table>

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
