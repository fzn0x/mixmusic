package main

import (
	"context"
	"image"
	"image/color"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// stageSong is made-up lyrics, the fourth line synced word by word.
var stageSong = []lyrics.Line{
	{TimeMs: 0, Text: "Stones remember every tide"},
	{TimeMs: 4000, Text: "Sand keeps the shape of it"},
	{TimeMs: 9000, Text: "Quiet as the harbour light"},
	{TimeMs: 14000, Text: "Slow wind over the pier", Words: []lyrics.Word{
		word("Slow", 14000, 14800), word("wind", 14800, 15600), word("over", 15600, 16400), word("the", 16400, 16800), word("pier", 16800, 18000)}},
	{TimeMs: 19000, Text: "Lifts me, sets me down again"},
	{TimeMs: 24000, Text: "Lantern after lantern"},
	{TimeMs: 29000, Text: "Home is where the tide turns"},
	{TimeMs: 34000, Text: "Carry me back to shore"},
}

// stageCoverURL is the address of the cover stageCover draws.
const stageCoverURL = "https://covers.test/harbour.jpg"

// stageCover is a source that gives every address the same cover: a red
// ground with slanting bands, a little as a record sleeve has.
type stageCover struct {
	// white makes the cover all white, the lightest ground white ink can
	// lie on.
	white bool
}

func (cv stageCover) Image(context.Context, string) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, 600, 600))
	for y := range 600 {
		for x := range 600 {
			c := color.RGBA{R: 150, G: 10, B: 14, A: 255}
			if d := (x + y) % 300; d > 170 && d < 240 {
				c = color.RGBA{R: 235, G: 40, B: 40, A: 255}
			}
			if y > 430 {
				c = color.RGBA{R: 20, G: 4, B: 4, A: 255}
			}
			if cv.white {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img, nil
}

// stageOn plays the first test track, whose made-up lyrics are lines, with
// the song at pos, and shows the stage over the window.
func stageOn(t *testing.T, lines []lyrics.Line, pos time.Duration, cover bool) (*shell, *ui.Tester) {
	t.Helper()
	if cover {
		return stageOnWith(t, lines, pos, stageCover{})
	}
	return stageOnWith(t, lines, pos, nil)
}

// stageOnWith is stageOn, whose track's cover is the image src gives, or
// none, for a nil src.
func stageOnWith(t *testing.T, lines []lyrics.Line, pos time.Duration, src artSource) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	track := testTracks[0]
	if src != nil {
		track.Artwork = []media.Image{{URL: stageCoverURL, Width: 600, Height: 600}}
		s.art.src = src
	}
	s.loads["lyrics:"+track.ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "en", Lines: lines}}
	s.p.PlayTracks([]media.Track{track}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
	tt := testerOf(t, s, 1280, 820)
	s.toggleStage()
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	s.p.Time(pos, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == pos })
	tt.Frame()
	return s, tt
}

