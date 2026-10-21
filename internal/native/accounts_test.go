package native

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

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
