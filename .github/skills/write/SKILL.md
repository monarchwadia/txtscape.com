---
name: write
description: "Write code or tests for txtscape. USE WHEN: user says 'write', 'test', 'implement', 'add', 'build', 'create handler', 'create test', 'exercise', 'ergonomics', 'take it for a drive', 'critique'. Supports: /write tests, /write handler, /write plan, /write what's missing, /write exercise. Auto-detects scope from open file. Follows red/green TDD and walking skeleton methodology — builds incrementally, one small step at a time."
---

# /write

One entry point for all code and test writing in the txtscape project. Read [Project.md](../../Project.md) for full project context.

## Step 1: Check project memory

Before writing anything, check txtscape for relevant decisions and patterns:

1. Call `search_pages` with the topic you're about to work on (e.g. "auth", "validation", "handler")
2. Call `list_pages` on `decisions/` and `patterns/` to see what constraints exist
3. Read any relevant pages with `get_page`
4. Follow any constraints or patterns found — do not contradict recorded decisions

If txtscape has no pages yet, skip this step.

## Step 2: Narrate what you're about to do

Before writing anything, tell the user what you detected and decided (include any txtscape findings):

```
Detected: main.go is open → writing unit tests for path validation
Level: unit
Target: txtscape-mcp/main_test.go
Approach: red/green TDD, starting with the simplest case
```

If ambiguous, ask using the ask-questions tool.

## Step 3: Route to the right mode

| User says | Mode | txtscape reference |
|---|---|---|
| `/write tests` | Auto-detect level from open file | See routing below |
| `/write unit tests` | Unit | `get_page` → `references/skills/shared/unit.txt` |
| `/write integration tests` | Integration | `get_page` → `references/skills/shared/integration.txt` |
| `/write e2e` | E2E | `get_page` → `references/skills/shared/e2e.txt` |
| `/write handler`, `/write code`, `/write migration` | Production code | `get_page` → `references/skills/shared/code.txt` |
| `/write plan` | Preview only — describe what you'd do, don't act | All references as needed |
| `/write what's missing` | Gap analysis — scan code vs tests, find holes | All references as needed |
| `/write exercise` | Tool ergonomics — exercise→critique→fix cycle | `get_page` → `references/skills/shared/exercise.txt` |

### Auto-detect test level

1. User explicitly says "unit", "integration", or "e2e" → use that
2. Open file ends in `_test.go` with `//go:build integration` → integration
3. Open file is in `e2e/` → e2e
4. Open file ends in `_test.go` → unit
5. Open file is a source `.go` file → unit tests for that file
6. Otherwise → ask

## Step 4: Follow the methodology

**This is the most important section. Use `get_page` → `references/skills/shared/methodology.txt` before writing anything.**

Summary:
- **Walking skeleton first**: get the thinnest possible slice working end-to-end before adding depth
- **Red/green TDD**: write ONE failing test, then write the minimum code to pass it, then repeat
- **Stepwise**: never generate a whole file. Build it up one test-and-implementation pair at a time
- **Verify each step**: after each green, confirm it compiles and passes before moving on

## Step 5: Post-run summary

After writing, output:

```
✓ Written: txtscape-mcp/main_test.go
  - TestValidatePath_EmptyInput_ReturnsError
  - TestValidatePath_ValidInput_ReturnsCleanPath
✓ Written: txtscape-mcp/main.go
  - validatePath() — implemented to pass above tests
  
Next: TestValidatePath_TraversalAttempt (not yet written)
Untested: handleGetPage, handlePutPage
```

## Step 6: Store what you learned

If you made a decision or discovered a pattern worth remembering, store it in txtscape:

- **Architectural decisions** → `put_page` to `decisions/<topic>.txt`
- **Coding patterns** → `put_page` to `patterns/<topic>.txt`
- **Boundaries/constraints** → `put_page` to `boundaries/<topic>.txt`

Only store things that would help a future conversation avoid re-discovering the same insight.
Do not store trivial implementation details.

## Shared conventions

All modes share these — load via txtscape `get_page`:
- `references/skills/shared/test-conventions.txt` — naming, comments, structure
- `references/skills/shared/test-helpers.txt` — setupTestServer, callTool, getTextContent
- `references/skills/shared/methodology.txt` — walking skeleton, red/green TDD
