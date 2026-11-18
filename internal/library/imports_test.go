package library

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLikeAll likes an import's tracks first, in their order, leaves
// those liked already where they are, and takes the dislikes back.
func TestLikeAll(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := media.Track{ID: "yt:a", Title: "A"}
	b := media.Track{ID: "yt:b", Title: "B"}
	c := media.Track{ID: "yt:c", Title: "C"}
	if err := l.SetLiked(b, true); err != nil {
		t.Fatal(err)
	}
	if _, err := l.SetDisliked(c, true); err != nil {
		t.Fatal(err)
	}
	added, err := l.LikeAll([]media.Track{a, b, c, a})
	if err != nil {
		t.Fatal(err)
	}
	ids := func(ts []media.Track) []string {
		var out []string
		for _, t := range ts {
			out = append(out, t.ID)
		}
		return out
	}
	if got := ids(added); !slices.Equal(got, []string{"yt:a", "yt:c"}) {
		t.Errorf("liked %q", got)
	}
	if got := ids(l.Liked()); !slices.Equal(got, []string{"yt:a", "yt:c", "yt:b"}) {
		t.Errorf("the liked are %q", got)
	}
	if l.IsDisliked("yt:c") {
		t.Error("C is still disliked")
	}
	p, _ := l.CreatePlaylist("Mix", "", []media.Track{c, {ID: "yt:d"}})
	_, native, _ := media.ParseID(p.ID)
	if got, _ := l.PlaylistTrackIDs(native); !got["yt:c"] || !got["yt:d"] || len(got) != 2 {
		t.Errorf("the mix holds %v", got)
	}
	if got := l.TrackIDs(); len(got) != 4 {
		t.Errorf("the library holds %v", got)
	}
	if got := ids(l.Tracks()); !slices.Equal(got, []string{"yt:a", "yt:c", "yt:b", "yt:d"}) {
		t.Errorf("the library's tracks are %q", got)
	}
}
