---
name: scan-vulnerabilities
description: "Scan txtscape for security vulnerabilities. USE WHEN: user says 'scan', 'security', 'vulnerabilities', 'audit', 'pentest', 'owasp', 'hardening'. Supports: /scan-vulnerabilities (full scan), /scan-vulnerabilities auth, /scan-vulnerabilities injection, /scan-vulnerabilities infra. Reads actual code, reports findings with severity and fix."
---

# /scan-vulnerabilities

Scan the txtscape codebase for security vulnerabilities. Read [Project.md](../../../Project.md) for full project context.

## Step 1: Determine scope

| User says | Scope | What to scan |
|-----------|-------|-------------|
| `/scan-vulnerabilities` | Full | All categories below |
| `/scan-vulnerabilities auth` | Auth only | Authentication, tokens, sessions |
| `/scan-vulnerabilities injection` | Injection only | SQL, path traversal, header injection |
| `/scan-vulnerabilities infra` | Infra only | Dockerfile, deps, TLS, headers |
| `/scan-vulnerabilities <file>` | Single file | All categories against that file |

## Step 2: Read the actual code

Do NOT guess. Read every file in the scan scope before reporting. Use the Explore subagent for thorough codebase reads when doing a full scan.

Key files to read per category:

| Category | Files |
|----------|-------|
| Auth | `internal/auth/crypto.go`, `internal/auth/store.go`, `internal/auth/validation.go`, `internal/handler/handler.go` (auth flow) |
| Injection | `internal/pages/store.go`, `internal/pages/path.go`, `internal/handler/handler.go` |
| Infra | `Dockerfile`, `go.mod`, `cmd/txtscape/main.go` |
| MCP | `internal/mcp/mcp.go` |

## Step 3: Check each category

### A. Authentication & Authorization

- [ ] Password hashing uses bcrypt with sufficient cost (≥10)
- [ ] Passwords have minimum length enforcement (≥8)
- [ ] Bcrypt 72-byte limit is enforced before hashing
- [ ] Tokens are generated with crypto/rand, not math/rand
- [ ] Token entropy is sufficient (≥32 bytes / 256 bits)
- [ ] Tokens are stored as bcrypt hashes, never plaintext
- [ ] Plaintext token is returned exactly once (on creation), never logged
- [ ] Bearer token extraction is correct (no off-by-one, no panic on empty)
- [ ] Authorization checks run BEFORE any write operation
- [ ] User A cannot write to user B's space (cross-user check)
- [ ] Timing-safe comparison for token validation (bcrypt handles this)
- [ ] Login returns same error for "user not found" and "wrong password" (no enumeration)
- [ ] No token expiration? Flag as accepted risk if intentional (HN-style)

### B. Injection

- [ ] All SQL uses parameterized queries ($1, $2...), never string concatenation
- [ ] Path traversal blocked: `..` rejected, backslash rejected
- [ ] Filename regex anchored with `^` and `$`
- [ ] Folder path regex anchored with `^` and `$`
- [ ] User input never interpolated into SQL identifiers (table/column names)
- [ ] `LIKE` patterns use user input safely (no unescaped `%` or `_` from user)
- [ ] HTTP header injection: user input never written to response headers
- [ ] Response content-type is set explicitly, not inferred

### C. Input Validation & Limits

- [ ] Request body size limited (100KB for pages)
- [ ] Body read uses `io.LimitReader`, not unbounded `io.ReadAll`
- [ ] Empty body rejected (prevents zero-byte writes)
- [ ] Username validation rejects special characters, null bytes
- [ ] Folder depth limit enforced (≤10)
- [ ] Files-per-folder limit enforced (≤100)
- [ ] Subfolders-per-folder limit enforced (≤10)
- [ ] Limits checked in a transaction (no TOCTOU race)

### D. HTTP & Transport

- [ ] Sensitive endpoints use POST, not GET (signup, login)
- [ ] No credentials in URL query strings
- [ ] Content-Type set on all responses
- [ ] No stack traces or internal errors leaked to client
- [ ] Error messages are generic ("internal error"), not detailed
- [ ] No CORS headers (or intentionally restrictive if present)
- [ ] No sensitive data in logs (passwords, tokens)

### E. Infrastructure & Dependencies

- [ ] Dockerfile uses multi-stage build (no compiler in final image)
- [ ] Final image is minimal (alpine, scratch, or distroless)
- [ ] `CGO_ENABLED=0` for static binary
- [ ] ca-certificates installed (for HTTPS fetch in MCP surf)
- [ ] Dependencies pinned to specific versions in go.mod
- [ ] No known CVEs in dependency versions (check go.sum)
- [ ] DATABASE_URL not hardcoded
- [ ] No secrets in source code

### F. MCP surf() Tool

- [ ] Only HTTPS URLs accepted (no http://, file://, ftp://)
- [ ] Response body size limited (prevents memory exhaustion)
- [ ] HTTP client has timeout set
- [ ] No follow-redirects to non-HTTPS (or redirects limited)
- [ ] User-Agent set (identifies the client)
- [ ] SSRF mitigation: consider whether to block private IPs (10.x, 127.x, 169.254.x)

## Step 4: Report findings

Output a table sorted by severity:

```
## Scan Results

| # | Severity | Category | Finding | File:Line | Fix |
|---|----------|----------|---------|-----------|-----|
| 1 | HIGH | MCP | No SSRF protection — surf() can fetch private IPs | mcp.go:180 | Block RFC1918/loopback ranges before fetch |
| 2 | MEDIUM | HTTP | No rate limiting on /signup and /login | main.go:55 | Add middleware or reverse proxy rate limit |
| 3 | LOW | Infra | No health check endpoint | main.go | Add GET /health for load balancer probes |
| 4 | INFO | Auth | Tokens never expire | handler.go:130 | Accepted risk (HN-style) — document in spec |

### Summary
- HIGH: 1
- MEDIUM: 2
- LOW: 1
- INFO: 1
```

### Severity definitions

| Severity | Meaning |
|----------|---------|
| **CRITICAL** | Exploitable now, data breach or RCE possible |
| **HIGH** | Exploitable with modest effort, significant impact |
| **MEDIUM** | Requires specific conditions, moderate impact |
| **LOW** | Minor issue, defense-in-depth concern |
| **INFO** | Not a vulnerability, but worth noting (accepted risk, missing hardening) |

## Step 5: Offer to fix

After reporting, ask:

> Want me to fix any of these? I can address them one at a time, starting with the highest severity.

For each fix, follow the `/write` skill methodology — write a failing test for the vulnerability first, then implement the fix.
