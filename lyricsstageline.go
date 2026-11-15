package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// stageLine is a line of lyrics as the stage shows it.
type stageLine struct {
	// style is the style the stage is in.
	style lyricStyle
	// index is the line's place, and active the place of the line sung now:
	// -1 before the first is sung.
	index, active int
	line          lyrics.Line
	// text, roman and meaning are what the line shows: its words, those in
	// Latin letters under it, and its translation (lyricsExtra.line).
	text, roman, meaning string
	// words are the line's words with their times, while it is the one sung
	// now.
	words []lyrics.Word
	// synced tells whether the lyrics follow the song, and reading whether
	// the user scrolled them lately, to read back.
	synced, reading bool
	// glide is set for the line sung when the song just went on to it from a
	// line near, which the lyrics glide to the middle for, rather than jump.
	glide bool
	// ball is set when the ball is on and the lyrics have words with times:
	// every line keeps room for it, and the line sung has it.
	ball bool
	// next is the line after, which the ball goes on to.
	next ballNext
}

// ballNext is the line after the one sung, as the ball goes on to it: set
// where that line has its words timed, with the text it shows and its first
// word.
type ballNext struct {
	ok   bool
	text string
	word lyrics.Word
}

// stageLine shows a line of lyrics in the style: the line sung now whole,
// lit word by word with a glow, where the style has one, and the others
// dimmer and blurrier the farther they lie from it. A line the user clicks
// plays the song from there.
func (s *shell) stageLine(c *ui.Context, a stageLine) {
	active := a.synced && a.index == a.active
	opacity, blur := float32(0.92), float32(0)
	if a.synced {
		// Before the first line is sung, all lie ahead: the first is the
		// nearest.
		dist := a.index - a.active
		if dist < 0 {
			dist = -dist
		}
		opacity, blur = a.style.opacity(dist), a.style.blurAt(dist)
		if a.reading {
			// The user reads back: nothing blurs, and all can be read.
			opacity, blur = max(opacity, 0.7), 0
		}
	}
	col := s.stageLineView(c, stageLineView{
		style: a.style, scale: 1, text: a.text, roman: a.roman, meaning: a.meaning,
		words: a.words, active: active, opacity: opacity, blur: blur, ball: a.ball,
		rig: &s.stage.ball, line: a.index, motion: s.set.LyricsBallMotion, next: a.next,
	})
	if a.glide && active {
		// Once, in the frame the song goes on to this line: the lyrics glide
		// to it, and stop where it is in the middle.
		col.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter, Animation: &stageGlide})
		s.stage.glided = true
	}
	if a.synced && !s.st.Loading {
		col.Cursor(ui.CursorPointer)
		if col.Clicked() {
			s.p.Seek(ms(a.line.TimeMs))
		}
	}
}

// stageLineView is what a line of lyrics looks like in the stage's styles,
// wherever it shows: the stage's lyrics, and the preview in Settings.
type stageLineView struct {
	style lyricStyle
	// scale multiplies the line's sizes, its blur and its glow: 1 on the
	// stage, less in the preview.
	scale                float32
	text, roman, meaning string
	// words are the words of the line sung, with their times.
	words  []lyrics.Word
	active bool
	// opacity and blur are where the line's distance from the line sung puts
	// it, which it moves to, rather than jumps.
	opacity, blur float32
	// state is the state of a player to follow in place of the real one.
	state *player.State
	// key, when set, keys the line, so that it keeps what it moves to (its
	// fade and blur) while the lines around it change places.
	key any
	// ball is set when every line keeps room above it for the ball, and the
	// line sung has it hop over its words, for the rig to paint (rig), in the
	// motion between lines the user chose, from the line's place, line, to the
	// line after, next.
	ball   bool
	rig    *ballRig
	line   int
	motion settings.LyricBallMotion
	next   ballNext
	// feed is what the line gives the rig, set as the line is built.
	feed *ballFeed
}

