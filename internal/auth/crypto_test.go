package auth

import "testing"

func TestHashPassword_ValidInput_StoreCredentials_ReturnsHash(t *testing.T) {
	// Business context: Passwords must be stored as bcrypt hashes, never plaintext.
	// Scenario: Hash a valid password.
	// Expected: Returns a non-empty hash that differs from the input.
	hash, err := HashPassword("mysecret1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "mysecret1" {
		t.Fatal("hash must differ from plaintext")
	}
}

func TestCheckPassword_Correct_AuthenticateUser_ReturnsNil(t *testing.T) {
	// Business context: Login flow compares user-provided password against stored hash.
	// Scenario: Hash a password then check it with the correct plaintext.
	// Expected: Returns nil (match).
	hash, _ := HashPassword("mysecret1")
	err := CheckPassword("mysecret1", hash)
	if err != nil {
		t.Fatalf("expected nil for correct password, got %v", err)
	}
}

func TestCheckPassword_Wrong_RejectIntruder_ReturnsError(t *testing.T) {
	// Business context: Wrong passwords must be rejected to prevent unauthorized access.
	// Scenario: Hash a password then check with a different plaintext.
	// Expected: Returns error.
	hash, _ := HashPassword("mysecret1")
	err := CheckPassword("wrongpass", hash)
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestGenerateToken_Uniqueness_PreventTokenCollision_ReturnsDifferentTokens(t *testing.T) {
	// Business context: Tokens authenticate write operations. Duplicate tokens
	// would let one user impersonate another.
	// Scenario: Generate two tokens.
	// Expected: Plaintext values differ.
	tok1, _, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tok2, _, _ := GenerateToken()
	if tok1 == tok2 {
		t.Fatal("two generated tokens should not be identical")
	}
}

func TestGenerateToken_Length_SufficientEntropy_Returns64Hex(t *testing.T) {
	// Business context: 32 random bytes = 64 hex chars = 256 bits of entropy.
	// This makes brute-force infeasible.
	// Scenario: Generate a token.
	// Expected: Plaintext is 64 characters (hex encoding of 32 bytes).
	tok, _, _ := GenerateToken()
	if len(tok) != 64 {
		t.Errorf("token length: got %d, want 64", len(tok))
	}
}

func TestCheckToken_Valid_AuthorizeWrites_ReturnsTrue(t *testing.T) {
	// Business context: Bearer tokens authorize PUT/DELETE operations.
	// Scenario: Generate a token, then check it against its own hash.
	// Expected: Returns true.
	plaintext, hash, _ := GenerateToken()
	if !CheckToken(plaintext, hash) {
		t.Fatal("expected true for valid token check")
	}
}

func TestCheckToken_Invalid_RejectUnauthorized_ReturnsFalse(t *testing.T) {
	// Business context: Invalid tokens must be rejected to prevent unauthorized writes.
	// Scenario: Check a random string against a token hash.
	// Expected: Returns false.
	_, hash, _ := GenerateToken()
	if CheckToken("notthetoken", hash) {
		t.Fatal("expected false for invalid token")
	}
}
