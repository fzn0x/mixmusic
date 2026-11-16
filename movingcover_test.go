package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// gifColors are the colours of the frames of solidGIF, one each.
var gifColors = []color.RGBA{{R: 230, A: 255}, {G: 230, A: 255}, {B: 230, A: 255}, {R: 230, G: 230, A: 255}}

// solidGIF is a GIF w by h whose frames are the colours given, one each,
// each shown for delay hundredths of a second.
func solidGIF(t testing.TB, w, h, delay int, colors ...color.RGBA) []byte {
	t.Helper()
	pal := color.Palette{}
	for _, c := range colors {
		pal = append(pal, c)
	}
	g := &gif.GIF{Config: image.Config{Width: w, Height: h, ColorModel: pal}}
	for i := range colors {
		f := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for p := range f.Pix {
			f.Pix[p] = uint8(i)
		}
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, delay)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// localGIF keeps data as a cover of the music on this computer, and
// returns a track of it.
func localGIF(t *testing.T, s *shell, name string, data []byte) media.Track {
	t.Helper()
	url := "localart:" + name
	path, ok := s.svc.LocalFiles.ArtworkFile(url)
	if !ok {
		t.Fatal("no folder of local covers")
	}
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	track := testTrack("yt:gifCover01", "Kites", "Ada Field", "Paper", media.SourceYouTube)
	track.Artwork = []media.Image{{URL: url}}
	return track
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// movingShell is a test shell playing track on the now playing page, with
// moving covers on as on says.
func movingShell(t *testing.T, s *shell, track media.Track, on bool) *ui.Tester {
	t.Helper()
	set := s.svc.Preferences.Get()
	set.MovingCovers = on
	if _, err := s.svc.Preferences.Set(set); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.panel = ""
	s.loads["related-"+track.ID] = &loaded[[]media.Shelf]{done: true}
	s.p.PlayTracks([]media.Track{track}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Source != nil })
	tt := testerOf(t, s, 1600, 1000)
	show(s, tt, "/now-playing")
	return tt
}

// coverColor is the colour the window shows in the upper middle of what
// is labeled label, above the visualizer.
func coverColor(t *testing.T, tt *ui.Tester, label string) color.RGBA {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("no %s: %q", label, tt.Texts())
	}
	return tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/4))
}

// nearRGBA reports whether two colours are about the same.
func nearRGBA(a, b color.RGBA) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) < 24 && d(a.G, b.G) < 24 && d(a.B, b.B) < 24
}

// TestMovingCoverSleeve plays a GIF cover on the now playing page's
// sleeve: each frame shows as it comes due, from the painter's time
// without building the page, and the first again after the last. It
// holds its frame while the music pauses and while the window is hidden,
// and goes on from there.
func TestMovingCoverSleeve(t *testing.T) {
	s := newTestShell(t)
	track := localGIF(t, s, "moving.gif", solidGIF(t, 64, 64, 10, gifColors...))
	tt := movingShell(t, s, track, true)
	label := "Moving cover: " + track.Title
	until(t, s, tt, "the GIF", func() bool { _, ok := tt.Find(label); return ok })
	first := coverColor(t, tt, label)
	if !nearRGBA(first, gifColors[0]) {
		t.Fatalf("the sleeve shows %v, not the first frame", first)
	}
	for i := 1; i <= len(gifColors); i++ {
		tt.Advance(100 * time.Millisecond)
		want := gifColors[i%len(gifColors)]
		if got := coverColor(t, tt, label); !nearRGBA(got, want) {
			t.Errorf("after %d frames the sleeve shows %v, not %v", i, got, want)
		}
	}

	// Paused, it holds its frame.
	tt.Advance(100 * time.Millisecond)
	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	held := coverColor(t, tt, label)
	tt.Advance(time.Second)
	if got := coverColor(t, tt, label); got != held {
		t.Errorf("paused, the sleeve went from %v to %v", held, got)
	}
	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if got := coverColor(t, tt, label); nearRGBA(got, held) {
		t.Errorf("playing again, the sleeve still shows %v", got)
	}

	// Hidden, it paints nothing, and shows the next frame once it shows.
	g := s.moving.m[track.Artwork[0].URL]
	tt.SetWindowHidden(true)
	// The window draws the frame its hiding asks for, then none.
	tt.Advance(100 * time.Millisecond)
	before, drawn := coverColor(t, tt, label), g.drawn
	tt.Advance(time.Second)
	if !g.drawn.Equal(drawn) {
		t.Error("the hidden window paints the GIF")
	}
	tt.SetWindowHidden(false)
	tt.Frame()
	i := colorIndex(before)
	if got := coverColor(t, tt, label); i < 0 || !nearRGBA(got, gifColors[(i+1)%len(gifColors)]) {
		t.Errorf("shown again after %v, the sleeve shows %v", before, got)
	}
}

