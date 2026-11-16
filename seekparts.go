package main

// The parts of the track that plays which the player passes over, as
// SponsorBlock marks them (#548), on every seek bar MixMusic draws: the
// player bar's and the now playing page's, the mini player's and the
// notch's. Each part is a band in a colour of the theme for its kind, and
// the highlight a dot; the pointer over one tells its kind and times.
// What each bar draws comes from here.

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/locales"
)

// hasMarks reports whether the seek bars mark anything of the track that
// plays.
func (s *shell) hasMarks() bool { return len(s.st.Parts) > 0 || s.st.Highlight > 0 }

// partColor is the colour of the theme a part of kind is marked in.
func (k *look) partColor(kind sponsorblock.Kind) ui.Color {
	switch kind {
	case sponsorblock.Sponsor:
		return k.success
	case sponsorblock.NonMusic:
		return k.tiles[0]
	case sponsorblock.SelfPromo:
		return k.tiles[1]
	case sponsorblock.Interaction:
		return k.tiles[2]
	case sponsorblock.Filler:
		return k.muted
	}
	// Intros, outros, previews and hooks: what leads in or out.
	return k.tiles[3]
}

// paintMarks paints the marks of the track that plays, of a length of dur,
// on a seek bar whose time runs along line, its track's box, from its
// start to its end: right to left when rtl.
func (s *shell) paintMarks(p *ui.Painter, line ui.Rect, dur time.Duration, rtl bool) {
	if dur <= 0 || line.W <= 0 {
		return
	}
	k := &s.lk
	at := func(t time.Duration) float32 {
		f := float32(min(max(float64(t)/float64(dur), 0), 1))
		if rtl {
			f = 1 - f
		}
		return line.X + line.W*f
	}
	for _, part := range s.st.Parts {
		x0, x1 := at(part.From), at(part.To)
		if x1 < x0 {
			x0, x1 = x1, x0
		}
		w := max(x1-x0, 2)
		p.Fill(ui.Rect{X: x0, Y: line.Y, W: w, H: line.H}, k.partColor(part.Kind).Alpha(0.9), min(line.H/2, w/2))
	}
	if h := s.st.Highlight; h > 0 && h < dur {
		d := line.H + 3
		p.Fill(ui.Rect{X: at(h) - d/2, Y: line.Y + line.H/2 - d/2, W: d, H: d}, k.danger, d/2)
	}
}

// markAt returns what is marked at f of a seek bar of the track that
// plays, from 0 at its start to 1 at its end, of a length of dur, as the
// pointer tells it: the part, or the highlight within near of it.
func (s *shell) markAt(f float64, dur time.Duration, near float64) (string, bool) {
	if dur <= 0 {
		return "", false
	}
	t := time.Duration(f * float64(dur))
	if h := s.st.Highlight; h > 0 && (t-h).Abs() <= time.Duration(near*float64(dur)) {
		return locales.SeekHighlightTip(partKindName(sponsorblock.Highlight), clock(h)), true
	}
	for _, part := range s.st.Parts {
		if t >= part.From && t < part.To {
			return partTip(part), true
		}
	}
	return "", false
}

// partTip tells a part's kind and times: "Sponsor, 1:20 to 2:05".
func partTip(part player.Part) string {
	return locales.SeekPartTip(partKindName(part.Kind), clock(part.From), clock(part.To))
}

// markTip has a seek bar tell, under the pointer, what is marked there:
// pad is how far within its box its time starts and ends, which runs right
// to left when rtl.
func (s *shell) markTip(bar ui.Element, dur time.Duration, pad float32, rtl bool) {
	if !s.hasMarks() {
		return
	}
	x, _, over := bar.PointerPosition()
	w := bar.Bounds().W - 2*pad
	if !over || w <= 0 {
		return
	}
	f := float64(min(max((x-pad)/w, 0), 1))
	if rtl {
		f = 1 - f
	}
	if tip, ok := s.markAt(f, dur, float64(6/w)); ok {
		bar.Tooltip(tip)
	}
}