// TestLyricStylesTable keeps the table of styles as the settings list them,
// and each style's fade and blur growing with the distance of a line from
// the one sung, to whole pixels, so that the lines hold a few levels.
func TestLyricStylesTable(t *testing.T) {
	if len(lyricStyles) != len(settings.LyricStyles) {
		t.Fatalf("%d styles in the table, %d in the settings", len(lyricStyles), len(settings.LyricStyles))
	}
	for i, st := range lyricStyles {
		if st.id != settings.LyricStyles[i] {
			t.Errorf("style %d is %q, the settings' is %q", i, st.id, settings.LyricStyles[i])
		}
		if st.title() == "" || st.about() == "" {
			t.Errorf("%q has no name or no description", st.id)
		}
		if !slices.Contains([]string{face, faceRound, faceMaterial, faceArchivo}, st.family) {
			t.Errorf("%q is set in %q, a face MixMusic doesn't bring", st.id, st.family)
		}
		if st.opacity(0) != 1 || st.blurAt(0) != 0 {
			t.Errorf("%q: the line sung is %v opaque and %v blurred", st.id, st.opacity(0), st.blurAt(0))
		}
		prevOpacity, prevBlur := float32(1), float32(0)
		for dist := 1; dist <= 12; dist++ {
			o, b := st.opacity(dist), st.blurAt(dist)
			if o > prevOpacity || o < st.floor {
				t.Errorf("%q: %d lines away is %v opaque, after %v, with a floor of %v", st.id, dist, o, prevOpacity, st.floor)
			}
			if b < prevBlur || b > st.maxBlur || b != float32(int(b)) {
				t.Errorf("%q: %d lines away is blurred %v, after %v, with a most of %v", st.id, dist, b, prevBlur, st.maxBlur)
			}
			prevOpacity, prevBlur = o, b
		}
		if lyricStyleOf(st.id).id != st.id {
			t.Errorf("lyricStyleOf(%q) finds %q", st.id, lyricStyleOf(st.id).id)
		}
	}
	if got := lyricStyleOf("nonesuch").id; got != settings.LyricStyles[0] {
		t.Errorf("an unknown style is %q, not the first", got)
	}
	if settings.LyricStyles[0] != settings.LyricStyleSpotlight {
		t.Errorf("the first style, a new install's, is %q, not Spotlight", settings.LyricStyles[0])
	}
}

// TestStageShowsTheLyrics shows the track and its lines over the window,
// the line sung now among them, and plays the song from a line that is
// clicked.
func TestStageShowsTheLyrics(t *testing.T) {
	s, tt := stageOn(t, stageSong, 15200*time.Millisecond, false)
	if !s.stage.open {
		t.Fatal("the stage didn't open")
	}
	for _, want := range []string{"Midnight City", "Stones remember every tide", "Slow wind over the pier"} {
		if !tt.HasText(want) {
			t.Errorf("the stage doesn't show %q: %q", want, tt.Texts())
		}
	}
	// The line sung comes to the middle of the window, and the next lies
	// below it, in view.
	tt.Advance(time.Second)
	if err := tt.Click("Lifts me, sets me down again"); err != nil {
		t.Fatalf("clicking a line: %v; the window shows %q", err, tt.Texts())
	}
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Position >= 19*time.Second })
}

// TestStageKeys leaves the arrows to seek, as they do outside the stage: the
// stage has no keys for its style or the size of its text, which are
// Settings'. Space still plays and pauses, and Escape closes it. A new
// install's style is Spotlight.
func TestStageKeys(t *testing.T) {
	s, tt := stageOn(t, stageSong, 15200*time.Millisecond, false)
	if got := s.set.LyricsStyle; got != settings.LyricStyleSpotlight {
		t.Fatalf("the style starts as %q, not Spotlight", got)
	}
	at := s.p.State().Position
	tt.Key(0, ui.KeyRight)
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Position > at })
	tt.Frame()
	forward := s.p.State().Position
	tt.Key(0, ui.KeyLeft)
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Position < forward })
	tt.Frame()
	if !s.stage.open {
		t.Fatal("an arrow closed the stage")
	}
	// The keys that used to change the style and the size do nothing now.
	style, size := s.set.LyricsStyle, s.set.LyricsSize
	for _, key := range []ui.Key{ui.KeyMinus, ui.KeyEqual} {
		tt.Key(0, key)
		s.drain()
		tt.Frame()
	}
	if s.set.LyricsStyle != style || s.set.LyricsSize != size {
		t.Errorf("keys changed the style to %q and the size to %v", s.set.LyricsStyle, s.set.LyricsSize)
	}
	for _, text := range []string{"to switch styles", "text size"} {
		if tt.HasText(text) {
			t.Errorf("the stage still has the hint %q: %q", text, tt.Texts())
		}
	}

	was := s.p.State().Playing
	tt.Key(0, ui.KeySpace)
	s.drain()
	s.settle(t, func(st player.State) bool { return st.Playing != was })
	tt.Frame()
	if !s.stage.open {
		t.Fatal("Space closed the stage")
	}

	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()
	if s.stage.open {
		t.Error("Escape didn't close the stage")
	}
}

