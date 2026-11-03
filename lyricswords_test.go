package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lanterns are made-up lyrics synced word by word, and lanternLines the
// same synced line by line.
var (
	lanterns = []lyrics.Line{
		{TimeMs: 1000, Text: "Lanterns drifting home", Words: []lyrics.Word{word("Lanterns", 1000, 2000), word("drifting", 2000, 3000), word("home", 3000, 4000)}},
		{TimeMs: 60000, Text: "Quiet harbour", Words: []lyrics.Word{word("Quiet", 60000, 60500), word("harbour", 60500, 61000)}},
	}
	lanternLines = []lyrics.Line{{TimeMs: 1000, Text: "Lanterns drifting home"}, {TimeMs: 60000, Text: "Quiet harbour"}}
)

// word is a word sung from start to end, in milliseconds.
func word(text string, start, end int) lyrics.Word {
	return lyrics.Word{Text: text, StartMs: start, EndMs: end}
}

// sungIn shows made-up lyrics in the language lang in the lyrics panel,
// the track playing at pos.
func sungIn(t *testing.T, lang string, lines []lyrics.Line, pos time.Duration) (*shell, *ui.Tester) {
	return sungOn(t, "/lyrics", lang, lines, pos)
}

// sungOn shows made-up lyrics as sungIn does, in the lyrics panel or on
// the page at path.
func sungOn(t *testing.T, path, lang string, lines []lyrics.Line, pos time.Duration) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: lang, Lines: lines}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
	tt := testerOf(t, s, 1280, 900)
	s.open(path)
	s.drain()
	tt.Frame()
	// The page slides in.
	tt.Advance(time.Second)
	s.p.Time(pos, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == pos })
	tt.Frame()
	return s, tt
}

// wordsAt returns where the words of the line text show in the panel,
// laid out apart from it: the words given, from the left.
func wordsAt(t *testing.T, s *shell, tt *ui.Tester, text string, words ...string) []ui.Rect {
	t.Helper()
	return wordsOf(t, s, tt, s.lk.bodySize(), text, words...)
}

// wordsOf is wordsAt of a line of lyrics at size.
func wordsOf(t *testing.T, s *shell, tt *ui.Tester, size float32, text string, words ...string) []ui.Rect {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q doesn't show: %q", text, tt.Texts())
	}
	k := &s.lk
	font := ui.Font{Family: k.body, Size: size, Weight: k.leadWeight()}
	width := func(s string) float32 { w, _ := ui.ShapeText(s, font, 0).Size(); return w }
	var out []ui.Rect
	before := ""
	for _, w := range words {
		x0, x1 := width(before), width(before+w)
		// The panel pads lines by 24 and 4.
		out = append(out, ui.Rect{X: r.X + 24 + x0 + 1, Y: r.Y + 4, W: x1 - x0 - 2, H: r.H - 8})
		before += w + " "
	}
	return out
}

// inkIn counts the pixels of r close to c.
func inkIn(img *image.RGBA, r ui.Rect, c ui.Color) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			p := img.RGBAAt(x, y)
			d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
			if d(p.R, c.R)+d(p.G, c.G)+d(p.B, c.B) < 60 {
				n++
			}
		}
	}
	return n
}

// lit tells whether r shows lit, in the accent, or dim, in the colour of
// the lines to come, and fails the test when it shows neither. Glyphs
// drawn with subpixel smoothing have fringes of other colours: their ink,
// summed, is that of the text, on the ground above r, which the line
// pads.
func lit(t *testing.T, s *shell, img *image.RGBA, r ui.Rect, what string) bool {
	t.Helper()
	ground := img.RGBAAt(int(r.X+r.W/2), int(r.Y)-3)
	off := func(c color.RGBA) [3]float64 {
		return [3]float64{float64(c.R) - float64(ground.R), float64(c.G) - float64(ground.G), float64(c.B) - float64(ground.B)}
	}
	var ink [3]float64
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			for i, v := range off(img.RGBAAt(x, y)) {
				ink[i] += v
			}
		}
	}
	// ink is a of the accent and b of the text, at least squares.
	u, v := off(color.RGBA(nrgba(s.lk.accent))), off(color.RGBA(nrgba(s.lk.text)))
	dot := func(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
	uu, uv, vv, iu, iv := dot(u, u), dot(u, v), dot(v, v), dot(ink, u), dot(ink, v)
	det := uu*vv - uv*uv
	a, b := (iu*vv-iv*uv)/det, (iv*uu-iu*uv)/det
	switch {
	case a > 0 && b < a/4:
		return true
	case b > 0 && a < b/4:
		return false
	}
	t.Fatalf("%s is neither lit nor dim: %.1f of the accent, %.1f of the text", what, a, b)
	return false
}

