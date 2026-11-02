package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The made-up housings of the tests: 180 × 32 points, as a MacBook's, and
// a wider one.
var (
	testHousing = housing{X: 662, W: 180, H: 32}
	wideHousing = housing{X: 500, Y: 25, W: 360, H: 38}
)

// fakeNotchHost keeps what the window around the notch asked of the
// keyboard: true to take it, false to give it back.
type fakeNotchHost struct {
	asked  []bool
	closed bool
}

func (f *fakeNotchHost) keyboard(take bool) { f.asked = append(f.asked, take) }
func (f *fakeNotchHost) close()             { f.closed = true }

// notchTest is the player around the notch of a test shell, in a Tester
// that takes the window's size as the notch places it.
type notchTest struct {
	t      *testing.T
	s      *shell
	n      *notch
	tt     *ui.Tester
	host   *fakeNotchHost
	placed []mygo.Rectangle
}

func newNotchTest(t *testing.T, s *shell, h housing) *notchTest {
	t.Helper()
	// The panel's lyrics are the test's, never the network's.
	for _, tr := range testTracks {
		if _, ok := s.loads["lyrics:"+tr.ID]; !ok {
			s.loads["lyrics:"+tr.ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Instrumental: true}}
		}
	}
	x := &notchTest{t: t, s: s, host: &fakeNotchHost{}}
	x.n = &notch{s: s, housing: h, host: x.host}
	x.n.place = func(r mygo.Rectangle) { x.placed = append(x.placed, r) }
	r := h.compact()
	x.tt = ui.NewTester(x.n.view, r.Width, r.Height)
	x.frame()
	return x
}

// playing is a notchTest of a shell whose player plays testTracks.
func playingNotch(t *testing.T, h housing) *notchTest {
	t.Helper()
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	return newNotchTest(t, s, h)
}

// follow sizes the Tester as the window was placed last, after the frame
// that placed it.
func (x *notchTest) follow() {
	if len(x.placed) == 0 {
		return
	}
	r := x.placed[len(x.placed)-1]
	x.placed = nil
	x.tt.SetSize(r.Width, r.Height)
	x.tt.Frame()
}

func (x *notchTest) frame() {
	x.s.drain()
	x.tt.Frame()
	x.follow()
}

func (x *notchTest) advance(d time.Duration) {
	x.s.drain()
	x.tt.Advance(d)
	x.follow()
}

func (x *notchTest) click(label string) {
	x.t.Helper()
	if err := x.tt.Click(label); err != nil {
		x.t.Fatalf("clicking %q: %v; the window shows %q", label, err, x.tt.Texts())
	}
	x.frame()
}

// size is the Tester's size, as the window's.
func (x *notchTest) size() (int, int) {
	b := x.tt.Image().Bounds()
	return b.Dx(), b.Dy()
}

// openPanel opens the panel, and lets it grow.
func (x *notchTest) openPanel() {
	x.t.Helper()
	x.click("Open the player")
	x.advance(time.Second)
	if !x.n.open {
		x.t.Fatal("the panel didn't open")
	}
}

// closed fails unless the panel closed and shrank back into the compact
// window, having given the keyboard back.
func (x *notchTest) closed(how string) {
	x.t.Helper()
	x.advance(time.Second)
	c := x.n.housing.compact()
	if w, h := x.size(); x.n.open || x.n.big || w != c.Width || h != c.Height {
		x.t.Errorf("%s: open %v, at %d × %d", how, x.n.open, w, h)
	}
	if a := x.host.asked; len(a) == 0 || a[len(a)-1] {
		x.t.Errorf("%s: the keyboard wasn't given back: %v", how, a)
	}
}

// alphaAt is the opacity of the window's pixel at (x, y), in points.
func alphaAt(img *image.RGBA, x, y int) uint8 { return img.RGBAAt(x, y).A }

// blackUnderCamera fails unless the window draws nothing but black under
// the camera.
func blackUnderCamera(t *testing.T, what string, img *image.RGBA, h housing) {
	t.Helper()
	w := img.Bounds().Dx()
	left := (w - h.W) / 2
	for y := range h.H {
		for x := left; x < left+h.W; x++ {
			if c := img.RGBAAt(x, y); c.R != 0 || c.G != 0 || c.B != 0 || c.A != 255 {
				t.Fatalf("%s: under the camera, at (%d, %d), it draws %v", what, x, y, c)
			}
		}
	}
}

