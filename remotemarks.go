package main

// The remote page marks the parts the player of the computer it controls
// passes over, and its highlight, as that computer's states tell them
// (#735), on its seek bar, as the seek bars of this computer's player mark
// its own (seekparts.go); Jump to the highlight seeks there. A computer
// whose MixMusic is older tells none: the page's seek bar is the theme's
// slider, as it was.

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/locales"
)

// remoteMarks are the marks of the track that plays on the computer
// controlled, as st tells them: none of a live stream, or of a track of no
// length, whose seek bar doesn't seek.
func remoteMarks(st remote.State) seekMarks {
	var m seekMarks
	if st.Track == nil || st.Live || st.DurationMs <= 0 {
		return m
	}
	for _, p := range st.Parts {
		from, to := time.Duration(p.FromMs)*time.Millisecond, time.Duration(p.ToMs)*time.Millisecond
		if p.Kind == sponsorblock.Highlight {
			m.highlight = from
			continue
		}
		m.parts = append(m.parts, player.Part{Kind: p.Kind, From: from, To: to})
	}
	return m
}

// remoteSeekSlider is the remote page's seek bar of *at, from 0 to most
// seconds: the theme's slider, or, while m marks something, the wavy
// slider lying flat (wavy.go), which looks as the theme's does, with m's
// marks, which tell what they are under the pointer.
func (s *shell) remoteSeekSlider(c *ui.Context, at *float64, most float64, m seekMarks) ui.Element {
	if !m.any() {
		return ui.Slider(c.Key("remote-seek"), at, 0, most)
	}
	dur := time.Duration(most * float64(time.Second))
	paint := func(p *ui.Painter, line ui.Rect, rtl bool) { m.paint(p, &s.lk, line, dur, rtl) }
	sl := wavySliderMarked(c.Key("remote-seek"), at, 0, most, false, paint)
	m.tip(sl, dur, c.Theme().Space(5)/2, c.Direction() == ui.RightToLeft)
	return sl
}

// remoteHighlight is Jump to the highlight of the track that plays on the
// computer id, when its state tells one: it seeks there, which the seek
// bar shows until the state of it comes.
func (s *shell) remoteHighlight(c *ui.Context, id string, st remote.State) {
	h := remoteMarks(st).highlight
	if h <= 0 {
		return
	}
	if s.roundButton(c, "remote-highlight", "highlight", locales.PlayerJumpToHighlight(), false, st.Loading).Clicked() {
		s.remoteDo(id, remote.Command{Op: remote.OpSeek, PositionMs: h.Milliseconds()})
		s.remote.seekTo, s.remote.seekAt = h.Seconds(), c.Now()
	}
}
