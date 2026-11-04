package downloads

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// TestSyncCollection downloads the tracks added to a playlist downloaded
// before, and leaves the others as they are: one that failed isn't tried
// again, one whose download was removed isn't downloaded again, and one
// taken out of the playlist stays downloaded with it.
func TestSyncCollection(t *testing.T) {
	f := &fetcher{fail: func(video string, _ int) error {
		if video == "fffffffffff" {
			return stream.ErrUnavailable
		}
		return nil
	}}
	m := open(t, t.TempDir(), Options{Fetcher: f})
	kept, failing, removed, out := yt("kkkkkkkkkkk", "Paper Lanterns"), yt("fffffffffff", "Static Bloom"),
		yt("rrrrrrrrrrr", "Low Tide Choir"), yt("ooooooooooo", "Window Seat")
	p := Collection{ID: "playlist:p", Kind: "playlist", Title: "Road Trip"}
	m.AddCollection(p, []media.Track{kept, failing, removed, out}, "best")
	m.Start()
	until(t, "the downloads", func() bool { return m.Status().Waiting == 0 })
	if kind(m, failing.ID) != Failed || m.Status().Tracks != 3 {
		t.Fatalf("the failing track is %v, with %d downloaded", kind(m, failing.ID), m.Status().Tracks)
	}
	m.Remove(removed.ID)
	asked := len(f.asked())

	// Out is taken out of the playlist, and two tracks are added.
	added, more := yt("aaaaaaaaaaa", "Neon Orchard"), yt("bbbbbbbbbbb", "Glass Harbor")
	now := []media.Track{kept, added, failing, removed, more}
	if n := m.SyncCollection(p, now, "best"); n != 2 {
		t.Errorf("the sync added %d", n)
	}
	c, _ := m.Snapshot().Collection(p.ID)
	if want := []string{kept.ID, added.ID, failing.ID, removed.ID, more.ID, out.ID}; !slices.Equal(c.Tracks, want) {
		t.Errorf("the playlist holds %v, not %v", c.Tracks, want)
	}
	until(t, "the tracks added", func() bool { return kind(m, added.ID) == Done && kind(m, more.ID) == Done })
	if kind(m, failing.ID) != Failed || kind(m, removed.ID) != None || kind(m, out.ID) != Done {
		t.Errorf("failing is %v, removed %v, out %v", kind(m, failing.ID), kind(m, removed.ID), kind(m, out.ID))
	}
	if got := len(f.asked()); got != asked+2 {
		t.Errorf("%d downloads asked for, of 2 added", got-asked)
	}

	// Again, nothing is new: nothing waits, as a track tried again would at
	// once.
	if n := m.SyncCollection(p, now, "best"); n != 0 {
		t.Errorf("the sync again added %d", n)
	}
	if kind(m, failing.ID) != Failed || kind(m, removed.ID) != None || m.Status().Waiting != 0 {
		t.Errorf("again, failing is %v and removed %v, with %d waiting", kind(m, failing.ID), kind(m, removed.ID), m.Status().Waiting)
	}

	// The track taken out goes with the playlist's download.
	m.RemoveCollection(p.ID)
	if s := m.Status(); s.Tracks != 0 || s.Failed != 0 || kind(m, out.ID) != None {
		t.Errorf("after the playlist went, %d downloads stay, %d failed, out %v", s.Tracks, s.Failed, kind(m, out.ID))
	}
}

// TestSyncNotDownloaded leaves a collection that isn't downloaded alone.
func TestSyncNotDownloaded(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	v := m.Snapshot().Version()
	if n := m.SyncCollection(Collection{ID: "playlist:p", Kind: "playlist", Title: "Road Trip"}, []media.Track{yt("aaaaaaaaaaa", "Neon Orchard")}, "best"); n != 0 {
		t.Errorf("the sync added %d", n)
	}
	if _, ok := m.Snapshot().Collection("playlist:p"); ok || m.Snapshot().Any() || m.Snapshot().Version() != v {
		t.Error("a collection not downloaded was downloaded")
	}
}

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
