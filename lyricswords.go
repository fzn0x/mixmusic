package main

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The line sung now of lyrics synced word by word lights up word by word:
// each word fills with the accent from its start to its end, from the side
// its line reads from. Words sung stay lit, and words to come keep the
// colour of the lines to come. Under Reduce Motion, a word lights whole as
// it starts.
//
// The line is a text as the others are, which MyGo lays out, sizes and
// reads to screen readers, but in a clear colour: its Draw paints its
// glyphs from a layout of its own of the same text, in the same font and
// width, cut where the song is, the part sung in the accent. The song's
// time moves on from where the player last told it was, at the speed the
// track plays at, so the words fill between the player's reports, every
// 250 ms, in frames that only paint again: the view isn't built for them.

// lyricsText is the text of a line of lyrics, at size in ink, with padX
// and padY around it. The words given, those of the line sung now, light
// up as they are sung.
func (s *shell) lyricsText(c *ui.Context, text string, size float32, ink ui.Color, words []lyrics.Word, padX, padY float32) ui.Element {
	k := &s.lk
	e := ui.Text(c, text).FontSize(size).FontWeight(k.leadWeight()).TextColor(ink).Padding(padY, padX)
	if len(words) == 0 {
		return e
	}
	l := ui.Local(e, "sung", func() sungLine { return sungLine{} })
	clock := l.follow(s.st, c.Now())
	font := ui.Font{Family: k.body, Size: size, Weight: k.leadWeight()}
	dir, still := c.Direction(), c.Preferences().ReduceMotion
	lit, dim := ink, k.text
	return e.Font(k.body).TextColor(ink.Alpha(0)).Draw(func(p *ui.Painter, r ui.Rect) {
		g := l.layout(sungKey{text, font, r.W - 2*padX, dir}, words)
		fill, next := l.light(words, clock.at(p.Now()), still)
		g.paint(p, r.X+padX, r.Y+padY, fill, lit, dim, &l.buf)
		switch {
		case !clock.moving || next < 0:
		case next == 0:
			// A fill sweeps as slowly as a level meter moves.
			p.AnimationFrameRate(60)
		default:
			p.After(clock.wall(next))
		}
	})
}

// sungLine is what the line sung now keeps from frame to frame: the song's
// time it last saw, its layout, and buffers of what it paints.
type sungLine struct {
	clock  songClock
	shaped *sungLayout
	fill   []float32
	buf    stretches
}

// songClock is the time of the song, as the view last saw the player tell
// it, and when: it moves on from there, at speed, while the track plays.
type songClock struct {
	pos    time.Duration
	seen   time.Time
	speed  float64
	moving bool
}

// glide is how far past the time the player last told the song's time
// moves on: the player tells it every 250 ms, and a Spotify device's every
// second. Past that, the track stalled.
const glide = 1500 * time.Millisecond

// follow takes in what the player does, as the view built at now sees
// it, and returns the song's clock.
func (l *sungLine) follow(st player.State, now time.Time) songClock {
	moving := st.Playing && !st.Buffering && !st.Loading && !st.Waiting
	if c := &l.clock; c.seen.IsZero() || st.Position != c.pos || moving != c.moving {
		c.pos, c.seen, c.moving = st.Position, now, moving
	}
	l.clock.speed = st.Speed
	if l.clock.speed <= 0 {
		l.clock.speed = 1
	}
	return l.clock
}

// at returns the song's time at now.
func (c songClock) at(now time.Time) time.Duration {
	if !c.moving {
		return c.pos
	}
	d := min(max(now.Sub(c.seen), 0), glide)
	return c.pos + time.Duration(float64(d)*c.speed)
}

// wall returns how long the song takes to play d of it.
func (c songClock) wall(d time.Duration) time.Duration {
	return time.Duration(float64(d) / c.speed)
}

// light returns how much of each word is lit at the song's time at: 1
// for the words sung, 0 for those to come, and the part sung of those
// being sung, which Reduce Motion (still) lights whole. It returns too in
// how long of the song the light changes next: 0 while words fill, -1
// when it doesn't.
func (l *sungLine) light(words []lyrics.Word, at time.Duration, still bool) ([]float32, time.Duration) {
	l.fill = slices.Grow(l.fill[:0], len(words))[:len(words)]
	next, filling := time.Duration(-1), false
	for i, w := range words {
		start, end := ms(w.StartMs), ms(w.EndMs)
		switch {
		case at < start:
			l.fill[i] = 0
			if next < 0 || start-at < next {
				next = start - at
			}
		case at >= end || still:
			l.fill[i] = 1
		default:
			l.fill[i] = float32(at-start) / float32(end-start)
			filling = true
		}
	}
	if filling {
		next = 0
	}
	return l.fill, next
}

// layout returns the layout of the line, made again when what it's made of
// changed.
func (l *sungLine) layout(key sungKey, words []lyrics.Word) *sungLayout {
	if l.shaped == nil || l.shaped.key != key {
		l.shaped = shapeSung(key, words)
	}
	return l.shaped
}

// sungKey is what the layout of a line sung word by word is made of: its
// text, its font, the width it wraps at and the direction of the view.
type sungKey struct {
	text  string
	font  ui.Font
	width float32
	dir   ui.Direction
}

// sungLayout is the layout of a line sung word by word, as its text lays
// it out, and where its words are on its rows.
type sungLayout struct {
	key  sungKey
	tl   *ui.TextLayout
	rows []sungRow
	// width is that of each word, on all its rows.
	width []float32
}

