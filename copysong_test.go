package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestCopySong copies a song's link, title, artists and "Artist - Title"
// from the Copy submenu of its menu on an album's page and in the queue,
// and says what was copied. A song with no link has no Copy link.
func TestCopySong(t *testing.T) {
	var copied []string
	was := copyText
	copyText = func(text string) { copied = append(copied, text) }
	t.Cleanup(func() { copyText = was })
	s := newTestShell(t)
	duo := testTrack("yt:11", "Paper Boats", "Ash Field", "Low Tide", media.SourceYouTube)
	duo.Artists = []media.Ref{{Name: "Ash Field"}, {Name: "Cold Harbor"}}
	duo.URL = "https://music.youtube.com/watch?v=paperboats"
	local := testTrack("file:12", "Quiet Room", "Ash Field", "Low Tide", media.SourceFile)
	album := media.Album{ID: "al-x", Title: "Low Tide", Source: media.SourceYouTube, Tracks: []media.Track{duo, local}}
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: album}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")

	copyOf := func(track media.Track, item, want, note string) {
		t.Helper()
		if err := tt.RightClick(track.Title); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem("Copy", item); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		tt.Frame()
		if len(copied) == 0 || copied[len(copied)-1] != want {
			t.Errorf("%s copied %q, not %q", item, copied, want)
		}
		if !tt.HasText(note) {
			t.Errorf("%s: no toast says %q: %q", item, note, tt.Texts())
		}
	}
	copyOf(duo, "Copy link", duo.URL, "Copied the link")
	copyOf(duo, "Copy title", "Paper Boats", "Copied the title")
	copyOf(duo, "Copy artist", "Ash Field, Cold Harbor", "Copied the artist")
	copyOf(duo, `Copy "Artist - Title"`, "Ash Field, Cold Harbor - Paper Boats", "Copied the artist and title")

	// A file on this computer has no link to copy.
	if err := tt.RightClick(local.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy", "Copy link"); err == nil {
		t.Error("a file on this computer offers to copy its link")
	}
	tt.CloseMenu()

	// The queue's menu has the same.
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.panel = "queue"
	tt = tester(t, s)
	copyOf(testTracks[3], `Copy "Artist - Title"`, "Aphex Twin - Windowlicker", "Copied the artist and title")
}