// TestNotchShapes draws nothing with nothing to play; while a track is
// loaded, the housing widened in black, nothing but black under the camera
// and transparent outside the rounded corners; open, the panel grown from
// it; around a wider housing, a wider panel.
func TestNotchShapes(t *testing.T) {
	x := newNotchTest(t, newTestShell(t), testHousing)
	img := x.tt.Image()
	for _, p := range []image.Point{{0, 0}, {126, 16}, {251, 31}} {
		if a := alphaAt(img, p.X, p.Y); a != 0 {
			t.Errorf("with nothing to play, it draws at %v", p)
		}
	}
	for _, h := range []housing{testHousing, wideHousing} {
		x := playingNotch(t, h)
		c := h.compact()
		if w, ht := x.size(); w != c.Width || ht != c.Height {
			t.Fatalf("%+v: compact at %d × %d", h, w, ht)
		}
		img := x.tt.Image()
		blackUnderCamera(t, "compact", img, h)
		if a := alphaAt(img, 0, c.Height-1); a != 0 {
			t.Errorf("%+v: compact, the bottom left corner isn't transparent", h)
		}
		if a := alphaAt(img, c.Width-1, c.Height-1); a != 0 {
			t.Errorf("%+v: compact, the bottom right corner isn't transparent", h)
		}
		if a := alphaAt(img, 0, 0); a != 255 {
			t.Errorf("%+v: compact, the top left corner isn't black, against the screen's top", h)
		}

		x.openPanel()
		o := h.open()
		if w, ht := x.size(); w != o.Width || ht != o.Height {
			t.Fatalf("%+v: open at %d × %d, not %+v", h, w, ht, o)
		}
		img = x.tt.Image()
		blackUnderCamera(t, "open", img, h)
		if a := alphaAt(img, 0, o.Height-1); a != 0 {
			t.Errorf("%+v: open, the bottom left corner isn't transparent", h)
		}
		if a := alphaAt(img, o.Width/2, o.Height-2); a != 255 {
			t.Errorf("%+v: open, the panel doesn't reach its foot", h)
		}
		for _, want := range []string{testTracks[0].Title, "M83", "Open MixMusic", "Close the player"} {
			if _, ok := x.tt.Find(want); !ok {
				t.Errorf("%+v: the open panel doesn't show %q: %q", h, want, x.tt.Texts())
			}
		}
	}
}