// TestSungWordsLight lights the words of the line sung now as they are
// sung: those sung, and the part sung of the word being sung, as the song
// plays on between the player's reports.
func TestSungWordsLight(t *testing.T) {
	s, tt := sungIn(t, "en", lanterns, 2500*time.Millisecond)
	at := wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")
	half := func(r ui.Rect, right bool) ui.Rect {
		r.W *= 0.4
		if right {
			r.X += r.W * 1.5
		}
		return r
	}
	img := tt.Image()
	if !lit(t, s, img, at[0], "Lanterns, sung") || !lit(t, s, img, half(at[1], false), "drifting's start") {
		t.Error("what was sung isn't lit")
	}
	if lit(t, s, img, half(at[1], true), "drifting's end") || lit(t, s, img, at[2], "home, to come") {
		t.Error("what is to come is lit")
	}

	// A second on, the player has said nothing since.
	tt.Advance(time.Second)
	img = tt.Image()
	if !lit(t, s, img, at[1], "drifting, sung") || !lit(t, s, img, half(at[2], false), "home's start") || lit(t, s, img, half(at[2], true), "home's end") {
		t.Error("the words don't light as the song plays on")
	}
}

// TestSungWordsOnThePage lights the words on the now playing page too, at
// its size.
func TestSungWordsOnThePage(t *testing.T) {
	s, tt := sungOn(t, "/now-playing", "en", lanterns, 2500*time.Millisecond)
	at := wordsOf(t, s, tt, s.lk.size(2), "Lanterns drifting home", "Lanterns", "drifting", "home")
	if img := tt.Image(); !lit(t, s, img, at[0], "Lanterns") || lit(t, s, img, at[2], "home") {
		t.Error("the words don't light as sung on the page")
	}
}

// TestSungWordsStill lights each word whole as it starts under Reduce
// Motion.
func TestSungWordsStill(t *testing.T) {
	s, tt := sungIn(t, "en", lanterns, 2100*time.Millisecond)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.drain()
	tt.Frame()
	at := wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")
	img := tt.Image()
	if !lit(t, s, img, at[0], "Lanterns") || !lit(t, s, img, at[1], "drifting") || lit(t, s, img, at[2], "home") {
		t.Error("words don't light whole as they start")
	}
}

// TestSungWordsSettle asks for no frames once the words of the line are
// sung, or while the track is paused.
func TestSungWordsSettle(t *testing.T) {
	s, tt := sungIn(t, "en", lanterns, 4500*time.Millisecond)
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("frames go on with the line sung")
	}
	s.p.Time(2500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 2500*time.Millisecond })
	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("frames go on while paused")
	}
	at := wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")
	if img := tt.Image(); lit(t, s, img, at[2], "home") {
		t.Error("the words light on while paused")
	}
}