// TestStagePaletteOverStage opens the command palette over the stage, whose
// Escape closes the palette first, and the stage only the next time.
func TestStagePaletteOverStage(t *testing.T) {
	s, tt := stageOn(t, stageSong, 15200*time.Millisecond, false)
	tt.Key(ui.Cmd, ui.KeyK)
	s.drain()
	tt.Frame()
	if !s.paletteOpen {
		t.Fatal("Ctrl+K doesn't open the palette over the stage")
	}
	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()
	if s.paletteOpen || !s.stage.open {
		t.Fatalf("Escape leaves the palette %v and the stage %v: the palette goes first", s.paletteOpen, s.stage.open)
	}
	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()
	if s.stage.open {
		t.Error("a second Escape didn't close the stage")
	}
}

// TestStageOpensFromTheLyrics opens the stage from the button of the lyrics
// panel and from its shortcut, and closes it with the shortcut again.
func TestStageOpensFromTheLyrics(t *testing.T) {
	s, tt := sungIn(t, "en", stageSong, 15200*time.Millisecond)
	if err := tt.Click("Full-screen lyrics"); err != nil {
		t.Fatalf("the lyrics panel has no button for the stage: %v; the window shows %q", err, tt.Texts())
	}
	s.drain()
	tt.Frame()
	if !s.stage.open {
		t.Fatal("the button didn't open the stage")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	s.drain()
	tt.Frame()
	if s.stage.open {
		t.Fatal("the shortcut didn't close the stage")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	s.drain()
	tt.Frame()
	if !s.stage.open {
		t.Fatal("the shortcut didn't open the stage")
	}
}

// TestStageInkReadsOnAnyCover checks the stage's ink, and its quieter one,
// at 4.5:1 on the lightest ground a style can have: that of a white cover
// under its scrim, a bright cover being the worst the stage lies over.
func TestStageInkReadsOnAnyCover(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, st := range lyricStyles {
		ground := over(color.NRGBA{A: uint8(255 * st.scrim)}, white)
		for _, ink := range []struct {
			name string
			c    ui.Color
		}{{"ink", stageInk}, {"quieter ink", stageDim}} {
			if got := contrast(over(nrgba(ink.c), ground), ground); got < 4.5 {
				t.Errorf("%s: the stage's %s reads at %.2f:1 on a white cover under a scrim of %v, 4.5:1 at least is needed", st.id, ink.name, got, st.scrim)
			}
		}
	}
}

// TestStageKeyboardOpen opens the stage with its shortcut, as a user without
// a pointer does, and finds Space still plays and pauses and Enter does
// nothing: no button of the stage, the close button included, takes them.
func TestStageKeyboardOpen(t *testing.T) {
	s, tt := sungIn(t, "en", stageSong, 15200*time.Millisecond)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	s.drain()
	tt.Frame()
	if !s.stage.open {
		t.Fatal("the shortcut didn't open the stage")
	}
	for _, key := range []struct {
		name string
		key  ui.Key
	}{{"Space", ui.KeySpace}, {"Enter", ui.KeyEnter}} {
		was := s.p.State().Playing
		tt.Key(0, key.key)
		s.drain()
		tt.Frame()
		if !s.stage.open {
			t.Fatalf("%s closed the stage", key.name)
		}
		if key.key == ui.KeySpace {
			s.settle(t, func(st player.State) bool { return st.Playing != was })
		}
	}
}

// TestLyricStyleSettings lists the styles in Settings › Appearance, in
// the words of the layout, and keeps the one chosen.
func TestLyricStyleSettings(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	// The tests wear the Tiles layout, whose words are lowercase.
	for _, st := range lyricStyles {
		if !tt.HasText(s.lk.words(st.title())) {
			t.Errorf("Settings doesn't list the style %q: %q", st.title(), tt.Texts())
		}
	}
	click(t, s, tt, s.lk.words("Glow"))
	if got := s.set.LyricsStyle; got != settings.LyricStyleGlow {
		t.Errorf("choosing Glow leaves the style %q", got)
	}
	if got := newShell(s.a, s.p).set.LyricsStyle; got != settings.LyricStyleGlow {
		t.Errorf("the style kept is %q", got)
	}
}

// TestLyricBallSwitch sets the bouncing ball from Settings: on in a new
// install, and kept as the user leaves it.
func TestLyricBallSwitch(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	if !s.set.LyricsBall {
		t.Fatal("the ball is off in a new install")
	}
	show(s, tt, "/settings/appearance")
	click(t, s, tt, "Show the bouncing ball")
	if s.set.LyricsBall {
		t.Error("the switch leaves the ball on")
	}
	if newShell(s.a, s.p).set.LyricsBall {
		t.Error("the ball kept is on")
	}
}

// TestLyricPreview builds the preview in Settings › Appearance with nothing
// playing, as it is the first time the page shows: the made-up line sung and
// the lines around it, in the style set, moving on as time passes.
func TestLyricPreview(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	if _, ok := tt.Find("Preview of the full-screen lyrics"); !ok {
		t.Fatalf("Settings has no preview: %q", tt.Texts())
	}
	shown := func() (n int) {
		for _, line := range previewLines {
			if tt.HasText(line) {
				n++
			}
		}
		return n
	}
	if got := shown(); got != 3 {
		t.Errorf("the preview shows %d of its lines, not the one sung and the two around it: %q", got, tt.Texts())
	}
	// It asks to be built again as the song goes on, and goes on to the line
	// after.
	first := tt.Texts()
	tt.Advance(previewLineFor + previewTick)
	tt.Frame()
	if got := shown(); got != 3 {
		t.Errorf("a line on, the preview shows %d of its lines", got)
	}
	if slices.Equal(first, tt.Texts()) {
		t.Error("the preview didn't go on to the next line")
	}
	// A style and a size of its own show in it without a reload.
	s.set.LyricsStyle, s.set.LyricsSize = settings.LyricStyleBold, 1.5
	s.drain()
	tt.Frame()
	if got := shown(); got != 3 {
		t.Errorf("in another style the preview shows %d of its lines", got)
	}
}

// brightIn counts the pixels of r in img that are nearly white.
func brightIn(img *image.RGBA, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if p := img.RGBAAt(x, y); min(p.R, p.G, p.B) > 170 {
				n++
			}
		}
	}
	return n
}