// TestNotchControls drives the player from the panel, which stays open,
// and has its lyrics follow a seek: the line sung, with those before and
// after it.
func TestNotchControls(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Paper boats on the river"}, {TimeMs: 30000, Text: "Carry what the morning said"},
			{TimeMs: 60000, Text: "Lanterns lean across the water"}, {TimeMs: 90000, Text: "Folding light into the bend"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	if !x.tt.HasText("Paper boats on the river") || !x.tt.HasText("Carry what the morning said") || x.tt.HasText("Lanterns lean across the water") {
		t.Errorf("at the start, the lyrics show %q", x.tt.Texts())
	}
	s.p.Seek(61 * time.Second)
	s.settle(t, func(st player.State) bool { return st.Position >= 61*time.Second })
	x.frame()
	for _, want := range []string{"Carry what the morning said", "Lanterns lean across the water", "Folding light into the bend", "1:01", "3:31"} {
		if !x.tt.HasText(want) {
			t.Errorf("after a seek, the panel doesn't show %q: %q", want, x.tt.Texts())
		}
	}
	if x.tt.HasText("Paper boats on the river") {
		t.Errorf("after a seek, the lyrics show a line two before: %q", x.tt.Texts())
	}

	x.click("Pause")
	s.settle(t, func(st player.State) bool { return !st.Playing })
	x.frame()
	x.click("Play")
	s.settle(t, func(st player.State) bool { return st.Playing })
	x.frame()
	x.click("Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	x.frame()
	if !x.tt.HasText(testTracks[1].Title) {
		t.Errorf("the panel doesn't show the next track: %q", x.tt.Texts())
	}
	x.click("Previous")
	s.settle(t, func(st player.State) bool { return st.Index == 0 })
	x.frame()
	if !x.n.open {
		t.Error("the transport closed the panel")
	}
	if len(x.host.asked) != 1 || !x.host.asked[0] {
		t.Errorf("opening asked the keyboard %v", x.host.asked)
	}
}

// TestRenderNotch draws the player around the notch to MIXMUSIC_RENDER,
// for a look: compact, opening, open, and around a wider housing.
func TestRenderNotch(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	write := func(x *notchTest, name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, x.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for name, h := range map[string]housing{"notch": testHousing, "notch-wide": wideHousing} {
		s := newTestShell(t)
		s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
			Lines: []lyrics.Line{{TimeMs: 0, Text: "Paper boats on the river"}, {TimeMs: 30000, Text: "Carry what the morning said"},
				{TimeMs: 60000, Text: "Lanterns lean across the water"}}}}
		s.p.PlayTracks(testTracks, 0)
		s.settle(t, func(st player.State) bool { return st.Playing })
		x := newNotchTest(t, s, h)
		write(x, name+"-compact")
		x.click("Open the player")
		x.advance(120 * time.Millisecond)
		write(x, name+"-opening")
		x.advance(time.Second)
		write(x, name+"-open")
	}
}

// TestNotchCloses closes the panel by a click on either side, by Escape,
// and once the pointer stayed away two seconds, which coming back in, a
// drag of the progress bar or the keyboard's focus on a control calls
// off; and as it opens MixMusic. Each gives the keyboard back.
func TestNotchCloses(t *testing.T) {
	x := playingNotch(t, testHousing)
	x.openPanel()
	x.click("Close the player")
	x.closed("a click on the left")

	x.openPanel()
	w, _ := x.size()
	x.tt.ClickAt(float32(w+testHousing.W)/2+float32(testHousing.wing())/2, float32(testHousing.H)/2)
	x.frame()
	x.closed("a click on the right")

	x.openPanel()
	x.tt.Key(0, ui.KeyEscape)
	x.frame()
	x.closed("Escape")

	x.openPanel()
	x.click("Open MixMusic")
	x.closed("Open MixMusic")

	x.openPanel()
	x.tt.Move(-20, -20)
	x.advance(1500 * time.Millisecond)
	x.tt.Move(240, 120)
	x.advance(1500 * time.Millisecond)
	if !x.n.open {
		t.Fatal("the pointer came back in, but the countdown went on")
	}
	x.tt.Move(-20, -20)
	x.advance(1900 * time.Millisecond)
	if !x.n.open {
		t.Fatal("the panel closed before two seconds")
	}
	x.advance(200 * time.Millisecond)
	x.closed("the pointer away")

	// A drag of the progress bar holds it open, wherever the pointer goes.
	x.openPanel()
	r, ok := x.tt.Find("Seek")
	if !ok {
		t.Fatalf("no progress bar: %q", x.tt.Texts())
	}
	x.tt.Press(r.X+r.W/2, r.Y+r.H/2)
	x.frame()
	x.tt.Move(r.X+r.W/2+10, r.Y+r.H/2)
	x.frame()
	x.tt.Move(-20, 300)
	x.advance(3 * time.Second)
	if !x.n.open {
		t.Fatal("the panel closed during a drag")
	}
	x.tt.Release(-20, 300)
	x.advance(2100 * time.Millisecond)
	x.closed("the pointer away after a drag")

	// So does the keyboard's focus on a control, until the keyboard goes.
	x.openPanel()
	x.tt.Key(0, ui.KeyTab)
	x.frame()
	x.tt.Move(-20, -20)
	x.advance(3 * time.Second)
	if !x.n.open {
		t.Fatal("the panel closed with the keyboard's focus on a control")
	}
	x.tt.SetFocused(false)
	x.advance(2100 * time.Millisecond)
	x.closed("the pointer away after the keyboard went")
}

// TestNotchKeyboard takes the keyboard as the panel opens and gives it
// back as it closes, and never while compact. The controls of a panel
// that closes, fading out, take no clicks and no keys.
func TestNotchKeyboard(t *testing.T) {
	x := playingNotch(t, testHousing)
	x.advance(time.Second)
	if len(x.host.asked) != 0 {
		t.Fatalf("compact, it asked the keyboard %v", x.host.asked)
	}
	x.openPanel()
	pause, ok := x.tt.Find("Pause")
	if !ok {
		t.Fatalf("no pause button: %q", x.tt.Texts())
	}
	x.click("Close the player")
	x.tt.ClickAt(pause.X+pause.W/2, pause.Y+pause.H/2)
	x.frame()
	if st := x.s.p.State(); !st.Playing {
		t.Error("the pause button of a panel that closes paused")
	}
	x.advance(time.Second)
	if a := x.host.asked; len(a) != 2 || !a[0] || a[1] {
		t.Errorf("opening and closing asked the keyboard %v", a)
	}
}

// TestNotchReduceMotion fades the panel in and out at its size, without
// growing, while the system asks for less motion; else it grows.
func TestNotchReduceMotion(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		x := playingNotch(t, testHousing)
		x.tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		x.frame()
		x.click("Open the player")
		x.advance(80 * time.Millisecond)
		img := x.tt.Image()
		foot := alphaAt(img, 240, 175)
		switch {
		case reduce && (foot == 0 || foot == 255):
			t.Errorf("under Reduce Motion, the panel's foot isn't fading in: %d", foot)
		case !reduce && foot != 0:
			t.Errorf("growing, the panel reached its foot at once: %d", foot)
		case !reduce && alphaAt(img, 240, 40) != 255:
			t.Errorf("growing, the panel's black isn't opaque: %d", alphaAt(img, 240, 40))
		}
		x.advance(time.Second)
		if a := alphaAt(x.tt.Image(), 240, 175); a != 255 {
			t.Errorf("reduce %v: the open panel's foot is %d", reduce, a)
		}
		x.click("Close the player")
		x.advance(50 * time.Millisecond)
		if w, _ := x.size(); w != testHousing.open().Width {
			t.Errorf("reduce %v: the window shrank before the panel", reduce)
		}
		if a := alphaAt(x.tt.Image(), 240, 175); reduce && (a == 0 || a == 255) {
			t.Errorf("under Reduce Motion, the panel's foot isn't fading out: %d", a)
		}
		x.advance(70 * time.Millisecond)
		if a := alphaAt(x.tt.Image(), 240, 175); !reduce && a != 0 {
			t.Errorf("shrinking, the panel still reaches its foot: %d", a)
		}
		x.closed("reduce " + strconv.FormatBool(reduce))
	}
}

