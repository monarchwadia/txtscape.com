package auth

import "testing"

func TestValidateUsername_Empty_RequireIdentity_ReturnsError(t *testing.T) {
	// Business context: Every user needs a unique namespace (~username).
	// Empty usernames would create an invalid URL path.
	// Scenario: Pass an empty string as username.
	// Expected: Returns error.
	err := ValidateUsername("")
	if err == nil {
		t.Fatal("expected error for empty username, got nil")
	}
}

func TestValidateUsername_Valid_AllowRegistration_ReturnsNil(t *testing.T) {
	// Business context: Usernames form the URL namespace (~alice).
	// Valid lowercase alphanumeric names must be accepted.
	// Scenario: Pass a valid lowercase username.
	// Expected: Returns nil.
	err := ValidateUsername("alice")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateUsername_Variations(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		reason  string
	}{
		{"lowercase", "bob", false, "basic valid username"},
		{"with-hyphen", "my-user", false, "hyphens allowed in usernames"},
		{"with-underscore", "my_user", false, "underscores allowed in usernames"},
		{"with-numbers", "user123", false, "numbers allowed in usernames"},
		{"uppercase", "Alice", true, "uppercase not allowed, URLs should be case-insensitive"},
		{"spaces", "my user", true, "spaces invalid in URLs"},
		{"too-long", "abcdefghijklmnopqrstuvwxyz12345", true, "31 chars exceeds 30 char limit"},
		{"exactly-30", "abcdefghijklmnopqrstuvwxyz1234", false, "30 chars is the boundary"},
		{"special-chars", "user@name", true, "special characters not allowed"},
		{"dot", "user.name", true, "dots not allowed, could confuse path resolution"},
		{"slash", "user/name", true, "slashes not allowed, would break URL routing"},
		{"tilde", "~user", true, "tilde not allowed, reserved for URL prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("[%s] got err=%v, wantErr=%v", tt.reason, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword_TooShort_EnforceMinSecurity_ReturnsError(t *testing.T) {
	// Business context: Passwords protect user content from unauthorized modification.
	// Short passwords are trivially brute-forced.
	// Scenario: Pass a 7-character password.
	// Expected: Returns error.
	err := ValidatePassword("1234567")
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
}

func TestValidatePassword_Empty_PreventBlankPasswords_ReturnsError(t *testing.T) {
	// Business context: A blank password means anyone can publish as this user.
	// Scenario: Pass an empty string as password.
	// Expected: Returns error.
	err := ValidatePassword("")
	if err == nil {
		t.Fatal("expected error for empty password, got nil")
	}
}

func TestValidatePassword_Exactly8_BoundaryMinLength_ReturnsNil(t *testing.T) {
	// Business context: 8 characters is the minimum accepted length.
	// Scenario: Pass exactly 8 characters.
	// Expected: Returns nil (accepted).
	err := ValidatePassword("12345678")
	if err != nil {
		t.Fatalf("expected nil for 8-char password, got %v", err)
	}
}

func TestValidatePassword_73Chars_BcryptLimit_ReturnsError(t *testing.T) {
	// Business context: bcrypt silently truncates passwords longer than 72 bytes.
	// Allowing longer passwords would give a false sense of security.
	// Scenario: Pass a 73-character password.
	// Expected: Returns error.
	pw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 73 a's
	err := ValidatePassword(pw)
	if err == nil {
		t.Fatal("expected error for 73-char password, got nil")
	}
}

func TestValidatePassword_72Chars_BcryptBoundary_ReturnsNil(t *testing.T) {
	// Business context: 72 bytes is bcrypt's max input. Must accept exactly 72.
	// Scenario: Pass exactly 72 characters.
	// Expected: Returns nil.
	pw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 72 a's
	err := ValidatePassword(pw)
	if err != nil {
		t.Fatalf("expected nil for 72-char password, got %v", err)
	}
}
