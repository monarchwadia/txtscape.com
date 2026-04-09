# txtscape — Project Context Document

## What It Is

txtscape is a protocol and platform for a decentralized, agent-readable web of plain text content. Think Gopher (the 1991 pre-web internet protocol) rebuilt on top of existing HTTPS infrastructure, designed for AI agents rather than human browsers, with markdown as the content format.

The domain txtscape.com has been purchased.

## The Problem

The human web is hostile to AI agents: JavaScript rendering, CAPTCHAs, cookie banners, popups, paywalls, anti-bot measures. Agents that need to browse the web must use heavyweight browser automation (Playwright, Puppeteer) or fragile HTML-to-markdown scrapers. There is no web designed for agents to read and navigate natively.

## The Solution

A parallel web of plain text. Publishers drop markdown .txt files on any HTTPS host (or on txtscape.com itself). Files contain standard markdown links to other .txt files. Agents navigate this web by fetching URLs and following links — the agent is the browser.

## Core Protocol

### Reading (the MCP server — "surfboard")

One MCP tool: `surf(url: string) → string`

- Agent passes any HTTPS URL pointing to a .txt file
- Server fetches it and returns the raw text
- Text is markdown with standard markdown links to other .txt files
- Agent reads content, decides which link to follow, calls surf() again
- The MCP server is stateless and minimal (~50 lines of Go)
- It does no parsing, rendering, or transformation — just fetches and returns
- All navigation logic lives in the agent

### Writing (the publish API on txtscape.com)

Authentication: Username + password, HN-style. No email, no OAuth, no verification.

Sign up:
```
POST https://txtscape.com/signup
Content-Type: application/x-www-form-urlencoded

username=alice&password=supersecret
→ {"token": "abc123..."}
```

Log in:
```
POST https://txtscape.com/login
Content-Type: application/x-www-form-urlencoded

username=alice&password=supersecret
→ {"token": "abc123..."}
```

Publish a page:
```
PUT https://txtscape.com/~username/page-name.txt
Authorization: Bearer <token>
Content-Type: text/plain

# My Page Title
Some content with [a link](https://txtscape.com/~otheruser/page.txt)
```

Delete a page:
```
DELETE https://txtscape.com/~username/page-name.txt
Authorization: Bearer <token>
```

Constraints:
- Only .txt files allowed
- 100KB maximum per file
- Max 10 levels of folder nesting
- Folder names max 10 characters, lowercase alphanumeric plus `-` and `_`
- Max 10 subfolders per folder
- Max 100 files per folder

### URL Structure & Directory Listings

- `txtscape.com/index.txt` — The site's own homepage
- `txtscape.com/~username` — Serves `index.txt` if it exists, otherwise returns an auto-generated directory listing of the user's root folder
- `txtscape.com/~username/index.txt` — Same content as `~username` when index.txt exists
- `txtscape.com/~username/path/to/file.txt` — User's pages, up to 10 levels deep
- The `~username` convention is a deliberate Gopher/Unix callback

When a user has no `index.txt` (or deletes it), `~username` returns a directory listing in markdown — a list of links to the user's files and subfolders, readable by agents.

### Storage

Postgres. All files stored as rows.

```sql
CREATE TABLE users (
    username      TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tokens (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      TEXT NOT NULL REFERENCES users(username),
    label         TEXT NOT NULL,        -- e.g. 'cli', 'laptop', 'my-agent'
    token_hash    TEXT NOT NULL UNIQUE,  -- bcrypt hash of the random hex token
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pages (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username    TEXT NOT NULL REFERENCES users(username),
    folder_path TEXT NOT NULL,  -- '/' or '/blog/' or '/blog/2026/'
    file_name   TEXT NOT NULL,  -- 'post.txt'
    contents    TEXT NOT NULL,
    size_bytes  INT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (username, folder_path, file_name),

    CHECK (file_name ~ '^[a-z0-9_-]{1,245}\.txt$'),
    CHECK (folder_path ~ '^(/[a-z0-9_-]{1,10}){0,9}/$' OR folder_path = '/'),
    CHECK (size_bytes > 0 AND size_bytes <= 102400)
);

CREATE INDEX idx_pages_folder ON pages (username, folder_path);
```