// colorIndex is the frame of gifColors c is, or -1.
func colorIndex(c color.RGBA) int {
	for i, g := range gifColors {
		if nearRGBA(c, g) {
			return i
		}
	}
	return -1
}

// TestMovingCoverStill keeps the still of a GIF cover with the setting
// off, under reduced motion, and for a GIF that takes more than
// movingCoverMax decoded, which isn't decoded.
func TestMovingCoverStill(t *testing.T) {
	s := newTestShell(t)
	track := localGIF(t, s, "moving.gif", solidGIF(t, 64, 64, 10, gifColors...))
	tt := movingShell(t, s, track, false)
	label := "Moving cover: " + track.Title
	tt.Advance(time.Second)
	if _, ok := tt.Find(label); ok {
		t.Error("the GIF moves with the setting off")
	}
	if len(s.moving.m) != 0 {
		t.Error("the GIF decodes with the setting off")
	}

	s.set.MovingCovers = true
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Advance(time.Second)
	if _, ok := tt.Find(label); ok {
		t.Error("the GIF moves under reduced motion")
	}
	tt.SetPreferences(ui.Preferences{})
	until(t, s, tt, "the GIF", func() bool { _, ok := tt.Find(label); return ok })

	// A GIF of a picture of 2,600 pixels square takes 27 MB decoded.
	big := localGIF(t, s, "big.gif", solidGIF(t, 2600, 2600, 10, gifColors[:2]...))
	big.ID = "yt:gifCover02"
	s.loads["related-"+big.ID] = &loaded[[]media.Shelf]{done: true}
	s.p.PlayTracks([]media.Track{big}, 0)
	s.settle(t, func(st player.State) bool { return st.Source != nil && st.Source.Track.ID == big.ID })
	until(t, s, tt, "the big GIF to be looked at", func() bool {
		g := s.moving.m["localart:big.gif"]
		return g != nil && !g.loading
	})
	if g := s.moving.m["localart:big.gif"]; !g.still || g.dec != nil {
		t.Errorf("a GIF of 27 MB decoded moves: %+v", g)
	}
	if _, ok := tt.Find(label); ok {
		t.Error("the big GIF moves")
	}
}

// TestMovingCoverLetGo lets go of a GIF no page showed for
// movingCoverKeep, and holds movingCoverHeld at most.
func TestMovingCoverLetGo(t *testing.T) {
	s := newTestShell(t)
	now := time.Now()
	s.moving.now = func() time.Time { return now }
	track := localGIF(t, s, "moving.gif", solidGIF(t, 64, 64, 10, gifColors...))
	tt := movingShell(t, s, track, true)
	until(t, s, tt, "the GIF", func() bool { _, ok := tt.Find("Moving cover: " + track.Title); return ok })
	show(s, tt, "/")
	now = now.Add(movingCoverKeep / 2)
	tt.Frame()
	if s.moving.m[track.Artwork[0].URL] == nil {
		t.Fatal("the GIF went at once")
	}
	now = now.Add(movingCoverKeep)
	tt.Frame()
	if len(s.moving.m) != 0 {
		t.Errorf("%d GIFs stay", len(s.moving.m))
	}

	for i := range movingCoverHeld + 2 {
		s.moving.m["held"+count(i)] = &gifCover{dec: &gifDecoderStub{}, built: now.Add(time.Duration(i))}
	}
	s.sweepMovingCovers()
	if len(s.moving.m) != movingCoverHeld || s.moving.m["held0"] != nil || s.moving.m["held1"] != nil {
		t.Errorf("held %d GIFs", len(s.moving.m))
	}
}

// gifDecoderStub decodes nothing.
type gifDecoderStub struct{ ui.VideoDecoder }

// TestMovingCoverMix plays the GIF of a mix's own cover in the header of
// its page, and the still of its first frame on its tile.
func TestMovingCoverMix(t *testing.T) {
	s := newTestShell(t)
	set := s.svc.Preferences.Get()
	set.MovingCovers = true
	if _, err := s.svc.Preferences.Set(set); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	mix, err := s.svc.Collection.CreatePlaylist("Night drive", "", testTracks[:2])
	if err != nil {
		t.Fatal(err)
	}
	file := writeTestFile(t, "night.gif", solidGIF(t, 200, 200, 10, gifColors...))
	if err := s.svc.Collection.SetMixCover(mix.ID, file); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	label := "Moving cover: Night drive"
	until(t, s, tt, "the mix's GIF", func() bool { _, ok := tt.Find(label); return ok })
	first := coverColor(t, tt, label)
	tt.Advance(100 * time.Millisecond)
	if got := coverColor(t, tt, label); nearRGBA(got, first) {
		t.Errorf("the header stays at %v", got)
	}
	show(s, tt, "/library/mixes")
	if _, ok := tt.Find(label); ok {
		t.Error("the mix's tile moves")
	}
}

// writeTestFile writes data to a file named name, in a folder of the
// test's, and returns its path.
func writeTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := t.TempDir() + string(os.PathSeparator) + name
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
