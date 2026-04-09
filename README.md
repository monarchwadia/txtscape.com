# txtscape

A decentralized, agent-readable web of plain text.

## Run

```
make build
DATABASE_URL=postgres://... bin/txtscape
```

MCP mode (stdio JSON-RPC):
```
bin/txtscape mcp
```

## Test

```
make test          # all (unit + integration + e2e)
make test-unit     # no DB required
make test-integration
make test-e2e
```

Integration and e2e need Postgres. Default `DATABASE_URL`: `postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable`

## Deploy

```
docker build -t txtscape .
docker run -e DATABASE_URL=postgres://... -p 8080:8080 txtscape
```

Run migrations first:
```
psql $DATABASE_URL -f migrations/001_init.sql
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /signup | — | `username=x&password=y` → `{"token":"..."}` |
| POST | /login | — | `username=x&password=y` → `{"token":"..."}` |
| PUT | /~user/path.txt | Bearer | Create/update page |
| DELETE | /~user/path.txt | Bearer | Delete page |
| GET | /~user/path.txt | — | Read page |
| GET | /~user | — | index.txt or directory listing |

## Structure

```
cmd/txtscape/       main.go — HTTP server + MCP mode
internal/auth/      validation, crypto, user/token stores
internal/pages/     path parsing, listings, page store
internal/handler/   HTTP handlers
internal/mcp/       surf(url) MCP tool
migrations/         SQL schema
content/            static .txt pages
e2e/                journey tests (-tags=e2e)
tests/integration/  endpoint tests (-tags=integration)
```

## /write skill

This repo has a Copilot skill at `.github/skills/write/SKILL.md`. Use `/write` in Copilot chat to invoke it.

| Command | What it does |
|---------|-------------|
| `/write tests` | Auto-detect test level from open file, write tests |
| `/write unit tests` | Unit tests for the open file |
| `/write integration tests` | Single-endpoint tests against real Postgres |
| `/write e2e` | Multi-step journey tests |
| `/write handler` | Production handler code |
| `/write plan` | Describe what it would do without acting |
| `/write what's missing` | Gap analysis — scan code vs tests, find holes |

Follows red/green TDD: one failing test → minimum code to pass → repeat. See `references/methodology.md` for details.

## /scan-vulnerabilities skill

Security scanning skill at `.github/skills/scan-vulnerabilities/SKILL.md`. Use `/scan-vulnerabilities` in Copilot chat.

| Command | What it does |
|---------|-------------|
| `/scan-vulnerabilities` | Full scan — all categories |
| `/scan-vulnerabilities auth` | Auth, tokens, password hashing |
| `/scan-vulnerabilities injection` | SQL injection, path traversal |
| `/scan-vulnerabilities infra` | Dockerfile, deps, secrets |
| `/scan-vulnerabilities <file>` | All checks against one file |

Reads actual code, reports findings as a severity-sorted table, offers to fix starting from highest severity.