Token flow: on signup or explicit token creation, server generates a random hex string, returns it once to the user in plaintext, stores only the bcrypt hash. On each authenticated request, bcrypt-compare the Bearer token against stored hashes. Bcrypt is intentionally slow (~100ms per compare), but tokens are only checked on write operations (PUT/DELETE), which are infrequent — so this is fine.

Auto-generated directory listing example (when no index.txt exists):
```
# ~alice

- [blog/](https://txtscape.com/~alice/blog/)
- [hello.txt](https://txtscape.com/~alice/hello.txt)
- [about.txt](https://txtscape.com/~alice/about.txt)
```

Limits enforced in application code before INSERT:
- Count files in folder ≤ 100
- Count distinct child subfolders ≤ 10
- Nesting depth ≤ 10 (enforced by folder_path regex)

### Content Format

- Files are plain markdown served as .txt over HTTPS
- Navigation is via standard markdown links: `[link text](https://example.com/page.txt)`
- Links can point to .txt files anywhere on the web, not just txtscape.com
- No special markup, no frontmatter, no metadata format required
- The format IS the protocol — if you can write markdown, you can publish

## Key Properties

- One MCP tool: surf(url)
- Content format: markdown .txt files over HTTPS
- Navigation: standard markdown links
- Infrastructure: the existing web (no new protocol, no custom servers)
- Agent is the browser
- Publishing is as simple as a PUT request
- The MCP server is stateless — it just fetches URLs

## The txtscape.com Website

The site itself is built in the format it promotes — every page is a .txt file. This is the demo.

Pages to build:

1. **index.txt** — Homepage. What this is, why it exists, how to get started, MCP server config. Links to spec and demo content. This is what an agent hits first.

2. **spec.txt** — The protocol specification. Content format, link conventions, the surf() MCP tool contract, the publish API, what a conforming page looks like. Includes quickstart.

3. **~txtscape/** — One demo user site showing what published content looks like in practice.

## Competitive Landscape

Nothing exactly like this exists. Related concepts:

- **llms.txt** — Convention where websites publish /llms.txt for LLMs. Same format idea but single file per site, not a navigable web of linked pages.
- **Gemini protocol** (2019) — Most spiritually similar. Modern Gopher successor, text-focused, minimal. But uses a new protocol (gemini://) rather than HTTPS, and targets humans not agents.
- **Gopher** (1991) — The original inspiration. Text menus, navigable, decentralized. Dead for humans.
- **MCP Fetch servers** (server-fetch, fetch-mcp, etc.) — Existing MCP tools that fetch URLs and convert HTML to markdown. They're plumbing that could power something like this, but lack the key idea: a purpose-built network of interlinked .txt files designed for agent navigation.

What makes txtscape distinct: agent-native content format + navigable link graph + agent-as-browser + zero new infrastructure + publisher-first simplicity. No existing project combines all of these.

## Technical Components to Build

Single Go binary, deployed on Railway with Railway Postgres.

1. **The server** — One Go binary that does everything:
   - MCP server: surf(url) tool — fetches any .txt URL over HTTPS, returns raw text
   - Auth: signup, login, token creation (bcrypt passwords, bcrypt token hashes)
   - Publish API: PUT/DELETE .txt files with Bearer token auth
   - Serving: reads pages from Postgres, serves at txtscape.com/~username/path.txt
   - Directory listings: auto-generated markdown when no index.txt exists
   - Limits: 100KB per file, folder count/depth limits enforced in app

2. **The website content** — index.txt, spec.txt, one demo user site.

## The Key Demo

"Give your agent the txtscape MCP server and tell it to go to txtscape.com/index.txt. It'll figure out the rest."

An agent should be able to surf index.txt, follow links to the spec, and — if it has a publish token — start publishing on its own. The site bootstraps itself through the agent.
