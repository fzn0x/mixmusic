package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestLastfmAccountTexts say how many scrobbles wait wherever the user
// chooses what becomes of them.
func TestLastfmAccountTexts(t *testing.T) {
	none := app.LastfmAccount{Connected: true, Name: "ann"}
	two := app.LastfmAccount{Connected: true, Name: "ann", Waiting: 2, Other: "bo"}
	one := app.LastfmAccount{Connected: true, Name: "ann", Waiting: 1}
	for _, c := range []struct {
		name, got, want string
	}{
		{"expired, none waiting", lastfmExpiredText(none), "Connect again to go on scrobbling."},
		{"expired, one", lastfmExpiredText(one), "1 scrobble waits to be sent."},
		{"expired, two", lastfmExpiredText(two), "2 scrobbles wait to be sent."},
		{"disconnect, none", lastfmDisconnectText(none), "your scrobbles stay on Last.fm."},
		{"disconnect, two", lastfmDisconnectText(two), "2 scrobbles not sent yet will be dropped."},
		{"other user", lastfmOtherText(two), "2 scrobbles were heard as ann"},
		{"other user's name", lastfmOtherText(two), "bo's profile"},
	} {
		if !strings.Contains(c.got, c.want) {
			t.Errorf("%s: %q lacks %q", c.name, c.got, c.want)
		}
	}
	if got := lastfmDisconnectText(none); strings.Contains(got, "dropped") {
		t.Errorf("disconnect with none waiting: %q", got)
	}
}
