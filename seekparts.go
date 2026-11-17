package main

// The parts of the track that plays which the player passes over, as
// SponsorBlock marks them (#548), on every seek bar MixMusic draws: the
// player bar's and the now playing page's, the mini player's and the
// notch's, and the remote page's, of the track that plays on the computer
// it controls (#735). Each part is a band in a colour of the theme for its
// kind, and the highlight a dot; the pointer over one tells its kind and
// times. What each bar draws comes from here.

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/locales"
)

// seekMarks are what a seek bar marks of a track: the parts the player
// passes over, and the highlight, 0 for none.
type seekMarks struct {
	parts     []player.Part
	highlight time.Duration
}

// any reports whether m marks anything.
func (m seekMarks) any() bool { return len(m.parts) > 0 || m.highlight > 0 }

// marks are the marks of the track that plays here.
func (s *shell) marks() seekMarks { return seekMarks{parts: s.st.Parts, highlight: s.st.Highlight} }

// hasMarks reports whether the seek bars mark anything of the track that
// plays.
func (s *shell) hasMarks() bool { return s.marks().any() }

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
	s.marks().paint(p, &s.lk, line, dur, rtl)
}

// paint paints m, of a track of a length of dur, in k's colours, as
// paintMarks paints the marks of the track that plays.
func (m seekMarks) paint(p *ui.Painter, k *look, line ui.Rect, dur time.Duration, rtl bool) {
	if dur <= 0 || line.W <= 0 {
		return
	}
	at := func(t time.Duration) float32 {
		f := float32(min(max(float64(t)/float64(dur), 0), 1))
		if rtl {
			f = 1 - f
		}
		return line.X + line.W*f
	}
	for _, part := range m.parts {
		x0, x1 := at(part.From), at(part.To)
		if x1 < x0 {
			x0, x1 = x1, x0
		}
		w := max(x1-x0, 2)
		p.Fill(ui.Rect{X: x0, Y: line.Y, W: w, H: line.H}, k.partColor(part.Kind).Alpha(0.9), min(line.H/2, w/2))
	}
	if h := m.highlight; h > 0 && h < dur {
		d := line.H + 3
		p.Fill(ui.Rect{X: at(h) - d/2, Y: line.Y + line.H/2 - d/2, W: d, H: d}, k.danger, d/2)
	}
}

// markAt returns what is marked at f of a seek bar of the track that
// plays, from 0 at its start to 1 at its end, of a length of dur, as the
// pointer tells it: the part, or the highlight within near of it.
func (s *shell) markAt(f float64, dur time.Duration, near float64) (string, bool) {
	return s.marks().at(f, dur, near)
}

// at is what m marks at f of a seek bar, as markAt tells of the track that
// plays.
func (m seekMarks) at(f float64, dur time.Duration, near float64) (string, bool) {
	if dur <= 0 {
		return "", false
	}
	t := time.Duration(f * float64(dur))
	if h := m.highlight; h > 0 && (t-h).Abs() <= time.Duration(near*float64(dur)) {
		return locales.SeekHighlightTip(partKindName(sponsorblock.Highlight), clock(h)), true
	}
	for _, part := range m.parts {
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
	s.marks().tip(bar, dur, pad, rtl)
}

// tip has a seek bar tell what m marks under the pointer, as markTip does
// of the track that plays.
func (m seekMarks) tip(bar ui.Element, dur time.Duration, pad float32, rtl bool) {
	if !m.any() {
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
	if tip, ok := m.at(f, dur, float64(6/w)); ok {
		bar.Tooltip(tip)
	}
}