// TestStageBall puts a ball on the letters of the word being sung, its bottom
// touching their tops, and none with the ball off.
func TestStageBall(t *testing.T) {
	// The line sung follows the song's clock as the window's goes on from the
	// player's time: a second after 14 s, the ball rests on "wind", whose
	// hop to "over" starts at 15.14 s.
	s, tt := stageOn(t, stageSong, 14000*time.Millisecond, false)
	tt.Advance(time.Second)
	if !s.set.LyricsBall {
		t.Fatal("the ball is off in a new install")
	}
	r, ok := tt.Find("Slow wind over the pier")
	if !ok {
		t.Fatalf("the line sung doesn't show: %q", tt.Texts())
	}
	on := tt.Image()
	rig := &s.stage.ball
	if !rig.has || len(rig.cur.spots) != 5 {
		t.Fatalf("the line sung gave the ball's rig %+v", rig.cur)
	}
	spot, size, radius := rig.cur.spots[1], rig.cur.size, rig.cur.radius
	// The ball lies on the spot, in the box just over the tops of the letters:
	// bright, as the lit letters are.
	ball := ui.Rect{X: spot.x - radius - 1, Y: spot.y - 2*radius - 1, W: 2*radius + 2, H: 2*radius + 1}
	withBall := brightIn(on, ball)

	if withBall < int(0.3*ball.W*ball.H) {
		t.Errorf("the pixels of a ball on %+v: %d of %v", spot, withBall, ball.W*ball.H)
	}

	// The ball's bottom is where the tops of the lit capitals and tall letters
	// beside it are: it touches them. They are looked for left of the ball, in
	// the word before it, where it covers nothing.
	left := ui.Rect{X: r.X, Y: spot.y - size/2, W: spot.x - 3*radius - r.X, H: size}
	top := -1
	for y := int(left.Y); y < int(left.Y+left.H) && top < 0; y++ {
		if brightIn(on, ui.Rect{X: left.X, Y: float32(y), W: left.W, H: 1}) > 0 {
			top = y
		}
	}
	if top < 0 || stageAbs(float32(top)-spot.y) > 0.07*size {
		t.Errorf("the ball touches the letters at y %v, where the tops of the letters beside it are at %d, in a font %v high", spot.y, top, size)
	}

	// The ball of a line that no longer gives it goes with it: the line's plan
	// isn't given again.
	was := rig.stamp
	s.set.LyricsBall = false
	s.drain()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !rig.stamp.Equal(was) {
		t.Errorf("with the ball off the line gave the ball a plan at %v, after %v", rig.stamp, was)
	}
}

