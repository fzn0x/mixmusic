package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/stream"
)

// TestNoAudioAsAccountSession tells the resolver why it has no session to
// ask as, for the words of a track YouTube plays only to signed-in viewers
// (#577): signed out, or another Google account of the session than its
// first; and nothing while it has one.
func TestNoAudioAsAccountSession(t *testing.T) {
	a, _ := cookieApp(t, premiumMenu(true))
	if a.resolver.NoSession == nil {
		t.Fatal("the resolver isn't told why it has no session")
	}
	if got := a.resolver.NoSession(); got != stream.AccountSignedOut {
		t.Errorf("signed out: %q", got)
	}
	if _, err := (&Accounts{a}).YouTubePasteCookies(context.Background(), "SAPISID=madeup-sapisid; LOGIN_INFO=madeup-login"); err != nil {
		t.Fatal(err)
	}
	if got := a.resolver.NoSession(); got != "" || len(a.audioAsAccountSession()) == 0 {
		t.Errorf("signed in: %q", got)
	}
	a.youtube.SetAuthUser(1)
	if got := a.resolver.NoSession(); got != stream.AccountNotFirst || a.audioAsAccountSession() != nil {
		t.Errorf("another Google account: %q", got)
	}
}