// TestNotchReopens opens the panel again as it shrinks: the end of the
// shrinking doesn't close it.
func TestNotchReopens(t *testing.T) {
	x := playingNotch(t, testHousing)
	x.openPanel()
	x.click("Close the player")
	x.advance(100 * time.Millisecond)
	x.click("Open the player")
	x.advance(time.Second)
	o := testHousing.open()
	if w, h := x.size(); !x.n.open || w != o.Width || h != o.Height {
		t.Errorf("opened as it shrank, the panel is open %v at %d × %d", x.n.open, w, h)
	}
}

// TestNotchLyricsWhileLoading shows the lyrics of the track playing: those
// of the track before, which come as it changed, don't show.
func TestNotchLyricsWhileLoading(t *testing.T) {
	s := newTestShell(t)
	first := &loaded[lyrics.Lyrics]{}
	s.loads["lyrics:"+testTracks[0].ID] = first
	s.loads["lyrics:"+testTracks[1].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Stones remember every tide"}, {TimeMs: 600000, Text: "Love is a doing word"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	if !x.tt.HasText("Looking for the lyrics…") {
		t.Errorf("the lyrics that load show %q", x.tt.Texts())
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing })
	x.frame()
	first.value, first.done = lyrics.Lyrics{Synced: true, Lines: []lyrics.Line{{TimeMs: 0, Text: "Paper boats on the river"}}}, true
	x.frame()
	if !x.tt.HasText("Stones remember every tide") || x.tt.HasText("Paper boats on the river") {
		t.Errorf("the lyrics of the track before came: %q", x.tt.Texts())
	}

	for _, c := range []struct {
		l    *loaded[lyrics.Lyrics]
		want string
		try  bool
	}{
		{&loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Lines: []lyrics.Line{{Text: "Stones remember every tide"}}}}, "No timed lyrics.", false},
		{&loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Instrumental: true}}, "This track is instrumental.", false},
		{&loaded[lyrics.Lyrics]{done: true, err: fmt.Errorf("No lyrics found for this track: %w", lyrics.ErrNotFound)}, "No lyrics found for this track", false},
		{&loaded[lyrics.Lyrics]{done: true, err: fmt.Errorf("Lyrics can't be loaded right now: %w", lyrics.ErrUnavailable)}, "Lyrics can't be loaded right now", true},
	} {
		s.loads["lyrics:"+testTracks[1].ID] = c.l
		x.frame()
		if !x.tt.HasText(c.want) || x.tt.HasText("try again") != c.try {
			t.Errorf("for %q, the panel shows %q", c.want, x.tt.Texts())
		}
	}
	failed := s.loads["lyrics:"+testTracks[1].ID]
	x.click("try again")
	if s.loads["lyrics:"+testTracks[1].ID] == failed {
		t.Error("try again didn't ask for the lyrics again")
	}
}