// TestStageBallRoomOnlyWithWordTimes keeps the room above each line for the
// ball only where the lyrics have words with times: most lyrics are timed
// line by line, and never show a ball, so with the ball on, as it is at
// first, they keep their spacing.
func TestStageBallRoomOnlyWithWordTimes(t *testing.T) {
	plain := make([]lyrics.Line, len(stageSong))
	for i, l := range stageSong {
		plain[i] = lyrics.Line{TimeMs: l.TimeMs, Text: l.Text}
	}
	// pitch is how far apart two lines lie, with the ball on or off.
	pitch := func(lines []lyrics.Line, ball bool) float32 {
		t.Helper()
		s, tt := stageOn(t, lines, 15200*time.Millisecond, false)
		s.set.LyricsBall = ball
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
		return stageMid(t, tt, "Lifts me, sets me down again") - stageMid(t, tt, "Slow wind over the pier")
	}
	if on, off := pitch(plain, true), pitch(plain, false); stageAbs(on-off) > 1 {
		t.Errorf("lyrics timed by line keep room for a ball they don't have: the lines lie %v apart with it on, %v with it off", on, off)
	}
	if on, off := pitch(stageSong, true), pitch(stageSong, false); on <= off+1 {
		t.Errorf("lyrics with words timed keep no room for the ball: the lines lie %v apart with it on, %v with it off", on, off)
	}
}

// stageMid is where the middle of the text shown lies, down the window.
func stageMid(t *testing.T, tt *ui.Tester, text string) float32 {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q doesn't show: %q", text, tt.Texts())
	}
	return r.Y + r.H/2
}

// stageAbs is the size of v.
func stageAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestStageGlidesToTheNextLine keeps the line sung in the middle of the
// window, and, as the song goes on to the next, glides the lyrics up to it
// rather than jumping: it still lies below the middle in the frame the song
// goes on, and rests at it once the glide ends.
func TestStageGlidesToTheNextLine(t *testing.T) {
	s, tt := stageOn(t, stageSong, 15200*time.Millisecond, false)
	tt.Advance(time.Second)
	middle := float32(820) / 2
	if d := stageMid(t, tt, "Slow wind over the pier") - middle; stageAbs(d) > 30 {
		t.Fatalf("the line sung lies %v from the middle of the window", d)
	}
	s.p.Time(19500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 19500*time.Millisecond })
	tt.Frame()
	if d := stageMid(t, tt, "Lifts me, sets me down again") - middle; d < 25 {
		t.Errorf("the next line jumps to the middle at once: it lies %v below it", d)
	}
	tt.Advance(time.Second)
	if d := stageMid(t, tt, "Lifts me, sets me down again") - middle; stageAbs(d) > 30 {
		t.Errorf("the next line doesn't come to the middle: it lies %v from it", d)
	}
}

