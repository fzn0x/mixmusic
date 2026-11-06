package player

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestNoRadioAfterLocalTrack stops at the end of a queue that ends on a
// track on this computer, with autoplay on: its radio would send it to a
// service.
func TestNoRadioAfterLocalTrack(t *testing.T) {
	h := newResumeHarness(t, nil, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{track("r1"), track("r2")}
	})
	local := track("memo")
	local.ID, local.Source = "file:L3NvbmdzL21lbW8ud2F2", media.SourceFile
	h.p.PlayTracks([]media.Track{local}, 0)
	h.wait("memo playing", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == local.ID && p.s.Playing && !p.s.Buffering
	})
	h.p.Ended()
	h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing })
	if n := len(h.p.State().Queue); n != 1 {
		t.Errorf("the radio added %d tracks", n-1)
	}
}