// TestSungWordsAsLines shows a line whose words are all sung as a line
// synced line by line shows, glyph for glyph, on one row or wrapped on
// several; and lyrics synced line by line as they did: the line sung now
// lit whole.
func TestSungWordsAsLines(t *testing.T) {
	s, tt := sungIn(t, "en", lanternLines, 2500*time.Millisecond)
	at := wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")
	if img := tt.Image(); !lit(t, s, img, at[0], "Lanterns") || !lit(t, s, img, at[2], "home") {
		t.Error("the line sung now isn't lit whole")
	}

	long := "Lanterns drifting home across the quiet harbour, while the long night comes down"
	var words []lyrics.Word
	for i, w := range strings.Fields(long) {
		words = append(words, word(w, 1000+100*i, 1100+100*i))
	}
	for _, line := range []lyrics.Line{lanterns[0], {TimeMs: 1000, Text: long, Words: words}} {
		shown := func(lines []lyrics.Line) (*image.RGBA, ui.Rect) {
			_, tt := sungIn(t, "en", lines, 4500*time.Millisecond)
			r, ok := tt.Find(line.Text)
			if !ok {
				t.Fatalf("%q doesn't show: %q", line.Text, tt.Texts())
			}
			return tt.Image().SubImage(image.Rect(int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))).(*image.RGBA), r
		}
		plain := line
		plain.Words = nil
		whole, r := shown([]lyrics.Line{plain, lanterns[1]})
		sung, r2 := shown([]lyrics.Line{line, lanterns[1]})
		if r != r2 {
			t.Fatalf("%q moved: %v, %v", line.Text, r2, r)
		}
		if r.H < 2*s.lk.bodySize() && line.Text == long {
			t.Fatalf("%q doesn't wrap: %v", line.Text, r)
		}
	pixels:
		for py := whole.Rect.Min.Y; py < whole.Rect.Max.Y; py++ {
			for px := whole.Rect.Min.X; px < whole.Rect.Max.X; px++ {
				if a, b := whole.RGBAAt(px, py), sung.RGBAAt(px, py); a != b {
					t.Errorf("%q sung word by word differs at (%d, %d): %v, %v", line.Text, px, py, b, a)
					break pixels
				}
			}
		}
	}
}

// TestSungWordsRomanized lights a line shown in Latin letters in their
// place as a whole, as their words aren't those sung, and the line as
// written word by word with them under it.
func TestSungWordsRomanized(t *testing.T) {
	lines := []lyrics.Line{
		{TimeMs: 1000, Text: "종이 배", Words: []lyrics.Word{word("종이", 1000, 2000), word("배", 2000, 3000)}},
		{TimeMs: 60000, Text: "바다", Words: []lyrics.Word{word("바다", 60000, 61000)}},
	}
	tr := translated{lyrics.Translation{From: "ko", To: "en", Lines: []lyrics.TranslatedLine{
		{Text: "Paper boat", Romanized: "jongi bae"}, {Text: "Sea", Romanized: "bada"}}}, true}
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "ko", Lines: lines}}
	s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = &loaded[translated]{done: true, value: tr}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeOnly}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
	s.p.Time(1500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 1500*time.Millisecond })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	s.drain()
	tt.Frame()
	at := wordsAt(t, s, tt, "jongi bae", "jongi", "bae")
	if img := tt.Image(); !lit(t, s, img, at[1], "bae") {
		t.Error("a line in Latin letters doesn't light whole")
	}

	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeUnder}
	s.drain()
	tt.Frame()
	under, ok := tt.Find("jongi bae")
	if !ok {
		t.Fatalf("no Latin letters under the line: %q", tt.Texts())
	}
	if img := tt.Image(); inkIn(img, under, s.lk.accent) > 0 {
		t.Error("the Latin letters under the line light")
	}
	if _, ok := tt.Find("종이 배"); !ok {
		t.Fatalf("the line doesn't show: %q", tt.Texts())
	}
}

// TestSungLight tells how much of each word is lit as the song plays: whole
// as it starts under Reduce Motion, and when the light changes next.
func TestSungLight(t *testing.T) {
	words := []lyrics.Word{word("one", 1000, 2000), word("two", 2000, 2000), word("three", 2500, 3500)}
	var l sungLine
	for _, c := range []struct {
		at    int
		still bool
		fill  []float32
		next  time.Duration
	}{
		{500, false, []float32{0, 0, 0}, 500 * time.Millisecond},
		{1250, false, []float32{0.25, 0, 0}, 0},
		{2000, false, []float32{1, 1, 0}, 500 * time.Millisecond},
		{3000, false, []float32{1, 1, 0.5}, 0},
		{4000, false, []float32{1, 1, 1}, -1},
		{1250, true, []float32{1, 0, 0}, 750 * time.Millisecond},
		{2600, true, []float32{1, 1, 1}, -1},
	} {
		fill, next := l.light(words, ms(c.at), c.still)
		for i := range fill {
			if fill[i] != c.fill[i] || next != c.next {
				t.Errorf("at %d ms (still %v): %v, next in %v; want %v, next in %v", c.at, c.still, fill, next, c.fill, c.next)
				break
			}
		}
	}
}

