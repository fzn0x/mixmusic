package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/lastfm"
)

// TestLastfmUserAgent has Last.fm hear MixMusic's version and address, as
// the lyrics services do.
func TestLastfmUserAgent(t *testing.T) {
	a := &App{opts: Options{DataDir: t.TempDir(), Version: "1.2.3"}}
	a.startScrobbling(lastfmCredentials{APIKey: "key", Secret: "secret", Session: lastfm.Session{Name: "ann", Key: "sk"}})
	if got, want := a.lastfm.client.UserAgent, "MixMusic/1.2.3 (https://github.com/fzn0x/mixmusic)"; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}
