---
name: usability-test
description: "Run usability tests on the tutorial or tool with parallel agent personas. USE WHEN: user says 'usability test', 'test the tutorial', 'test the docs', 'user test', 'persona test', 'try as a beginner', 'first-time experience'. Supports: /usability-test tutorial, /usability-test tool, /usability-test docs. Launches parallel subagents with distinct personas, synthesizes findings by severity."
---

# /usability-test

Test the txtscape tutorial, tool, or docs for real-world usability by launching parallel agent personas. Each persona follows the material end-to-end in isolation, then reports what worked, what broke, and what confused them.

Read `get_page` → `learnings/usability-testing.txt` for methodology background.

## Step 1: Determine scope

| User says | Scope | What to test |
|-----------|-------|--------------|
| `/usability-test` | Tutorial | Full tutorial, steps 1–6 |
| `/usability-test tutorial` | Tutorial | Full tutorial, steps 1–6 |
| `/usability-test tool` | Tool | MCP tools directly, no tutorial |
| `/usability-test docs` | Docs | Reference docs / README accuracy |
| `/usability-test <url>` | Custom | Specific page or content |

## Step 2: Build the binary

Build the MCP binary to a known temporary path:

```bash
cd txtscape-mcp && go build -o /tmp/txtscape-mcp . && echo "BINARY OK"
```

If the build fails, fix the build error before proceeding.

## Step 3: Verify the server is running (tutorial scope only)

For tutorial testing, the web server must be running so agents can fetch tutorial pages:

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/tutorial
```

If not 200, start the server first.

## Step 4: Launch 3 parallel subagents

Launch all 3 simultaneously via `runSubagent`. Each agent gets:

- A distinct persona (see below)
- The binary path (`/tmp/txtscape-mcp`)
- Instructions to create its own temp dir (`mktemp -d`)
- The JSON-RPC protocol (initialize first, then tool calls)
- Explicit instruction that `config.json` is a filesystem file, not an MCP call

### Persona prompts

Each prompt must include:

1. **Identity**: who they are and how they approach instructions
2. **Setup**: binary path, temp dir creation, JSON-RPC protocol with examples
3. **Task**: read the material, follow every step, make real MCP calls
4. **Report format**: structured findings with quotes from the material vs actual output

#### Persona A: Junior Developer

> You are a junior developer encountering txtscape for the first time. You follow instructions as written and report immediately when confused or stuck. You don't guess — if something is unclear, you flag it.
>
> You catch: missing prerequisites, assumed knowledge, jargon without definition, steps that don't work on first attempt.

#### Persona B: Skeptical Mid-Level Developer

> You are a skeptical mid-level developer evaluating whether to adopt this tool. You follow instructions but also try to break things — follow wording too literally, test edge cases, try things out of order.
>
> You catch: factual errors, misleading claims, edge cases, things that work by accident, inconsistent terminology.

#### Persona C: Learn-by-Doing Developer

> You are a developer who learns by doing. You execute every step, verify every claim against actual output, and keep a running log. You don't skip anything.
>
> You catch: gaps between documented and actual output, friction in the happy path, moments where the concept "clicks" or doesn't.

### Shared prompt sections (include in all 3)

**Setup block** (copy into each prompt):

```
## Your setup

- MCP binary: `/tmp/txtscape-mcp` (stdin/stdout JSON-RPC)
- Working directory: create a fresh temp dir with `mktemp -d`
- The MCP server root = its cwd. Run: `(cd <your-temp-dir> && /tmp/txtscape-mcp)`
- Pipe JSON-RPC messages via heredoc:

\`\`\`bash
(cd /your/temp/dir && /tmp/txtscape-mcp) <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_pages","arguments":{"path":""}}}
EOF
\`\`\`

- config.json is a plain file, not an MCP call:

\`\`\`bash
mkdir -p <tmpdir>/.txtscape
cat > <tmpdir>/.txtscape/config.json <<'CONF'
{ ... }
CONF
\`\`\`
```

**Report format block** (copy into each prompt):

```
## What to report

- **Step-by-step results**: what you did, what the server returned, whether it matched
- **Confusion points**: ambiguous or unclear instructions (quote them)
- **Broken promises**: tutorial says X, actual output is Y (quote both)
- **Missing steps**: where you got stuck and had to figure it out yourself
- **Nice surprises**: anything that worked better than expected
```

## Step 5: Synthesize findings

After all 3 agents return, synthesize into a single report:

### Severity categories

| Severity | Meaning |
|----------|---------|
| **Critical** | Breaks the experience — user cannot proceed |
| **High** | Visible mismatch, user recovers but loses confidence |
| **Low** | Polish — minor inconsistency, cosmetic |
| **Working well** | Confirmed accurate, good design (note these too) |

### Cross-reference rule

- **2+ agents report the same issue** → definitely real, include in findings
- **1 agent reports it** → likely real but may be persona-specific, include with note
- **All 3 agents confirm something works** → list under "Working well"

### Output format

```
## Usability Test Results

### Critical
1. **[title]** — [description]. Agents: A, B. Quote: "tutorial says X" → actual: Y

### High
...

### Low
...

### Working Well
- [thing that all agents confirmed works correctly]
```

## Step 6: Offer to fix

After presenting findings:

> Want me to fix these? I'll start with Critical, then High. Each fix follows red/green TDD where applicable.

## Step 7: Record findings

If significant patterns emerge, update txtscape:

- Recurring usability issues → `learnings/`
- Design decisions made during fixes → `decisions/`
- Tutorial content fixes don't need txtscape entries — the HTML files are the source of truth
