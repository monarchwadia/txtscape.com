# Test Conventions

Shared across all test levels (unit, integration, E2E).

## Test Naming

```
TestSubject_Scenario_BusinessReason_Expected
```

The business reason is the key differentiator. It answers "why does this test exist?"

Examples:
- `TestHashPassword_EmptyInput_PreventBlankPasswords_ReturnsError`
- `TestSignup_DuplicateUsername_PreventImpersonation_Returns409`
- `TestPutPage_11LevelsDeep_Enforce10LevelLimit_Returns400`
- `TestJourney_NewUserPublishesFirstPage_CoreUseCase_PageIsAccessible`

## Test Documentation

Every test MUST have a structured comment block:

```go
// Business context: Users pick their own usernames on signup. Duplicate usernames
// would let someone impersonate another user or overwrite their content.
// Scenario: Attempt to create a user with a username that already exists in the DB.
// Expected: Returns 409 Conflict, does not modify existing user.
```

This block serves two purposes:
1. A human (or LLM) reading it later understands exactly why the test exists
2. When modifying the test, the business context prevents accidental changes that violate the requirement

## Test Structure

Use Go stdlib `testing` only. No testify, no gomock, no external test deps.

```go
func TestSubject_Scenario_Reason_Expected(t *testing.T) {
    // Arrange
    ...

    // Act
    result, err := FunctionUnderTest(input)

    // Assert
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Table-Driven Tests

Use when testing multiple variations of the same logic. Each case includes a `reason` field:

```go
tests := []struct {
    name    string
    input   string
    wantErr bool
    reason  string
}{
    {
        name:    "root index",
        input:   "/index.txt",
        wantErr: false,
        reason:  "users must be able to publish a homepage",
    },
    {
        name:    "11 levels deep",
        input:   "/a/b/c/d/e/f/g/h/i/j/k/file.txt",
        wantErr: true,
        reason:  "enforce 10-level nesting limit to prevent abuse",
    },
}
```

## Edge Cases to Always Consider

- Empty strings and nil inputs
- Boundary values (exactly at the limit: 100KB, 10 levels, 10 chars)
- Off-by-one (one past the limit: 100KB+1, 11 levels, 11 chars)
- Invalid characters in usernames, folder names, file names
- Path traversal attempts (`../`, `..%2f`, `~alice/../~bob/`)
- SQL injection in string inputs (should be harmless with parameterized queries, but validate the validation layer rejects them)
- Unicode / non-ASCII in names
- Concurrent operations (for integration/e2e only)

## Assertion Patterns

Prefer specific assertions over generic ones:

```go
// Good — tells you exactly what went wrong
if got != want {
    t.Errorf("username: got %q, want %q", got, want)
}

// Bad — just says "not equal"
if got != want {
    t.Error("mismatch")
}
```

For HTTP status codes:
```go
if resp.StatusCode != http.StatusConflict {
    body, _ := io.ReadAll(resp.Body)
    t.Fatalf("status: got %d, want %d. Body: %s", resp.StatusCode, http.StatusConflict, body)
}
```

## File Placement

- Unit tests: same package, `*_test.go` (no build tag)
- Integration tests: same package, `*_integration_test.go` with `//go:build integration`
- E2E tests: `e2e/` directory with `//go:build e2e`
