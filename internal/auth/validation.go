package auth

import (
	"fmt"
	"regexp"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_-]{1,30}$`)

func ValidateUsername(username string) error {
	if !usernameRe.MatchString(username) {
		return fmt.Errorf("username must be 1-30 lowercase alphanumeric characters, hyphens, or underscores")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 72 {
		return fmt.Errorf("password must be at most 72 characters")
	}
	return nil
}
