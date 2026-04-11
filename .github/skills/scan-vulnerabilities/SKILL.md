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
| Injection | `txtscape-mcp/main.go` (path validation, file operations) |
| Infra | `Dockerfile`, `go.mod`, `cmd/txtscape/main.go` |
| MCP | `txtscape-mcp/main.go` (all tool handlers) |

## Step 3: Check each category

### A. Path Traversal & File Safety

- [ ] Path traversal blocked: `..` rejected, backslash rejected
- [ ] Filename regex anchored with `^` and `$`
- [ ] Folder path regex anchored with `^` and `$`
- [ ] File operations stay within `.txtscape/pages/` (no escape to parent)
- [ ] Symlinks not followed outside allowed directory

### B. Input Validation & Limits

- [ ] File size limited (1MB max)
- [ ] Empty content rejected (prevents zero-byte writes)
- [ ] Folder depth limit enforced (≤10)
- [ ] Path validation rejects null bytes, control characters
- [ ] Regex search input sanitized (invalid regex returns error, not panic)

### C. Infrastructure & Dependencies

- [ ] txtscape-mcp has zero external dependencies (standard library only)
- [ ] Dockerfile uses multi-stage build (no compiler in final image)
- [ ] Final image is minimal (alpine)
- [ ] `CGO_ENABLED=0` for static binary
- [ ] No secrets in source code

### D. MCP Tool Safety

- [ ] All tool handlers validate required parameters before acting
- [ ] Error messages don't leak filesystem paths outside project
- [ ] Optimistic concurrency (expected_hash) prevents stale overwrites
- [ ] str_replace_page requires exactly-once match (no ambiguous edits)

## Step 4: Report findings

Output a table sorted by severity:

```
## Scan Results

| # | Severity | Category | Finding | File:Line | Fix |
|---|----------|----------|---------|-----------|-----|
| 1 | MEDIUM | Path | validatePath allows overly long total paths | main.go:115 | Add total path length check |
| 2 | LOW | Infra | No file permission hardening on created files | main.go:560 | Use 0o600 instead of 0o644 |
| 3 | INFO | MCP | No rate limiting on tool calls | main.go | Accepted risk — local stdio transport |

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
