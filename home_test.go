package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestHomeFollowsPlays shows on Home what the user played and made after
// it first showed: the jump back in tile, recently played and your mixes
// follow the library, as the tiles of liked songs and mixes did.
func TestHomeFollowsPlays(t *testing.T) {
	s := newTestShell(t)
	col := s.svc.Collection
	if err := col.RecordPlay(testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, "/")
	if _, ok := tt.Find("Play " + testTracks[0].Title + " again"); !ok {
		t.Fatalf("jump back in doesn't offer what played: %q", tt.Texts())
	}
	if !shelfShows(tt, "Recently played") || shelfShows(tt, "Your mixes") {
		t.Fatalf("Home's own shelves: %q", tt.Texts())
	}

	// Behind the page, as the player records a play.
	if err := col.RecordPlay(testTracks[2]); err != nil {
		t.Fatal(err)
	}
	mix, err := col.CreatePlaylist("Night drive", "", testTracks[:1])
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("Play " + testTracks[2].Title + " again"); !ok {
		t.Errorf("jump back in doesn't offer what played last: %q", tt.Texts())
	}
	if !shelfShows(tt, "Your mixes") || !tt.HasText("Night drive") {
		t.Errorf("the new mix isn't on Home: %q", tt.Texts())
	}

	if err := col.DeletePlaylist(mix.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shelfShows(tt, "Your mixes") {
		t.Errorf("the mix deleted stays on Home: %q", tt.Texts())
	}
}

// TestHomeYoursAlone shows the user's own shelves under the yours pivot
// while the services' shelves still load, and no spinner for them.
func TestHomeYoursAlone(t *testing.T) {
	s := newTestShell(t)
	if err := s.svc.Collection.RecordPlay(testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	s.loads["home"] = &loaded[[]media.Shelf]{}
	s.homeFilter = 1
	show(s, tt, "/")
	if !shelfShows(tt, "Recently played") {
		t.Errorf("yours doesn't show what played: %q", tt.Texts())
	}
	if _, ok := tt.Find("Loading"); ok {
		t.Error("yours waits for the services")
	}
}

// shelfShows tells whether the shelf of title shows, by its arrows.
func shelfShows(tt *ui.Tester, title string) bool {
	_, ok := tt.Find("Scroll " + title + " on")
	return ok
}