// sungRow is a row of the layout: its box, whether it reads right to
// left, and the parts of words on it, in the order it reads.
type sungRow struct {
	box   ui.Rect
	rtl   bool
	cells []sungCell
}

// sungCell is the part of a word on a row: the word, the left and right
// of its glyphs, and how wide the word is on the rows before.
type sungCell struct {
	word           int
	x0, x1, before float32
}

// shapeSung lays out the line of key, with its words.
func shapeSung(key sungKey, words []lyrics.Word) *sungLayout {
	g := &sungLayout{key: key, width: make([]float32, len(words))}
	g.tl = ui.ShapeRichTextWith([]ui.Span{{Text: key.text}}, key.font, ui.TextShape{Width: key.width, Direction: key.dir})
	for _, line := range g.tl.Lines() {
		g.rows = append(g.rows, sungRow{box: line.Rect, rtl: line.RTL})
	}
	// The layout counts in UTF-16.
	from, units := 0, 0
	for i, w := range words {
		at := strings.Index(key.text[from:], w.Text)
		if at < 0 {
			break
		}
		start := units + utf16Len(key.text[from:from+at])
		end := start + utf16Len(w.Text)
		from, units = from+at+len(w.Text), end
		for _, r := range g.tl.SelectionRects(ui.TextInputRange{Start: start, End: end}) {
			if row := g.rowAt(r.Y + r.H/2); row >= 0 {
				g.rows[row].cells = append(g.rows[row].cells, sungCell{word: i, x0: r.X, x1: r.X + r.W, before: g.width[i]})
				g.width[i] += r.W
			}
		}
	}
	for i := range g.rows {
		row := &g.rows[i]
		slices.SortStableFunc(row.cells, func(a, b sungCell) int {
			if row.rtl {
				return cmp.Compare(b.x1, a.x1)
			}
			return cmp.Compare(a.x0, b.x0)
		})
	}
	return g
}

// rowAt returns the row at y, -1 when none is.
func (g *sungLayout) rowAt(y float32) int {
	for i, row := range g.rows {
		if y >= row.box.Y && y < row.box.Y+row.box.H {
			return i
		}
	}
	return -1
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// paint draws the line from (x, y), each word as much lit as fill says:
// the part lit in lit, the rest in dim. buf holds the stretches of its
// rows.
func (g *sungLayout) paint(p *ui.Painter, x, y float32, fill []float32, lit, dim ui.Color, buf *stretches) {
	// Glyphs reach past their boxes, as an italic's or an accent's.
	reach := g.key.font.Size
	buf.on = false
	for i := range g.rows {
		top, bottom := g.rows[i].box.Y, g.rows[i].box.Y+g.rows[i].box.H
		if i == 0 {
			top -= reach
		}
		if i == len(g.rows)-1 {
			bottom += reach
		}
		g.stretchesOf(i, fill, reach, buf)
		for _, st := range buf.out {
			ink := dim
			if st.lit {
				ink = lit
			}
			p.Clip(ui.Rect{X: x + st.x0, Y: y + top, W: st.x1 - st.x0, H: bottom - top}, 0, func() { p.TextLayout(g.tl, x, y, ink) })
		}
	}
}

// stretches are those of a row painted lit or not, and whether the last
// is lit.
type stretches struct {
	out []stretch
	on  bool
}

// stretch is a stretch of a row painted lit or not, from x0 to x1.
type stretch struct {
	x0, x1 float32
	lit    bool
}

// add adds the stretch from x0 to x1, which goes on the last when they
// touch and light alike.
func (s *stretches) add(x0, x1 float32, lit bool) {
	if x1 <= x0 {
		return
	}
	s.on = lit
	if n := len(s.out); n > 0 && s.out[n-1].lit == lit && (s.out[n-1].x1 == x0 || s.out[n-1].x0 == x1) {
		last := &s.out[n-1]
		last.x0, last.x1 = min(last.x0, x0), max(last.x1, x1)
		return
	}
	s.out = append(s.out, stretch{x0, x1, lit})
}

// stretchesOf makes s the stretches of row i, as lit as fill says, reaching
// out past its glyphs by reach: each word's goes from its start to the
// next word's, so what lies between words lights with the word before,
// and a row's start with its first word. A row without words is as lit as
// what came before it, as s.on tells.
func (g *sungLayout) stretchesOf(i int, fill []float32, reach float32, s *stretches) {
	s.out = s.out[:0]
	row := g.rows[i]
	left, right := row.box.X-reach, row.box.X+row.box.W+reach
	if len(row.cells) == 0 {
		s.add(left, right, s.on)
		return
	}
	for k, c := range row.cells {
		f := fill[c.word]
		part := min(max(f*g.width[c.word]-c.before, 0), c.x1-c.x0)
		if row.rtl {
			from, to := c.x1, left
			if k == 0 {
				from = right
			}
			if k+1 < len(row.cells) {
				to = row.cells[k+1].x1
			}
			cut := c.x1 - part
			switch {
			case f >= 1:
				s.add(to, from, true)
			case part <= 0:
				s.add(to, from, false)
			default:
				s.add(cut, from, true)
				s.add(to, cut, false)
			}
			continue
		}
		from, to := c.x0, right
		if k == 0 {
			from = left
		}
		if k+1 < len(row.cells) {
			to = row.cells[k+1].x0
		}
		cut := c.x0 + part
		switch {
		case f >= 1:
			s.add(from, to, true)
		case part <= 0:
			s.add(from, to, false)
		default:
			s.add(from, cut, true)
			s.add(cut, to, false)
		}
	}
}
