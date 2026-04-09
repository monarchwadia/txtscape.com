---
name: write
description: "Write code or tests for txtscape. USE WHEN: user says 'write', 'test', 'implement', 'add', 'build', 'create handler', 'create test'. Supports: /write tests, /write handler, /write plan, /write what's missing. Auto-detects scope from open file. Follows red/green TDD and walking skeleton methodology — builds incrementally, one small step at a time."
---

# /write

One entry point for all code and test writing in the txtscape project. Read [Project.md](../../Project.md) for full project context.

## Step 1: Narrate what you're about to do

Before writing anything, tell the user what you detected and decided:

```
Detected: auth.go is open → writing unit tests for auth
Level: unit
Target: internal/auth/auth_test.go
Approach: red/green TDD, starting with the simplest case
```

If ambiguous, ask using the ask-questions tool.

## Step 2: Route to the right mode

| User says | Mode | Reference |
|---|---|---|
| `/write tests` | Auto-detect level from open file | See routing below |
| `/write unit tests` | Unit | [unit.md](references/unit.md) |
| `/write integration tests` | Integration | [integration.md](references/integration.md) |
| `/write e2e` | E2E | [e2e.md](references/e2e.md) |
| `/write handler`, `/write code`, `/write migration` | Production code | [code.md](references/code.md) |
| `/write plan` | Preview only — describe what you'd do, don't act | All references as needed |
| `/write what's missing` | Gap analysis — scan code vs tests, find holes | All references as needed |

### Auto-detect test level

1. User explicitly says "unit", "integration", or "e2e" → use that
2. Open file ends in `_test.go` with `//go:build integration` → integration
3. Open file is in `e2e/` → e2e
4. Open file ends in `_test.go` → unit
5. Open file is a source `.go` file → unit tests for that file
6. Otherwise → ask

## Step 3: Follow the methodology

**This is the most important section. Read [methodology.md](references/methodology.md) before writing anything.**

Summary:
- **Walking skeleton first**: get the thinnest possible slice working end-to-end before adding depth
- **Red/green TDD**: write ONE failing test, then write the minimum code to pass it, then repeat
- **Stepwise**: never generate a whole file. Build it up one test-and-implementation pair at a time
- **Verify each step**: after each green, confirm it compiles and passes before moving on

## Step 4: Post-run summary

After writing, output:

```
✓ Written: internal/auth/auth_test.go
  - TestHashPassword_EmptyInput_PreventBlankPasswords_ReturnsError
  - TestHashPassword_ValidInput_StoreCredentials_ReturnsHash
✓ Written: internal/auth/auth.go
  - HashPassword() — implemented to pass above tests
  
Next: TestValidatePassword (not yet written)
Untested: CreateUser, GetUser
```

## Shared conventions

All modes share these:
- [Test conventions](references/test-conventions.md) — naming, comments, structure
- [Validation rules](references/validation-rules.md) — regexes, limits, status codes
- [DB test setup](references/db-setup.md) — TestMain, per-package DB, cleanup
- [Test helpers](references/test-helpers.md) — signUp, putPage, requireStatus