// TestSongClock moves the song's time on from what the player told, at
// its speed, while it plays, for a while at most.
func TestSongClock(t *testing.T) {
	now := time.Now()
	var l sungLine
	c := l.follow(player.State{Playing: true, Position: 10 * time.Second, Speed: 1.5}, now)
	for d, want := range map[time.Duration]time.Duration{
		0: 10 * time.Second, time.Second: 11500 * time.Millisecond, 10 * time.Second: 10*time.Second + glide*3/2, -time.Second: 10 * time.Second,
	} {
		if got := c.at(now.Add(d)); got != want {
			t.Errorf("%v on: %v, want %v", d, got, want)
		}
	}
	// The same position seen later keeps when the player told it.
	if c := l.follow(player.State{Playing: true, Position: 10 * time.Second, Speed: 1.5}, now.Add(time.Second)); c.at(now.Add(time.Second)) != 11500*time.Millisecond {
		t.Errorf("seen again: %+v", c)
	}
	for _, st := range []player.State{{Position: 12 * time.Second}, {Playing: true, Buffering: true, Position: 12 * time.Second}} {
		if c := l.follow(st, now); c.at(now.Add(time.Second)) != 12*time.Second {
			t.Errorf("%+v: the time moves", st)
		}
	}
}

// TestSungStretches cuts the rows of a line where the words sung end:
// each word's stretch reaches to the next word's, and a word on two rows
// lights the first before the second; rows reading right to left light
// from the right.
func TestSungStretches(t *testing.T) {
	g := &sungLayout{
		rows: []sungRow{
			{box: ui.Rect{W: 100, H: 20}, cells: []sungCell{{word: 0, x0: 0, x1: 40}, {word: 1, x0: 50, x1: 100}}},
			{box: ui.Rect{Y: 20, W: 60, H: 20}, cells: []sungCell{{word: 1, x0: 0, x1: 30, before: 50}, {word: 2, x0: 40, x1: 60}}},
			{box: ui.Rect{Y: 40, W: 60, H: 20}},
			{box: ui.Rect{Y: 60, X: 40, W: 60, H: 20}, rtl: true, cells: []sungCell{{word: 3, x0: 70, x1: 100}, {word: 4, x0: 40, x1: 60}}},
		},
		width: []float32{40, 80, 20, 30, 20},
	}
	litWidth := func(row int, fill []float32) float32 {
		var s stretches
		s.on = fill[2] >= 1 // as the row before ends
		g.stretchesOf(row, fill, 0, &s)
		w := float32(0)
		for _, st := range s.out {
			if st.lit {
				w += st.x1 - st.x0
			}
		}
		return w
	}
	for _, c := range []struct {
		row  int
		fill []float32
		want float32
	}{
		{0, []float32{0, 0, 0, 0, 0}, 0},
		{0, []float32{0.5, 0, 0, 0, 0}, 20},
		{0, []float32{1, 0, 0, 0, 0}, 50},
		{0, []float32{1, 0.25, 0, 0, 0}, 70},
		{0, []float32{1, 0.75, 0, 0, 0}, 100},
		{1, []float32{1, 0.75, 0, 0, 0}, 10},
		{1, []float32{1, 1, 0, 0, 0}, 40},
		{1, []float32{1, 1, 1, 0, 0}, 60},
		{2, []float32{1, 1, 1, 0, 0}, 60},
		{2, []float32{1, 1, 0.5, 0, 0}, 0},
		{3, []float32{1, 1, 1, 0.5, 0}, 15},
		{3, []float32{1, 1, 1, 1, 0}, 40},
		{3, []float32{1, 1, 1, 1, 1}, 60},
	} {
		if got := litWidth(c.row, c.fill); got != c.want {
			t.Errorf("row %d lit by %v: %v wide, want %v", c.row, c.fill, got, c.want)
		}
	}
}
