package downloads

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestAddCollectionKeepsTakenOut keeps a track taken out of a playlist
// downloaded, as the playlist's download takes the tracks it holds now:
// its download's removal removes it, rather than leaving it behind.
func TestAddCollectionKeepsTakenOut(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	a, b, c := yt("aaaaaaaaaaa", "Neon Orchard"), yt("bbbbbbbbbbb", "Glass Harbor"), yt("ccccccccccc", "Paper Lanterns")
	p := Collection{ID: "playlist:p", Kind: "playlist", Title: "Road Trip"}
	m.AddCollection(p, []media.Track{a, b}, "best")
	m.AddCollection(p, []media.Track{c, a}, "best")
	if col, _ := m.Snapshot().Collection(p.ID); !slices.Equal(col.Tracks, []string{c.ID, a.ID, b.ID}) {
		t.Errorf("the playlist holds %v", col.Tracks)
	}
	m.RemoveCollection(p.ID)
	if m.Snapshot().Any() {
		t.Errorf("after the playlist went, the downloads hold %+v", m.Status())
	}
}