// TestStageCentersWhenTheLookChanges centers the line sung again, at once,
// when a style or a size changes the height of the lines.
func TestStageCentersWhenTheLookChanges(t *testing.T) {
	s, tt := stageOn(t, stageSong, 24500*time.Millisecond, false)
	tt.Advance(time.Second)
	middle := float32(820) / 2
	for _, step := range []struct {
		name string
		do   func()
	}{
		{"a larger style", func() { s.set.LyricsStyle = settings.LyricStyleBold }},
		{"a larger size", func() { s.set.LyricsSize = 1.6 }},
		{"a smaller size", func() { s.set.LyricsSize = 0.7 }},
		{"the ball off", func() { s.set.LyricsBall = false }},
	} {
		step.do()
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
		if d := stageMid(t, tt, "Lantern after lantern") - middle; stageAbs(d) > 40 {
			t.Errorf("after %s, the line sung lies %v from the middle of the window", step.name, d)
		}
	}
}

// TestStageFollowsTheWindow centers the line sung again when the window
// changes size, which wraps the lines anew.
func TestStageFollowsTheWindow(t *testing.T) {
	s, _ := stageOn(t, stageSong, 15200*time.Millisecond, false)
	narrow := testerOf(t, s, 960, 640)
	narrow.Advance(time.Second)
	if d := stageMid(t, narrow, "Slow wind over the pier") - float32(640)/2; stageAbs(d) > 50 {
		t.Errorf("in a narrow window the line sung lies %v from the middle", d)
	}
}

// TestStageWithoutATrack shows nothing, and says why.
func TestStageWithoutATrack(t *testing.T) {
	s := newTestShell(t)
	s.toggleStage()
	if s.stage.open {
		t.Error("the stage opened with nothing playing")
	}
	if len(s.notes) == 0 {
		t.Error("nothing says why the stage doesn't open")
	}
}

// TestStageRender draws each style of the stage to MIXMUSIC_RENDER, for a
// look: blur and glow are for the eye.
func TestStageRender(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s, tt := stageOn(t, stageSong, 15200*time.Millisecond, true)
	coversLoad(t, s, tt)
	for _, id := range settings.LyricStyles {
		s.set.LyricsStyle = id
		renderTo(t, s, tt, dir, "stage-"+string(id))
	}
	// A narrow window, where the sides give way to the lyrics.
	s.set.LyricsStyle = settings.LyricStyleSpotlight
	renderTo(t, s, testerOf(t, s, 960, 640), dir, "stage-narrow")
	// Without the ball.
	s.set.LyricsBall = false
	renderTo(t, s, tt, dir, "stage-spotlight-noball")
	s.set.LyricsBall = true
	// The covers of the queue, with covers in them.
	qs, qtt := stageQueueWith(t, stageCover{})
	coversLoad(t, qs, qtt)
	renderTo(t, qs, qtt, dir, "stage-queue")
	// The preview in Settings.
	s.stage.open = false
	for _, id := range []settings.LyricStyle{settings.LyricStyleSpotlight, settings.LyricStyleGlow, settings.LyricStyleCalm} {
		s.set.LyricsStyle = id
		show(s, tt, "/settings/appearance")
		reveal(tt, "Preview of the full-screen lyrics")
		renderTo(t, s, tt, dir, "preview-"+string(id))
	}
	// A white cover is the worst ground for white ink.
	s, tt = stageOnWith(t, stageSong, 15200*time.Millisecond, stageCover{white: true})
	coversLoad(t, s, tt)
	for _, id := range []settings.LyricStyle{settings.LyricStyleGlow, settings.LyricStyleCalm} {
		s.set.LyricsStyle = id
		renderTo(t, s, tt, dir, "stage-white-"+string(id))
	}
}

// coversLoad runs the window until the covers it asked for are loaded.
func coversLoad(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		s.drain()
		tt.Frame()
		ready := len(s.art.m) > 0
		for _, e := range s.art.m {
			ready = ready && e.bmp != nil
		}
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the cover didn't load")
		}
	}
}
