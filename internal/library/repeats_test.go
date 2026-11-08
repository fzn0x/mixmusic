package library

import (
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestAddNewToPlaylist adds the tracks a mix doesn't hold, and tells which
// it held; adding them again puts them in a second time.
func TestAddNewToPlaylist(t *testing.T) {
	l := open(t)
	a, b, c := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B"), tr("yt:c", media.SourceYouTube, "C")
	p, err := l.CreatePlaylist("Road trip", "", []media.Track{a})
	if err != nil {
		t.Fatal(err)
	}
	_, id, _ := media.ParseID(p.ID)
	ids := func() []string {
		got, _ := l.Playlist(id)
		var out []string
		for _, t := range got.Tracks {
			out = append(out, t.ID)
		}
		return out
	}

	held, err := l.AddNewToPlaylist(id, []media.Track{b, a, c, b})
	if err != nil || len(held) != 2 || held[0].ID != "yt:a" || held[1].ID != "sp:b" {
		t.Fatalf("held %+v, %v", held, err)
	}
	if got := ids(); len(got) != 3 || got[1] != "sp:b" || got[2] != "yt:c" {
		t.Fatalf("the mix holds %q", got)
	}

	// All of them held, the mix stays as it was, not moved first.
	other, err := l.CreatePlaylist("Other", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	held, err = l.AddNewToPlaylist(id, []media.Track{c})
	if err != nil || len(held) != 1 || len(ids()) != 3 {
		t.Errorf("all held: held %d, the mix holds %q, %v", len(held), ids(), err)
	}
	if first := l.Playlists()[0]; first.ID != other.ID {
		t.Errorf("an add of nothing new moved %s first", first.Title)
	}

	if err := l.AddToPlaylistAgain(id, []media.Track{c}); err != nil {
		t.Fatal(err)
	}
	if got := ids(); len(got) != 4 || got[3] != "yt:c" {
		t.Errorf("added again, the mix holds %q", got)
	}
	if _, err := l.AddNewToPlaylist("nope", []media.Track{a}); !errors.Is(err, ErrNoPlaylist) {
		t.Errorf("a mix that isn't: %v", err)
	}
	if err := l.AddToPlaylistAgain("nope", []media.Track{a}); !errors.Is(err, ErrNoPlaylist) {
		t.Errorf("a mix that isn't, again: %v", err)
	}
}
