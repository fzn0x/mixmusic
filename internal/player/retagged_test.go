package player

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestRetagged gives the entries of the queue of a track whose file's tags
// changed its new title, artists and album, while it plays on.
func TestRetagged(t *testing.T) {
	h := newHarness(t, Saved{})
	song := track("memo")
	other := track("other")
	h.p.PlayTracks([]media.Track{song, other, song}, 0)
	h.wait("the song playing", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == song.ID && p.s.Playing
	})
	changed := song
	changed.Title, changed.Artists, changed.Album = "Memo, renamed", []media.Ref{{Name: "Ada Field"}}, &media.Ref{Name: "Kites"}
	h.p.Retagged([]media.Track{changed})
	h.wait("the queue retagged", func(p *Player) bool { return p.s.Queue[0].Track.Title == "Memo, renamed" })
	q := h.p.State().Queue
	if q[2].Track.Title != "Memo, renamed" || q[1].Track.Title != other.Title || q[0].Track.Artists[0].Name != "Ada Field" {
		t.Errorf("the queue is %+v", q)
	}
	if st := h.p.State(); st.Index != 0 || !st.Playing || h.stream.count("load") != 1 {
		t.Errorf("the player went from the song: %+v, loads %d", st, h.stream.count("load"))
	}
}
