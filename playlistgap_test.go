package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func TestGapText(t *testing.T) {
	for _, c := range []struct {
		left   media.Unplayable
		hidden int
		want   string
	}{
		{media.Unplayable{}, 0, ""},
		{media.Unplayable{Local: 30}, 0, "30 of this playlist's items can't play here: 30 local files."},
		{media.Unplayable{Local: 1, Episodes: 2, Gone: 3}, 0, "6 of this playlist's items can't play here: 1 local file, 2 episodes and 3 removed or unavailable items."},
		{media.Unplayable{}, 1, "1 explicit song is hidden by your settings."},
		{media.Unplayable{Episodes: 1}, 4, "1 of this playlist's items can't play here: 1 episode. 4 explicit songs are hidden by your settings."},
	} {
		if got := gapText("playlist", c.left, c.hidden); got != c.want {
			t.Errorf("gapText(%+v, %d) = %q, want %q", c.left, c.hidden, got, c.want)
		}
	}
}

// TestPlaylistSaysWhatItLeavesOut tells, once a playlist is read, why its
// count is above its rows, and says nothing while it loads.
func TestPlaylistSaysWhatItLeavesOut(t *testing.T) {
	s := newTestShell(t)
	key := "playlist:" + longPlaylistID
	f := slowReading(t, s, key)
	head := longHead(longPlaylistID)
	head.value.Unplayable = media.Unplayable{Local: 2}
	s.loads[key] = head
	tt := tester(t, s)
	show(s, tt, playlistPath(longPlaylistID))
	if tt.HasText("can't play here") {
		t.Errorf("the line shows while the tracks load: %q", tt.Texts())
	}
	f.page(t, s, tt)
	f.page(t, s, tt)
	if !tt.HasText("2 of this playlist's items can't play here: 2 local files.") {
		t.Errorf("the read playlist shows %q", tt.Texts())
	}
}

// TestPlaylistSaysWhatSettingsHide counts the explicit songs that Hide
// explicit content leaves out of another's playlist, under their own
// name, and not those of a playlist that lists them dimmed.
func TestPlaylistSaysWhatSettingsHide(t *testing.T) {
	s := newTestShell(t)
	other := media.Playlist{ID: "yt:PLgaps", Source: media.SourceYouTube, Title: "Gull Mix", Owner: "YouTube Music",
		Tracks: []media.Track{saltWind, paperBoats, saltSpray, lowWater}}
	s.loads["playlist:"+other.ID] = &loaded[media.Playlist]{done: true, value: other}
	hidingExplicit(t, s)
	tt := testerOf(t, s, 1280, 1200)
	show(s, tt, playlistPath(other.ID))
	until(t, s, tt, "the playlist", func() bool { return showsExactly(tt, "Low Water") })
	hidden := len(other.Tracks) - len(media.WithoutExplicit(other.Tracks))
	if want := plural(hidden, "explicit song") + " are hidden by your settings."; hidden < 2 || !tt.HasText(want) {
		t.Errorf("want %q, the page shows %q", want, tt.Texts())
	}
}
