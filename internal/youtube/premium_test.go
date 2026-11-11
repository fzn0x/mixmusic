package youtube

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// TestReadPremium reads whether an answer tells of Premium: from
// has_unlimited_entitlement, else from premium_membership's YouTube
// Premium, and not at all from an answer that tells neither.
func TestReadPremium(t *testing.T) {
	membership := func(value string) node {
		return node{map[string]any{"responseContext": map[string]any{"serviceTrackingParams": []any{
			map[string]any{"service": "GFEEDBACK", "params": []any{map[string]any{"key": "premium_membership", "value": value}}},
		}}}}
	}
	for name, c := range map[string]struct {
		n             node
		premium, told bool
	}{
		"Premium":                 {fixture(t, "account-menu-premium"), true, true},
		"free":                    {fixture(t, "account-menu-free"), false, true},
		"neither":                 {fixture(t, "playlist-suggestions"), false, false},
		"YouTube Premium alone":   {membership("youtube_premium"), true, true},
		"another membership":      {membership("something_else"), false, false},
		"no responseContext":      {node{map[string]any{}}, false, false},
		"unlimited, in lowercase": {node{map[string]any{"responseContext": map[string]any{"serviceTrackingParams": []any{map[string]any{"service": "GFEEDBACK", "params": []any{map[string]any{"key": "has_unlimited_entitlement", "value": "true"}}}}}}}, true, true},
	} {
		if premium, told := readPremium(c.n); premium != c.premium || told != c.told {
			t.Errorf("%s: readPremium = %v, %v, want %v, %v", name, premium, told, c.premium, c.told)
		}
	}
}

// TestPremiumFromTheAccountMenu tells the account has Premium once the
// account's menu, which MixMusic reads of every account, said so, as any
// signed-in answer may; until then, of another session or account, or
// after a sign-out, it doesn't. An answer that tells nothing of it keeps
// what was told, and one to a visitor or to an anonymous request counts
// for nothing.
func TestPremiumFromTheAccountMenu(t *testing.T) {
	var mu sync.Mutex
	answer := "account-menu-premium"
	c, _, _ := fakeYouTube(t, func(path string) any {
		mu.Lock()
		defer mu.Unlock()
		if answer == "visitor" {
			return loggedIn("0")
		}
		return fixture(t, answer).v
	})
	set := func(name string) {
		mu.Lock()
		answer = name
		mu.Unlock()
	}
	ctx := context.Background()

	// Signed out, nothing is told.
	c.Home(ctx)
	if c.Premium() {
		t.Error("Premium signed out")
	}

	c.SetCookies(session)
	if c.Premium() {
		t.Error("Premium before an answer told")
	}
	if a, err := c.Account(ctx); err != nil || a.Name != "Ada Example" || !c.Premium() {
		t.Fatalf("Account = %+v, %v; Premium %v", a, err, c.Premium())
	}
	// An answer that tells nothing of it keeps what was told.
	set("playlist-suggestions")
	c.Home(ctx)
	if !c.Premium() {
		t.Error("an answer that told nothing took Premium away")
	}
	// Another account of the session, or another session, isn't told yet.
	c.SetAuthUser(1)
	if c.Premium() {
		t.Error("Premium of another account of the session")
	}
	c.SetAuthUser(0)
	if !c.Premium() {
		t.Error("Premium lost going back to the account")
	}
	c.SetCookies([]*http.Cookie{{Name: "SAPISID", Value: "another/session"}})
	if c.Premium() {
		t.Error("Premium of another session")
	}
	c.SetCookies(session)

	// A free account's menu says it hasn't.
	set("account-menu-free")
	if _, err := c.Account(ctx); err != nil || c.Premium() {
		t.Errorf("free: %v, Premium %v", err, c.Premium())
	}
	// Answered as a visitor, the session ended: no Premium of it.
	set("account-menu-premium")
	if _, err := c.Account(ctx); err != nil || !c.Premium() {
		t.Errorf("Premium again: %v, Premium %v", err, c.Premium())
	}
	set("visitor")
	c.Home(ctx)
	if !c.Rejected() || c.Premium() {
		t.Errorf("rejected %v, Premium %v", c.Rejected(), c.Premium())
	}
	c.SetCookies(nil)
	if c.Premium() {
		t.Error("Premium signed out")
	}
}

// TestPremiumNotFromAnonymousRequests keeps nothing of an answer to a
// request asked as a visitor YouTube doesn't know, as streams are.
func TestPremiumNotFromAnonymousRequests(t *testing.T) {
	c, _, _ := fakeYouTube(t, func(string) any { return fixture(t, "account-menu-premium").v })
	c.SetCookies(session)
	if _, err := c.callWith(context.Background(), musicClient, "browse", map[string]any{}, nil, callOptions{anonymous: true}); err != nil {
		t.Fatal(err)
	}
	if c.Premium() {
		t.Error("Premium from an anonymous request")
	}
}