// stageLineView builds a line of lyrics in the style, and returns it. The
// fade and the blur move to where the distance puts them as the lyrics
// glide, rather than jump; a line built for the first time starts there.
// With the ball on, every line keeps room above it for the ball, so that
// the lyrics don't change height as the line sung changes.
func (s *shell) stageLineView(c *ui.Context, v stageLineView) ui.Element {
	k, st := &s.lk, v.style
	size := k.size(st.size*float32(s.set.LyricsSize)) * v.scale
	pad := 0.14 * size
	top := pad
	if v.ball {
		top += 0.42 * size
	}
	cc := c
	if v.key != nil {
		cc = c.Key(v.key)
	}
	col := ui.Column(cc).Padding(top, 28*v.scale, pad).Gap(0.1 * size)
	if v.ball && v.active && v.rig != nil {
		// The line gives the ball's rig where its words lie, and the box of its
		// column tells where the next line begins.
		v.feed = &ballFeed{
			rig: v.rig, look: ballLook{ink: stageInk, radius: min(max(0.11*size, 4), 12)},
			line: v.line, motion: v.motion, top: top, col: col,
			hasNext: v.next.ok, nextText: v.next.text, nextWord: v.next.word,
		}
	}
	opacity := col.Animate("opacity", v.opacity, stageFade)
	blur := col.Animate("blur", v.blur*v.scale, stageFade)
	col.Opacity(opacity)
	if blur > 0.05 {
		col.Blur(blur)
		if !v.active {
			// A line that waits holds the layer it blurred, and is not drawn
			// again until its look changes.
			col.Cache()
		}
	}
	col.Children(func() {
		s.stageLineText(c, v, size)
		if v.roman != "" {
			ui.Text(c, v.roman).Font(st.family).FontSize(0.55 * size).FontWeight(400).TextColor(stageDim)
		}
		if v.meaning != "" {
			ui.Text(c, v.meaning).Font(st.family).FontSize(0.5 * size).FontWeight(400).TextColor(stageDim)
		}
	})
	return col
}

// stageLineText is the words of a line, size big. The line sung now lights
// word by word when its words have times (sungText), has the ball hop over
// them when the ball is on, and, where the style has a glow, has a blurred
// copy of its lit part under it, as the Terminal theme's headings have,
// which a blur of the line's own drawing (Blur) makes: the glow follows the
// words as they light. The ball is the crisp copy's alone.
func (s *shell) stageLineText(c *ui.Context, v stageLineView, size float32) {
	st := v.style
	font := ui.Font{Family: st.family, Size: size, Weight: st.weight, LineHeight: st.leading}
	plain := func(ink ui.Color) ui.Element {
		return ui.Text(c, v.text).Font(st.family).FontSize(size).FontWeight(st.weight).LineHeight(st.leading).TextColor(ink)
	}
	if !v.active {
		plain(stageInk)
		return
	}
	// draw is the line with its sung part in lit and the rest in dim. Its
	// layout of the words has no letter spacing, nor does the line's text.
	draw := func(lit, dim ui.Color, ball bool) ui.Element {
		if len(v.words) == 0 {
			return plain(lit)
		}
		e := ui.Text(c, v.text).FontSize(size).LineHeight(st.leading).TextColor(lit)
		o := sungOptions{state: v.state}
		if ball && v.feed != nil {
			o.ball = v.feed
		}
		return s.sungTextWith(c, e, v.text, font, lit, dim, v.words, 0, 0, o)
	}
	ui.Box(c).Children(func() {
		if st.glow > 0 {
			// The glow is the blur's Gaussian, of half the glow's reach, as a
			// text-shadow's is.
			draw(stageInk.Alpha(0.6), ui.Color{}, false).Absolute().Top(0).Right(0).Left(0).Blur(st.glow * v.scale / 2).PassThrough()
		}
		draw(stageInk, stageInk.Alpha(0.5), true)
	})
}
