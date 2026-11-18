package downloads

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestHiddenExplicit downloads nothing the settings hide as explicit (#583):
// an explicit track, without looking for its recording, nor a clean
// Spotify track whose recording is explicit. A clean one downloads.
func TestHiddenExplicit(t *testing.T) {
	f := &fetcher{}
	explicit, clean, dirty := yt("eeeeeeeeeee", "Explicit"), yt("ccccccccccc", "Clean"), yt("xxxxxxxxxxx", "Dirty")
	explicit.Explicit, dirty.Explicit = true, true
	marked := sp("marked", "Marked")
	marked.Explicit = true
	mt := &matcher{matches: map[string]media.Track{"sp:clean": clean, "sp:dirty": dirty, "sp:marked": clean}}
	m := open(t, t.TempDir(), Options{Fetcher: f, Matcher: mt, Hides: media.Track.ExplicitSong})
	m.Add([]media.Track{explicit, sp("clean", "Clean"), sp("dirty", "Dirty"), marked}, "best")
	m.Start()
	until(t, "the downloads", func() bool {
		return kind(m, "sp:clean") == Done && kind(m, explicit.ID) == Failed && kind(m, "sp:dirty") == Failed && kind(m, "sp:marked") == Failed
	})
	for _, id := range []string{explicit.ID, "sp:dirty", "sp:marked"} {
		if st := m.State(id); st.Reason != ReasonExplicit || st.Why != errHidden.Error() {
			t.Errorf("%s: %+v", id, st)
		}
	}
	if got := f.asked(); len(got) != 1 || got[0].video != "ccccccccccc" {
		t.Errorf("downloads %+v", got)
	}
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if mt.n != 2 {
		t.Errorf("%d searches: the explicit Spotify track was looked for", mt.n)
	}
}
