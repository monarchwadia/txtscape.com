//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Journey: Cross-user protection — can't write to someone else's space
//
// Steps:
//  1. Signup alice and bob
//  2. Alice tries PUT to /~bob/hack.txt → 403
//  3. Verify bob's space is untouched
func TestJourney_CrossUserProtection_PreventUnauthorized_Forbidden(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "alice", "password123")
	aliceToken := signup(t, srv, "bob", "password123") // intentionally swapped below
	_ = aliceToken

	// Re-signup properly
	srv2, cleanup2 := setupServer(t)
	defer cleanup2()

	aliceToken = signup(t, srv2, "alice", "password123")
	signup(t, srv2, "bob", "password123")

	// Alice's token trying to write to bob's space
	resp := putPage(t, srv2, aliceToken, "/~bob/hack.txt", "hacked!")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-user PUT status = %d, want 403", resp.StatusCode)
	}

	// Verify bob's space is empty
	status, listing := getPage(t, srv2, "/~bob")
	if status != 200 {
		t.Fatalf("bob listing status = %d", status)
	}
	if strings.Contains(listing, "hack.txt") {
		t.Fatal("hack.txt should not exist in bob's space")
	}
}

// Journey: Multiple tokens both work
//
// Steps:
//  1. Signup → get token1
//  2. Login → get token2
//  3. Both tokens can PUT pages
func TestJourney_MultipleTokens_BothValid_BothAuthorize(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	token1 := signup(t, srv, "frank", "password123")

	// Login to get second token
	resp, err := http.Post(srv.URL+"/login",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=frank&password=password123"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var tok tokenResp
	json.NewDecoder(resp.Body).Decode(&tok)
	token2 := tok.Token

	if token1 == token2 {
		t.Fatal("tokens should be different")
	}

	// Both tokens work
	r1 := putPage(t, srv, token1, "/~frank/from-token1.txt", "token 1 wrote this")
	r1.Body.Close()
	if r1.StatusCode != http.StatusNoContent {
		t.Fatalf("token1 PUT status = %d", r1.StatusCode)
	}

	r2 := putPage(t, srv, token2, "/~frank/from-token2.txt", "token 2 wrote this")
	r2.Body.Close()
	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("token2 PUT status = %d", r2.StatusCode)
	}

	// Both pages accessible
	status, _ := getPage(t, srv, "/~frank/from-token1.txt")
	if status != 200 {
		t.Fatal("token1's page not accessible")
	}
	status, _ = getPage(t, srv, "/~frank/from-token2.txt")
	if status != 200 {
		t.Fatal("token2's page not accessible")
	}
}

// Journey: Wrong password rejected
//
// Steps:
//  1. Signup
//  2. Login with wrong password → 401
//  3. Login with right password → 200 + token
func TestJourney_WrongPassword_RejectIntruder_ThenSucceed(t *testing.T) {
	srv, cleanup := setupServer(t)
	defer cleanup()

	signup(t, srv, "grace", "correctpassword")

	// Wrong password
	resp, err := http.Post(srv.URL+"/login",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=grace&password=wrongpassword"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", resp.StatusCode)
	}

	// Right password
	resp, err = http.Post(srv.URL+"/login",
		"application/x-www-form-urlencoded",
		strings.NewReader("username=grace&password=correctpassword"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("right password status = %d, body = %s", resp.StatusCode, body)
	}
}
