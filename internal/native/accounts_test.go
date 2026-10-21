package native

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestRefusedSpotifySignOut offers to sign out of a sign-in Spotify
// refused, which would fail every request of Spotify meanwhile, and asks
// first.
func TestRefusedSpotifySignOut(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Spotify: app.SpotifyAccount{
		SignedIn: true, Error: "spotify: this Spotify account is not one of the app's users: add it under Users Management",
	}}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	if !tt.HasText("Spotify refused the sign-in") {
		t.Fatalf("the refusal isn't shown: %q", tt.Texts())
	}
	click(t, s, tt, "sign out")
	if s.asking == nil || s.asking.title != "Sign out of Spotify" {
		t.Errorf("signing out asks %+v", s.asking)
	}
}

// TestSpotifySteps asks, before signing in, for what Spotify asks of an
// app in development mode: an owner with Premium, and the account that
// signs in under Users Management.
func TestSpotifySteps(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	texts := strings.Join(tt.Texts(), " ")
	for _, want := range []string{"a Spotify account that has Premium", "under Users Management, add your name", "Client ID here"} {
		if !strings.Contains(texts, want) {
			t.Errorf("the steps don't say %q: %q", want, tt.Texts())
		}
	}
}
